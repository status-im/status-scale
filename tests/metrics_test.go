package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/stretchr/testify/require"
)

func TestMeterDiscovery(t *testing.T) {
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Boot: 2, Relay: 20, Deploy: true}))
	defer c.Clean(context.TODO())
	for i := 0; i < 10; i++ {
		require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Users: 1, Deploy: true}))
		time.Sleep(2 * time.Second)
	}
	time.Sleep(10 * time.Second) // some time for metrics to accumulate
	tab := metrics.NewCompleteTab("container", metrics.DiscoveryColumns(), metrics.OnlyPeers())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()

	// FIXME think about extending metrics table with summary rows
	log.Info("see that metrics didn't change. discovery was stopped")
	time.Sleep(5 * time.Second)
	tab = metrics.NewCompleteTab("container", metrics.DiscoveryColumns(), metrics.OnlyPeers())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()

	log.Info("reboot nodes. no singnificant traffic should be observed.")
	require.NoError(t, c.Reboot(context.TODO(), cluster.User))
	time.Sleep(5 * time.Second)
	tab = metrics.NewCompleteTab("container", metrics.DiscoveryColumns(), metrics.OnlyPeers())
	require.NoError(t, c.FillMetrics(context.TODO(), tab, cluster.MetricsOpts{NoRelay: true}))
	metrics.ToASCII(tab, os.Stdout).Render()
}
