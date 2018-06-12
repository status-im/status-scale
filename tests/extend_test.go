package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/network"
	"github.com/stretchr/testify/require"
)

/*
what am i testing here?
we deploy bootnode and relay node in region A
relay node in region B need to discover node from region A
but access to bootnode from region A is blocked
so we deploy another bootnode that is not blocked in region A
and use it from relay node in region B

this way we test that bootnode B synced dht from bootnode A
and relay B was able to get required information
*/
func TestExtendCluster(t *testing.T) {
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{}))
	defer c.Clean(context.TODO())
	// makes a link to a previous bootnode
	require.NoError(t, c.Add(context.TODO(), cluster.ScaleOpts{Boot: 1, Deploy: true}))
	// create pending relay so that we can block ip on first bootnode before it gets connected to it
	// connect it only to 2nd bootnode
	require.NoError(t, c.Add(context.TODO(), cluster.ScaleOpts{
		Relay:  1,
		Enodes: []string{c.GetBootnode(1).Self().String()},
	}))
	// block second relay ip
	require.NoError(t, c.GetBootnode(0).EnableConditions(context.TODO(), network.Options{
		PacketLoss: 100,
		TargetAddr: c.GetPendingRelay(0).IP(),
	}))
	require.NoError(t, c.DeployPending(context.TODO()))
	Eventually(t, func() error {
		peers, err := c.GetRelay(0).Admin().Peers(context.TODO())
		if err != nil {
			return err
		}
		if len(peers) != 1 {
			return fmt.Errorf("peers %+v expected to be %d", peersToIPs(peers), 1)
		}
		return nil
	}, 30*time.Second, 1*time.Second)
}
