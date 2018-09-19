package tests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/whisper/whisperv6"
	"github.com/status-im/status-go/params"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneralWhisperRateLimiting tests two things:
// 1st round users exchange messages without any spam
// 2nd round spammer generates messages and users do receive them
// how fast we disconnect, what would be % of usefull lost messages
func TestGeneralWhisperRateLimiting(t *testing.T) {
	userIngress := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 1 << 20,  // 1mb - surge of 35k envelopes per single connection
		Quantum:  29 << 10, // 29kb - 98 envelopes per second
	}
	userEgress := userIngress
	userEgress.Quantum *= 5
	relayIngress := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 50 << 20,  // 50mb, surge of ~170k envelopes per single connection
		Quantum:  292 << 10, // 1000 envelopes per second. 292kb
	}
	relayEgress := relayIngress
	relayEgress.Quantum *= 5
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Boot: 1, Relay: 4, Users: 10,
		RelayIngress: relayIngress,
		RelayEgress:  relayEgress,
		UserIngress:  userIngress,
		UserEgress:   userEgress,
		Deploy:       true}))
	defer c.Clean(context.TODO())

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		count = map[int]int{}
		passw = "test"
		topic = whisperv6.TopicType{1, 1, 1, 1}
	)
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer wg.Done()
			ticker := time.Tick(100 * time.Millisecond)
			timeout := time.After(10 * time.Second)
			final := time.After(15 * time.Second)
			stop := false
			shh := c.GetUser(i).WhisperOriginal()
			symid, err := shh.GenerateSymmetricKeyFromPassword(context.TODO(), passw)
			assert.NoError(t, err)
			fid, err := shh.NewMessageFilter(context.TODO(), whisperv6.Criteria{
				SymKeyID: symid,
				MinPow:   .001,
				Topics:   []whisperv6.TopicType{topic},
			})
			assert.NoError(t, err)
			for {
				select {
				case <-ticker:
					msgs, err := shh.FilterMessages(context.TODO(), fid)
					if assert.NoError(t, err) {
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
							Payload:   make([]byte, 250),
							PowTime:   10,
							PowTarget: .002,
						})
						if assert.NoError(t, err) {
							return
						}
					}
				case <-timeout:
					stop = true
				case <-final:
					return
				}
			}
		}(i)
	}
	wg.Wait()
	// verify that everyone got same number of messages
	// there is 5 second window when everyone stopped sending but still have time to catch up
	for i := 0; i < 9; i++ {
		assert.Equal(t, count[i], count[i+1])
	}
	tab := metrics.NewCompleteTab("container", metrics.P2PColumns())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()
}
