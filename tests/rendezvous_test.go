package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/stretchr/testify/require"
)

func TestRendezvousDiscovery(t *testing.T) {
	c := ClusterFromConfig()

	// Setup the testing cluster
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Rendezvous: 1, Relay: 10, Deploy: true}))
	defer c.Clean(context.TODO()) // handle interrupt signal

	for i := 0; i < 5; i++ {
		require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Users: 10, Deploy: true}))
	}
	time.Sleep(10 * time.Second)
	tab := metrics.NewCompleteTab("container", metrics.RendezvousColumns(), metrics.OnlyPeers())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()
}

func TestMuxerDiscovery(t *testing.T) {
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Rendezvous: 1, Boot: 1, Deploy: true}))
	defer c.Clean(context.TODO())
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Relay: 1, Deploy: true,
		Enodes:          []string{c.GetBootnode(0).Self().String()},
		RendezvousNodes: []string{},
	}))
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Relay: 1, Deploy: true,
		RendezvousNodes: []string{c.GetRendezvous(0).Addr()},
		Enodes:          []string{},
	}))
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Users: 10, Deploy: true}))
	time.Sleep(10 * time.Second)
	tab := metrics.NewCompleteTab("container", metrics.RendezvousColumns(), metrics.DiscoveryColumns(), metrics.OnlyPeers())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()
}
