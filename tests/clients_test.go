package tests

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/churn"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/utils"
	"github.com/stretchr/testify/require"
)

func TestClientsExample(t *testing.T) {
	rand.Seed(time.Now().Unix())

	c := ClusterFromConfig()

	// Setup cluster from 20 relays, mailserver and bootnode to connect them.
	err := c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Mails: 1, Relay: 10, Deploy: true})
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
	//require.NoError(t, c.EnableConditionsGloobally(context.TODO(), network.Options{TargetAddr: c.IPAM.String(), Latency: 50}))
	churn := churn.NewChurnSim(c.GetUsers(), churn.Params{
		TargetAddrs: []string{c.IPAM.String()},
		Period:      30 * time.Second,
		ChurnRate:   0.1,
	})
	go func() {
		require.NoError(t, utils.PollImmediate(context.Background(), func(ctx context.Context) error {
			return churn.Control(ctx)
		}, 200*time.Millisecond, 20*time.Minute))
	}()
	rtt := client.NewRTTMeter(chat, c.GetUser(0), c.GetUser(1))
	// TODO(dshulyak) figure out how to measure distance between two peers.
	// one way is to get peers from one of the user and do bf search from there to second user.
	log.Debug("started metering latency")
	rtt.MeterFor(4 * time.Minute)
	log.Info("metered rtt", "messages", rtt.Messages(),
		"latency for 75 percentile", rtt.Percentile(75),
		"latency for 90 percentile", rtt.Percentile(90),
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
}
