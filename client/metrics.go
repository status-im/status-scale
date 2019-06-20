package client

import (
	"context"

	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/status-im/status-scale/utils"
)

func collect(ctx context.Context, tab *metrics.Table, peer *cluster.Peer) error {
	payload, err := peer.RawMetrics(ctx)
	if err != nil {
		return err
	}
	return tab.Append(peer.UID(), payload)
}

// TODO(dshulyak) Find common interface for relays and users.
func CollectMetrics(ctx context.Context, tab *metrics.Table, users []*cluster.Client, relays []*cluster.Peer) error {
	group := utils.NewGroup(ctx, len(users)+len(relays))
	for i := range users {
		c := users[i]
		group.Run(func(ctx context.Context) error {
			return collect(ctx, tab, c.Peer)
		})
	}
	for i := range relays {
		c := relays[i]
		group.Run(func(ctx context.Context) error {
			return collect(ctx, tab, c)
		})
	}
	return group.Error()
}
