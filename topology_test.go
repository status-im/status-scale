package scale

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/whisper/shhclient"
	"github.com/ethereum/go-ethereum/whisper/whisperv5"
	"github.com/status-im/status-scale/project"
	"github.com/stretchr/testify/suite"
)

var (
	central = flag.Int("central", 3, "central peers number.")
	leaf    = flag.Int("leaf", 7, "leaf peers number")
)

func TestTopologySuite(t *testing.T) {
	suite.Run(t, new(TopologySuite))
}

type TopologySuite struct {
	suite.Suite

	p        project.Project
	centrals []containerInfo
	leafs    []containerInfo
}

func (s *TopologySuite) SetupSuite() {
	flag.Parse()
}

func (s *TopologySuite) SetupTest() {
	cli, err := client.NewEnvClient()
	s.Require().NoError(err)
	cwd, err := os.Getwd()
	s.Require().NoError(err)
	s.p = project.New(
		filepath.Join(cwd, "custom-topology", "docker-compose.yml"),
		"customtopology",
		cli)
	s.NoError(s.p.Up(project.UpOpts{
		Scale: map[string]int{"central": *central, "leaf": *leaf},
		Wait:  *dockerTimeout,
	}))
	centrals, err := s.p.Containers(project.FilterOpts{SvcName: "central"})
	s.Require().NoError(err)
	s.centrals = makeContainerInfos(centrals)
	leafs, err := s.p.Containers(project.FilterOpts{SvcName: "leaf"})
	s.Require().NoError(err)
	s.leafs = makeContainerInfos(leafs)
}

func (s *TopologySuite) TearDownTest() {
	if !*keep {
		s.NoError(s.p.Down()) // make it optional and wait
	}
}

func (s *TopologySuite) NoErrors(errors []error) {
	failed := false
	for _, err := range errors {
		if err != nil {
			failed = true
		}
		s.NoError(err)
	}
	if failed {
		s.Require().FailNow("expected no errors")
	}
}

func (s *TopologySuite) TestCentralTopology() {
	msgNum := 100
	interval := 500 * time.Millisecond
	senderCount := 10
	payload := make([]byte, 150)
	if len(s.leafs) < senderCount {
		senderCount = len(s.leafs)
	}

	var mu sync.Mutex
	enodes := make([]string, len(s.centrals))
	s.NoErrors(runConcurrent(s.centrals, func(i int, c containerInfo) error {
		return runWithRetries(defaultRetries, defaultInterval, func() (err error) {
			client, err := rpc.Dial(c.RPC)
			if err != nil {
				return err
			}
			var info p2p.NodeInfo
			if err := client.CallContext(context.TODO(), &info, "admin_nodeInfo"); err != nil {
				return err
			}
			mu.Lock()
			enodes[i] = fmt.Sprintf(
				"enode://%s@%s:%d",
				info.ID, s.centrals[i].OverlayIP, info.Ports.Listener)
			mu.Unlock()
			return nil
		})
	}))
	s.NoErrors(runConcurrent(s.leafs, func(i int, c containerInfo) error {
		return runWithRetries(defaultRetries, defaultInterval, func() (err error) {
			client, err := rpc.Dial(c.RPC)
			if err != nil {
				return err
			}
			for _, enode := range enodes {
				if err := client.Call(nil, "admin_addPeer", enode); err != nil {
					return err
				}
			}
			return nil
		})
	}))
	s.NoErrors(runConcurrent(s.leafs, func(i int, c containerInfo) error {
		return runWithRetries(defaultRetries, 2*time.Second, func() (err error) {
			client, err := rpc.Dial(c.RPC)
			if err != nil {
				return err
			}
			var peers []*p2p.PeerInfo
			if err := client.Call(&peers, "admin_peers"); err != nil {
				return err
			}
			if len(peers) != len(s.centrals) {
				return fmt.Errorf("peers number is too low: %d", len(peers))
			}
			return nil
		})
	}))
	s.NoErrors(runConcurrent(s.leafs[:senderCount], func(i int, w containerInfo) error {
		c, err := shhclient.Dial(w.RPC)
		if err != nil {
			return err
		}
		var info whisperv5.Info
		if err := runWithRetries(defaultRetries, defaultInterval, func() (err error) {
			info, err = c.Info(context.TODO())
			return err
		}); err != nil {
			return err
		}

		symkey, err := c.NewSymmetricKey(context.TODO())
		if err != nil {
			return err
		}
		for j := 0; j < msgNum; j++ {
			// we should only fail if there was burst of errors
			if err := c.Post(context.TODO(), whisperv5.NewMessage{
				SymKeyID:  symkey,
				PowTarget: info.MinPow,
				PowTime:   200,
				Topic:     whisperv5.TopicType{0x03, 0x02, 0x02, 0x05},
				Payload:   payload,
			}); err != nil {
				return err
			}
			time.Sleep(interval)
		}
		return nil
	}))

	reports := make(Summary, len(s.leafs))
	s.NoErrors(runConcurrent(s.leafs, func(i int, w containerInfo) error {
		var prevOldCount, prevNewCount float64
		for {
			// wait till no duplicates are received
			// given that transmission cycle is 200 ms, 5s should be enough
			var oldCount, newCount float64
			if err := runWithRetries(defaultRetries, defaultInterval, func() (err error) {
				oldCount, newCount, err = pullOldNewEnvelopesCount(w.Metrics)
				return err
			}); err != nil {
				return err
			}
			prevNewCount = newCount
			if oldCount > prevOldCount {
				prevOldCount = oldCount
				time.Sleep(5 * time.Second)
				continue
			}
			break
		}
		mu.Lock()
		reports[i].NewEnvelopes = prevNewCount
		reports[i].OldEnvelopes = prevOldCount
		mu.Unlock()
		return nil
	}))
	s.NoErrors(runConcurrent(s.leafs, func(i int, w containerInfo) error {
		metrics, err := getEthMetrics(w.RPC)
		if err != nil {
			return err
		}
		mu.Lock()
		reports[i].Ingress = metrics.Peer2Peer.InboundTraffic.Overall
		reports[i].Egress = metrics.Peer2Peer.OutboundTraffic.Overall
		mu.Unlock()
		return nil
	}))
	s.NoError(reports.Print(os.Stdout))
}
