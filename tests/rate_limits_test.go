package tests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/whisper/whisperv6"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneralWhisperRateLimiting tests two things:
// 1st round users exchange messages without any spam
// 2nd round spammer generates messages and users receive them
// how fast we disconnect, what would be % of usefull lost messages
func TestGeneralWhisperRateLimiting(t *testing.T) {
	userLimit := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 1 << 20,  // 1mb - surge of 35k envelopes per single connection
		Quantum:  29 << 10, // 29kb - 98 envelopes per second
	}
	relayLimit := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 50 << 20,  // 50mb, surge of ~170k envelopes per single connection
		Quantum:  292 << 10, // 1000 envelopes per second. 292kb
	}
	c := ClusterFromConfig()
	users := 10
	relays := 4
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Rendezvous: 1, Relay: relays, Users: users,
		RelayIngress: relayLimit,
		RelayEgress:  relayLimit,
		UserIngress:  userLimit,
		UserEgress:   userLimit,
		TopicLimit:   userLimit,
		Deploy:       true}))
	defer c.Clean(context.TODO())

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = map[int]int{}
		passw = "test"
		topic = whisperv6.TopicType{1, 1, 1, 1}
	)
	log.Info("wait users are connected")
	wg.Add(users)
	errors := make(chan error, users)
	for i := 0; i < users; i++ {
		go func(i int) {
			defer wg.Done()
			errors <- c.GetUser(i).WaitConnected(context.TODO(), 1, 5*time.Second)
		}(i)
	}
	wg.Wait()
	close(errors)
	stop := false
	for err := range errors {
		stop = stop || !assert.NoError(t, err)
	}
	require.False(t, stop, "all users must be interconnected before main test starts")

	log.Info("ROUND 1: communication without spam")
	peerCommunication := func(i int, p *cluster.Peer, period time.Duration, size int) {
		ticker := time.Tick(period)
		timeout := time.After(10 * time.Second)
		final := time.After(15 * time.Second)
		stop := false
		shh := p.WhisperOriginal()
		symid, err := shh.GenerateSymmetricKeyFromPassword(context.TODO(), passw)
		assert.NoError(t, err)
		fid, err := shh.NewMessageFilter(context.TODO(), whisperv6.Criteria{
			SymKeyID: symid,
			MinPow:   .001,
			Topics:   []whisperv6.TopicType{topic},
		})
		if !assert.NoError(t, err) {
			return
		}
		for {
			select {
			case <-ticker:
				msgs, err := shh.FilterMessages(context.TODO(), fid)
				if !assert.NoError(t, err) {
					return
				}
				mu.Lock()
				count[i] += len(msgs)
				mu.Unlock()
				if !stop {
					_, err = shh.Post(context.TODO(), whisperv6.NewMessage{
						SymKeyID:  symid,
						TTL:       5,
						Topic:     topic,
						Payload:   make([]byte, size),
						PowTime:   10,
						PowTarget: .002,
					})
					if !assert.NoError(t, err) {
						return
					}
				}
			case <-timeout:
				stop = true
			case <-final:
				return
			}
		}
	}
	communicationRound := func() {
		wg.Add(users)
		period := 500 * time.Millisecond
		size := 250
		for i := 0; i < users; i++ {
			go func(i int, p *cluster.Peer) {
				defer wg.Done()
				peerCommunication(i, p, period, size)
			}(i, c.GetUser(i))
		}
		wg.Wait()
	}
	communicationRound()
	// verify that everyone got same number of messages
	// there is 5 second window when everyone stopped sending but still have time to catch up
	for i := 0; i < users-1; i++ {
		assert.Equal(t, count[i], count[i+1])
	}
	tab := metrics.NewCompleteTab("container", metrics.P2PColumns())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()

	log.Info("deploy spammer and setup static connections with every relay node")
	spammerLimits := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 10 << 30,
		Quantum:  10 << 20,
	}
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Relay:           1,
		RendezvousNodes: []string{},
		RelayIngress:    spammerLimits,
		RelayEgress:     spammerLimits,
		TopicLimit:      spammerLimits,
		IgnoreEgress:    true,
		Deploy:          true}))
	spammer := c.GetRelay(4)
	for i := 0; i < 4; i++ {
		admin := spammer.Admin()
		info, err := c.GetRelay(i).Admin().Self(context.TODO())
		require.NoError(t, err)
		require.NoError(t, admin.AddPeer(context.TODO(), info.Enode))
	}
	log.Info("ROUND 2: communication with spam")
	require.NoError(t, spammer.WaitConnected(context.TODO(), 4, 2*time.Second))
	peerCommunication(255, spammer, 100*time.Millisecond, 50<<10)
	tab = metrics.NewCompleteTab("container", metrics.P2PColumns())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{}))
	metrics.ToASCII(tab, os.Stdout).Render()
}
