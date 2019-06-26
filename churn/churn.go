package churn

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/network"
	"github.com/status-im/status-scale/utils"
)

func NewChurnSim(participants []*cluster.Client, params Params) *ChurnSim {
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

type Params struct {
	TargetAddrs []string
	// ChurnRate specifies part of time that peer is online
	ChurnRate float64
	Period    time.Duration
}

// ChurnSim controls live and offline period for each participant.
type ChurnSim struct {
	Params Params

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
