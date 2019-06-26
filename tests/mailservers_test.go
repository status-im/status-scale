package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/montanaflynn/stats"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/client"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/metrics"
	"github.com/status-im/status-scale/network"
	"github.com/status-im/status-scale/utils"
	"github.com/stretchr/testify/require"
)

func TestHistoryDownload(t *testing.T) {
	c := ClusterFromConfig()

	err := c.Create(context.TODO(), cluster.ScaleOpts{Boot: 1, Mails: 1, Deploy: true})
	defer c.Clean(context.TODO())
	require.NoError(t, err)
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{Users: 1, Deploy: true}))

	user0 := client.ChatClient(c.GetUser(0).Rpc())

	name := make([]byte, 10)
	n, err := rand.Read(name)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	chat := gethservice.Contact{
		Name: hexutil.Encode(name),
	}
	require.NoError(t, user0.AddContact(context.Background(), chat))
	size := 1000
	start := time.Now()
	for j := 0; j < 2; j++ {
		group := utils.NewGroup(context.Background(), size)
		log.Info("started generating messages", "at", start)
		padding := string(make([]byte, 100))
		for i := 0; i < size; i++ {
			i := i
			group.Run(func(ctx context.Context) error {
				payload := strconv.Itoa(i)
				log.Trace("sending message with payload", "payload", payload)
				err := user0.Send(ctx, chat, string(padding)+payload+strconv.Itoa(j))
				if err != nil {
					return fmt.Errorf("failed to save msg with payload %s: %v", payload, err)
				}
				return nil
			})
		}
		require.NoError(t, group.Error())
	}
	log.Info("messages generated. started collecting requests stats", "took", time.Since(start))
	mail := c.GetMail(0)
	for _, latency := range []int{40, 140, 400} {
		require.NoError(t, mail.EnableConditions(context.Background(), network.Options{
			TargetAddrs: []string{c.GetUser(0).IP()},
			Latency:     latency,
			BW:          40000,
		}))
		samples := make([]float64, 30)
		for i := range samples {
			start := time.Now()
			require.NoError(t, user0.Request(context.Background(), client.RequestParams{
				Contact: chat,
				From:    time.Now().Add(-20 * time.Minute).Unix(),
				To:      time.Now().Unix(),
				Limit:   2001,
			}))
			samples[i] = time.Since(start).Seconds()
		}

		percentile95, err := stats.Percentile(samples, 95)
		require.NoError(t, err)
		percentile99, err := stats.Percentile(samples, 99)
		require.NoError(t, err)
		log.Info("collected request stats", "latency", latency, "percentile 95", percentile95, "percentile 99", percentile99)
		require.NoError(t, mail.DisableConditions(context.Background(), network.Options{
			PacketLoss:  1,
			TargetAddrs: []string{c.GetUser(0).IP()},
			Latency:     latency,
		}))
	}
	table := metrics.NewCompleteTab("container name", metrics.P2PColumns())
	require.NoError(t, client.CollectMetrics(context.Background(), table, c.GetUsers(), nil))
	metrics.ToASCII(table, os.Stdout).Render()
}
