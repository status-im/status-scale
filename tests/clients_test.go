package tests

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/montanaflynn/stats"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/network"
	"github.com/stretchr/testify/require"
)

func TestClientsExample(t *testing.T) {
	c := ClusterFromConfig()

	// Setup cluster from 20 relays, mailserver and bootnode to connect them.
	err := c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Mails: 1, Relay: 20, Deploy: true})
	defer c.Clean(context.TODO())
	require.NoError(t, err)
	// Add two console client to cluster. Note that mailserver has to be deployed before adding clients
	// as the current client code depends on available mailservers.
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Users: 2, Deploy: true}))

	user0 := client.ChatClient(c.GetUser(0).Rpc())
	user1 := client.ChatClient(c.GetUser(1).Rpc())

	name := make([]byte, 10)
	n, err := rand.Read(name)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	chat := gethservice.Contact{
		Name: hexutil.Encode(name),
	}

	require.NoError(t, user0.AddContact(context.TODO(), chat))
	require.NoError(t, user1.AddContact(context.TODO(), chat))

	// FIXME(dshulyak) if addr is not provided comcast will use both iptables and ip6tables to insert mangle rules
	// ip6tables fails in the container on my enviornment due to lack of kernel module
	require.NoError(t, c.EnableConditionsGloobally(context.TODO(), network.Options{TargetAddr: c.IPAM.String(), Latency: 25}))
	rtt := &RTTMeter{
		chat:     chat,
		sender:   c.GetUser(0),
		receiver: c.GetUser(1),
	}
	// TODO(dshulyak) figure out how to measure distance between two peers.
	// one way is to get peers from one of the user and do breadth-first search
	log.Info("started metering latency")
	require.NoError(t, rtt.MeterSequantially(100))
	log.Info("metered rtt", "messages", 100,
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
}

type RTTMeter struct {
	chat gethservice.Contact

	sender, receiver *cluster.Client
	samples          []float64
}

func (m *RTTMeter) MeterSequantially(count int) error {
	for i := 0; i < count; i++ {
		err := m.meter(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m RTTMeter) Percentile(percent float64) float64 {
	rst, err := stats.Percentile(m.samples, percent)
	if err != nil {
		return 0
	}
	return rst
}

func (m *RTTMeter) send(i int) (time.Time, error) {
	tick := time.Tick(20 * time.Millisecond)
	after := time.After(10 * time.Minute)
	payload := fmt.Sprintf("hello receiver: %d", i)
	// TODO(dshulyak) add util PollImmediatly(func(context.Context) error, period, timeout time.Duration)
	for {
		select {
		case <-tick:
			sent := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := client.ChatClient(m.sender.Rpc()).Send(ctx, m.chat, payload)
			cancel()
			if err != nil {
				log.Debug("can't send msg", "payload", payload, "error", err)
				continue
			}
			return sent, nil
		case <-after:
			return time.Time{}, fmt.Errorf("failed to send a message %s", payload)
		}
	}
}

func (m *RTTMeter) receive(i int) error {
	tick := time.Tick(20 * time.Millisecond)
	after := time.After(10 * time.Minute)
	payload := fmt.Sprintf("hello receiver: %d", i)
	for {
		select {
		case <-tick:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msgs, err := client.ChatClient(m.sender.Rpc()).Messages(ctx, m.chat, int64(i))
			cancel()
			if err != nil {
				return err
			}
			for _, msg := range msgs {
				if msg.Text == payload {
					return nil
				}
			}
		case <-after:
			return fmt.Errorf("failed waiting for a message with payload %s", payload)
		}
	}
}

func (m *RTTMeter) meter(i int) error {
	sent, err := m.send(i)
	if err != nil {
		return err
	}
	err = m.receive(i)
	if err != nil {
		return err
	}
	log.Debug("latency for msg", "i", i, "duration", time.Since(sent))
	m.samples = append(m.samples, time.Since(sent).Seconds())
	return nil
}
