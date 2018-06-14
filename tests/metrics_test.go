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

func TestMetrics(t *testing.T) {
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Relay: 2, Deploy: true}))
	defer c.Clean(context.TODO())
	tab := metrics.NewCompleteTab("container", metrics.P2PColumns(), metrics.DiscoveryColumns())
	time.Sleep(30 * time.Second) // some time for metrics to accumulate
	require.NoError(t, c.FillMetrics(context.TODO(), tab))
	metrics.ToASCII(tab, os.Stdout).Render()
}
