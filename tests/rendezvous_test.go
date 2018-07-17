package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/status-im/status-scale/cluster"
	"github.com/stretchr/testify/require"
)

func TestRendezvousDiscovery(t *testing.T) {
	c := ClusterFromConfig()

	// Setup the testing cluster
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Rendezvous: 1, Relay: 3, Deploy: true}))
	defer c.Clean(context.TODO()) // handle interrupt signal
	var peers []*p2p.PeerInfo

	// All connected peers to the first relay peer should be eq 2
	Eventually(t, func() error {
		var err error
		peers, err = c.GetRelay(0).Admin().Peers(context.TODO())
		log.Trace("waiting for 2 peers", "peers", len(peers), "err", err)
		if err != nil {
			return err
		}
		if len(peers) != 2 {
			return fmt.Errorf("peers %+v expected to be %d", peersToIPs(peers), 2)
		}
		return nil
	}, 30*time.Second, 1*time.Second)
}
