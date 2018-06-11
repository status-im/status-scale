package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/status-im/status-scale/network"
	"github.com/stretchr/testify/require"
)

func TestBlockedPeer(t *testing.T) {
	c := ClusterFromConfig()
	c.Boot = 1
	c.Relay = 3
	require.NoError(t, c.Create(context.TODO()))
	defer c.Clean(context.TODO()) // handle interrupt signal
	var peers []*p2p.PeerInfo
	Eventually(t, func() error {
		var err error
		peers, err = c.GetRelay(0).Admin().Peers(context.TODO())
		if err != nil {
			return err
		}
		if len(peers) != 2 {
			return fmt.Errorf("peers %+v expected to be %d", peersToIPs(peers), 2)
		}
		return nil
	}, 30*time.Second, 1*time.Second)
	require.NoError(t, c.GetRelay(0).EnableConditions(context.TODO(), network.Options{
		PacketLoss: 100,
		TargetAddr: strings.Split(peers[0].Network.RemoteAddress, ":")[0],
	}))
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
