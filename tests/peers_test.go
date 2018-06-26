package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/whisper/whisperv6"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/network"
	"github.com/stretchr/testify/require"
)

func TestSentEnvelope(t *testing.T) {
	// Setup cluster
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Relay: 2, Deploy: true}))
	defer c.Clean(context.TODO()) // handle interrupt signal

	// Get the rpc client
	client := c.GetRelay(0).Whisper()
	require.NotNil(t, client)

	var peers []*p2p.PeerInfo
	// Check connected peers
	Eventually(t, func() error {
		var err error
		peers, err = c.GetRelay(0).Admin().Peers(context.TODO())
		log.Trace("waiting for 1 peers", "peers", len(peers), "err", err)
		if err != nil {
			return err
		}
		if len(peers) != 1 {
			return fmt.Errorf("peers %+v expected to be %d", peersToIPs(peers), 2)
		}
		return nil
	}, 30*time.Second, 1*time.Second)

	symID, err := client.NewSymKey()
	require.NoError(t, err)
	msg := whisperv6.NewMessage{
		SymKeyID:  symID,
		PowTarget: whisperv6.DefaultMinimumPoW,
		PowTime:   200,
		TTL:       10,
		Topic:     whisperv6.TopicType{0x01, 0x01, 0x01, 0x01},
		Payload:   []byte("hello"),
	}

	// With default network capabilities we should be able to succesfully send
	// messages
	hash, err := client.DebugPost(msg)
	require.NoError(t, err)
	require.NotEmpty(t, hash.String())

	// Switching connected peers network connection
	require.NoError(t, c.GetRelay(1).EnableConditions(context.TODO(), network.Options{
		PacketLoss: 100,
		TargetAddr: c.GetRelay(0).IP(),
	}))

	// As our current node doesn't have any connected peer, no message should be sent
	hash, err = client.DebugPost(msg)
	require.Error(t, err)

	var expected common.Hash
	require.Equal(t, expected, hash)
}
