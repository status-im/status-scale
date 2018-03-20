package scale

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/status-scale/project"
	"github.com/stretchr/testify/suite"
)

func TestV5TopologySuite(t *testing.T) {
	suite.Run(t, new(V5TopologySuite))
}

type V5TopologySuite struct {
	suite.Suite

	p        project.Project
	centrals []containerInfo
	leafs    []containerInfo
}

func (s *V5TopologySuite) SetupSuite() {
	flag.Parse()
}

func (s *V5TopologySuite) SetupTest() {
	cli, err := client.NewEnvClient()
	s.Require().NoError(err)
	cwd, err := os.Getwd()
	s.Require().NoError(err)
	s.p = project.New(
		filepath.Join(cwd, "v5test", "docker-compose.yml"),
		"v5test",
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

func (s *V5TopologySuite) TearDownTest() {
	if !*keep {
		s.NoError(s.p.Down()) // make it optional and wait
	}
}

func (s *V5TopologySuite) NoErrors(errors []error) {
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

func waitPeersConnected(peers []containerInfo, maxPeers int) []error {
	return runConcurrent(peers, func(i int, c containerInfo) error {
		return runWithRetries(100, 3*time.Second, func() (err error) {
			client, err := rpc.Dial(c.RPC)
			if err != nil {
				return err
			}
			var info []*p2p.PeerInfo
			if err := client.CallContext(context.TODO(), &info, "admin_peers"); err != nil {
				return err
			}
			fmt.Println(c.Name, len(info))
			if len(info) == maxPeers {
				return nil
			}
			if len(info) >= maxPeers {
				return errors.New("more than max peers connected")
			}
			return errors.New("not enough peers connected")
		})
	})
}

// TestIdle meters discv5 network and cpu usage while peers are connected
// but in idle state.
func (s *V5TopologySuite) TestIdle() {
	// checkpoint network/cpu usage before min amount of peers is reached
	// checkpoint network/cpu usage between min and max peers
	maxPeers := 3
	var mu sync.Mutex
	s.NoErrors(waitPeersConnected(s.leafs, maxPeers))
	reports := make(DiscoverySummary, len(s.leafs))
	s.NoErrors(runConcurrent(s.leafs, func(i int, w containerInfo) error {
		metrics, err := getEthMetrics(w.RPC)
		if err != nil {
			return err
		}
		mu.Lock()
		reports[i].Ingress = metrics.Discv5.InboundTraffic.Overall
		reports[i].Egress = metrics.Discv5.OutboundTraffic.Overall
		mu.Unlock()
		return nil
	}))
	reports.Print(os.Stdout)
}

type DiscoverySummary []DiscoveryReport

func (s DiscoverySummary) Print(w io.Writer) error {
	tab := newASCIITable(w)
	_, err := fmt.Fprintln(w, "=== SUMMARY")
	if err != nil {
		return err
	}
	if err := tab.AddHeaders("HEADERS", "ingress", "egress"); err != nil {
		return err
	}
	for i, r := range s {
		if err := tab.AddRow(
			fmt.Sprintf("%d", i),
			fmt.Sprintf("%f mb", r.Ingress/1024/1024),
			fmt.Sprintf("%f mb", r.Egress/1024/1024),
		); err != nil {
			return err
		}
	}
	return tab.Flush()
}

type DiscoveryReport struct {
	Ingress float64
	Egress  float64
}
