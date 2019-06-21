package tests

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
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
	rtt := client.NewRTTMeter(chat, c.GetUser(0), c.GetUser(1))
	// TODO(dshulyak) figure out how to measure distance between two peers.
	// one way is to get peers from one of the user and do breadth-first search
	log.Info("started metering latency")
	require.NoError(t, rtt.MeterSequantially(100))
	log.Info("metered rtt", "messages", 100,
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
}

func NewChurnSim(participants []*cluster.Client, total time.Duration, churn int) *ChurnSim {
	live := total / time.Duration(churn)
	jitter := total / time.Duration(2)
	lth := len(participants)
	liveSince := make([]time.Time, lth)
	now := time.Now()
	for i := range liveSince {
		liveSince[i] = now
	}
	return &ChurnSim{
		live:         live,
		total:        total,
		jitter:       jitter,
		participants: participants,
		offline:      make([]bool, lth),
		offlineSince: make([]time.Time, lth),
		liveSince:    liveSince,
	}
}

// ChurnSim controls live and offline period for each participant.
type ChurnSim struct {
	live         time.Duration
	total        time.Duration
	jitter       time.Duration // jitter is a half of the total
	participants []*cluster.Client
	liveSince    []time.Time
	offlineSince []time.Time
	offline      []bool
}

func (c *ChurnSim) Control(ctx context.Context) error {
	for i := range c.participants {
		err := c.control(ctx, i)
		if err != nil {
			return err
		}
	}
}

func (c *ChurnSim) control(ctx context.Context, i int) error {
	if time.Since(c.liveSince[i]) > c.live && !c.offline[i] {
		err := c.participants[i].Stop(ctx)
		if err != nil {
			return err
		}
		c.offlineSince[i] = time.Now()
		c.offline[i] = true
	}
	total := c.jitter
	total += time.Duration(rand.Int64(rand.Reader, new(big.Int).SetInt64(c.jitter)))
	if time.Since(c.offlineSince[i]) > total {
		err := c.participants[i].Start(ctx)
		if err != nil {
			return err
		}
		c.liveSince[i] = time.Now()
		c.offline[i] = false
	}
}
