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
		sender:   user0,
		receiver: user1,
	}
	log.Info("started metering latency")
	require.NoError(t, rtt.MeterSequantially(100))
	log.Info("metered rtt", "messages", 100,
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
}

type RTTMeter struct {
	chat gethservice.Contact

	sender, receiver client.Chat
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

func (m *RTTMeter) meter(i int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	payload := fmt.Sprintf("hello receiver: %d", i)
	sent := time.Now()
	err := m.sender.Send(ctx, m.chat, payload)
	defer cancel()
	if err != nil {
		return err
	}
	tick := time.Tick(50 * time.Millisecond)
	after := time.After(10 * time.Second)
	for {
		select {
		case <-tick:
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			msgs, err := m.receiver.Messages(ctx, m.chat, int64(i))
			defer cancel()
			if err != nil {
				return err
			}
			for _, msg := range msgs {
				if msg.Text == payload {
					m.samples = append(m.samples, time.Since(sent).Seconds())
					return nil
				}
			}
		case <-after:
			return fmt.Errorf("failed waiting for a message with payload %s", payload)
		}
	}
}
