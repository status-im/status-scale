package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/status-im/status-scale/network"
	"github.com/status-im/status-scale/utils"
	"github.com/stretchr/testify/require"
)

func TestClientsExample(t *testing.T) {
	rand.Seed(time.Now().Unix())

	c := ClusterFromConfig()

	// Setup cluster from 20 relays, mailserver and bootnode to connect them.
	err := c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Mails: 1, Relay: 20, Deploy: true})
	defer c.Clean(context.TODO())
	require.NoError(t, err)
	// Add two console client to cluster. Note that mailserver has to be deployed before adding clients
	// as the current client code depends on available mailservers.
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{MVDS: 2, Deploy: true}))

	user0 := client.ChatClient(c.GetMVDS(0).Rpc())
	user1 := client.ChatClient(c.GetMVDS(1).Rpc())

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
	churn := NewChurnSim(c.GetMVDSClients(), ChurnParams{
		TargetAddrs: []string{c.IPAM.String()},
		Period:      10 * time.Second,
		ChurnRate:   0.1,
	})
	go func() {
		require.NoError(t, utils.PollImmediate(context.Background(), func(ctx context.Context) error {
			return churn.Control(ctx)
		}, 200*time.Millisecond, 20*time.Minute))
	}()
	rtt := client.NewRTTMeter(chat, c.GetMVDS(0), c.GetMVDS(1))
	// TODO(dshulyak) figure out how to measure distance between two peers.
	// one way is to get peers from one of the user and do breadth-first search
	log.Debug("started metering latency")
	rtt.MeterFor(10 * time.Minute)
	log.Info("metered rtt", "messages", rtt.Messages(),
		"latency for 75 percentile", rtt.Percentile(75),
		"latency for 90 percentile", rtt.Percentile(90),
		"latency for 95 percentile", rtt.Percentile(95),
		"latency for 99.9 percentile", rtt.Percentile(99.9))
	table := metrics.NewCompleteTab("container name", metrics.P2PColumns())
	require.NoError(t, client.CollectMetrics(context.Background(), table, c.GetMVDSClients(), nil))
	metrics.ToASCII(table, os.Stdout).Render()
}

func NewChurnSim(participants []*cluster.Client, params ChurnParams) *ChurnSim {
	live := time.Duration(params.Period.Seconds()*params.ChurnRate) * time.Second
	jitter := params.Period / time.Duration(2)
	lth := len(participants)
	liveSince := make([]time.Time, lth)
	return &ChurnSim{
		Params:       params,
		live:         live,
		total:        params.Period,
		jitter:       jitter,
		participants: participants,
		offline:      make([]bool, lth),
		offlineUntil: make([]time.Time, lth),
		liveSince:    liveSince,
	}
}

type ChurnParams struct {
	TargetAddrs []string
	// ChurnRate specifies part of time that peer is online
	ChurnRate float64
	Period    time.Duration
}

// ChurnSim controls live and offline period for each participant.
type ChurnSim struct {
	Params ChurnParams

	live         time.Duration
	total        time.Duration
	jitter       time.Duration // jitter is a half of the total
	participants []*cluster.Client
	liveSince    []time.Time
	offlineUntil []time.Time
	offline      []bool
}

func (c *ChurnSim) Control(ctx context.Context) error {
	for i := range c.participants {
		err := c.control(ctx, i)
		if err != nil {
			return err
		}
	}
	return nil
}

// stop must prevent peer from receiving any traffic from peers in the same network
func (c *ChurnSim) stop(ctx context.Context, p *cluster.Client) error {
	return p.EnableConditions(ctx, network.Options{
		PacketLoss:  100,
		TargetAddrs: c.Params.TargetAddrs,
	})
}

// start must disable packet loss and trigger history request for a specific chat or all history
// note(dshulyak) requesting all history will make latency higher but it is more releastic.
func (c *ChurnSim) start(ctx context.Context, p *cluster.Client) error {
	err := p.DisableConditions(ctx, network.Options{
		PacketLoss:  100,
		TargetAddrs: c.Params.TargetAddrs,
	})
	if err != nil {
		return fmt.Errorf("failed to disable packet loss: %v", err)
	}
	return utils.PollImmediateNoError(ctx, func(parent context.Context) error {
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		defer cancel()
		err := client.ChatClient(p.Rpc()).RequestAll(ctx)
		if err != nil {
			return fmt.Errorf("requesting messages failed: %v", err)
		}
		return nil
	}, 2*time.Second, 30*time.Second)
}

// FIXME (dshulyak) rewrite in a concurrently friendly way
func (c *ChurnSim) control(ctx context.Context, i int) error {
	if !c.offline[i] && time.Since(c.liveSince[i]) > c.live {
		log.Debug("peer will be stopped", "peer", c.participants[i].UID())
		err := c.stop(ctx, c.participants[i])
		if err != nil {
			log.Error("stopping peer failed", "peer", c.participants[i].UID(), "error", err)
			return err
		}
		log.Debug("peer is stopped", "peer", c.participants[i].UID())
		offline := c.jitter
		jitter := time.Duration(rand.Int63n(int64(c.jitter.Seconds()))*2) * time.Second
		offline += jitter
		log.Debug("peer will be offline", "peer", c.participants[i].UID(), "duration", offline)
		c.offlineUntil[i] = time.Now().Add(offline)
		c.offline[i] = true
		return nil
	}
	if c.offline[i] && time.Now().Sub(c.offlineUntil[i]) > 0 {
		log.Debug("peer will be started", "peer", c.participants[i].UID())
		err := c.start(ctx, c.participants[i])
		if err != nil {
			log.Error("starting peer failed", "peer", c.participants[i].UID(), "error", err)
			return err
		}
		log.Debug("peer is started", "peer", c.participants[i].UID())
		c.liveSince[i] = time.Now()
		c.offline[i] = false
	}
	return nil
}
