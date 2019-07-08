package tests

import (
	"context"
	"crypto/elliptic"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/churn"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/status-im/status-scale/utils"
	"github.com/stretchr/testify/assert"
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

	var (
		user0               = client.ChatClient(c.GetUser(0).Rpc())
		user1               = client.ChatClient(c.GetUser(1).Rpc())
		id0                 = c.GetUser(0).Identity
		id1                 = c.GetUser(1).Identity
		key0  hexutil.Bytes = elliptic.Marshal(crypto.S256(), id1.PublicKey.X, id1.PublicKey.Y)
		key1  hexutil.Bytes = elliptic.Marshal(crypto.S256(), id0.PublicKey.X, id0.PublicKey.Y)
	)

	name := make([]byte, 10)
	n, err := rand.Read(name)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	chat0 := gethservice.Contact{
		Name:      hexutil.Encode(name),
		PublicKey: key1,
	}
	chat1 := gethservice.Contact{
		Name:      hexutil.Encode(name),
		PublicKey: key0,
	}

	require.NoError(t, user0.AddContact(context.TODO(), chat0))
	require.NoError(t, user1.AddContact(context.TODO(), chat1))

	// FIXME(dshulyak) if addr is not provided comcast will use both iptables and ip6tables to insert mangle rules
	// ip6tables fails in the container on my enviornment due to lack of kernel module
	//require.NoError(t, c.EnableConditionsGloobally(context.TODO(), network.Options{TargetAddr: c.IPAM.String(), Latency: 50}))
	churn := churn.NewChurnSim(c.GetUsers(), churn.Params{
		TargetAddrs: []string{c.IPAM.String()},
		Period:      10 * time.Second,
		ChurnRate:   0.1,
	})
	churnCtx, cancel := context.WithCancel(context.Background())
	go func() {
		assert.NoError(t, utils.PollImmediate(churnCtx, func(ctx context.Context) error {
			return churn.Control(ctx)
		}, 200*time.Millisecond, 180*time.Minute))
		// start all nodes after churn simulator was terminated
		// FIXME(dshulyak) Start is confusing, it starts nodes but stops simulating networking issues
		// need a better name
		log.Debug("starting nodes")
		assert.NoError(t, churn.Start(context.Background()))
	}()
	rtt := client.NewRTTMeter(chat0, c.GetUser(0), c.GetUser(1))
	// TODO(dshulyak) figure out how to measure distance between two peers.
	// one way is to get peers from one of the user and do bf search from there to second user.
	log.Debug("started metering latency")
	rtt.MeterFor(1 * time.Minute)
	cancel()
	log.Info("metered rtt", "messages", rtt.Messages(),
		"latency for 75 percentile", rtt.Percentile(75),
		"latency for 90 percentile", rtt.Percentile(90),
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
	table := metrics.NewCompleteTab("container name", metrics.Envelopes())
	log.Debug("collecting metrics")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	require.NoError(t, client.CollectMetrics(ctx, table, c.GetUsers(), nil))
	cancel()
	log.Debug("collected metrics")
	metrics.ToASCII(table, os.Stdout).Render()
}
