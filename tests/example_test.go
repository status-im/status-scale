package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/network"
	"github.com/stretchr/testify/require"
)

func TestBlockedPeer(t *testing.T) {
	c := ClusterFromConfig()

	// Setup the testing cluster
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Relay: 3, Deploy: true}))
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

	// Set flaky network connection on the first relay peer
	require.NoError(t, c.GetRelay(0).EnableConditions(context.TODO(), network.Options{
		PacketLoss: 100,
		TargetAddr: strings.Split(peers[0].Network.RemoteAddress, ":")[0],
	}))

	// As one of the connected peers will have a flaky connection now, we are
	// checking the number of connected peers is one less
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

func peersToIPs(peers []*p2p.PeerInfo) (ips []string) {
	for _, p := range peers {
		ips = append(ips, p.Network.RemoteAddress)
	}
	return
}
