package tests

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/stretchr/testify/require"
)

func TestClientsExample(t *testing.T) {
	c := ClusterFromConfig()

	// Setup cluster from two status clients and bootnode to connect them.
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Users: 2, Deploy: true}))
	defer c.Clean(context.TODO())

	Eventually(t, func() error {
		peers, err := client.AdminClient(c.GetUser(0).Rpc()).Peers(context.TODO())
		log.Trace("waiting for 1 peer", "peers", len(peers), "err", err)
		if err != nil {
			return err
		}
		if len(peers) != 1 {
			return fmt.Errorf("expecting connection with one peer")
		}
		return nil
	}, 30*time.Second, 1*time.Second)

	user0 := client.ChatClient(c.GetUser(0).Rpc())
	user1 := client.ChatClient(c.GetUser(1).Rpc())

	name := make([]byte, 10)
	n, err := rand.Read(name)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	chat := gethservice.Contact{
		Name: hexutil.Encode(name),
	}
	require.NoError(t, user0.AddContact(context.TODO(), chat))
	require.NoError(t, user1.AddContact(context.TODO(), chat))

	require.NoError(t, user0.Send(context.TODO(), chat, "hello user1"))

	Eventually(t, func() error {
		msgs, err := user1.Messages(context.TODO(), chat, 0)
		if err != nil {
			return err
		}
		if len(msgs) != 1 {
			return fmt.Errorf("expecting single message")
		}
		return nil
	}, 30*time.Second, 1*time.Second)

	tab := metrics.NewCompleteTab("container", metrics.P2PColumns())
	require.NoError(t, client.CollectMetrics(context.TODO(), tab, c.GetUsers(), nil))
	metrics.ToASCII(tab, os.Stdout).Render()
}
