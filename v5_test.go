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

var (
	rare     = flag.Int("rare", 2, "rare peers that are required by leaf nodes.")
	idleTime = flag.Duration("idle", 0, "Defines how long test will sleep after connecting with peers.")
)

func TestV5TopologySuite(t *testing.T) {
	suite.Run(t, new(V5TopologySuite))
}

type V5TopologySuite struct {
	suite.Suite

	p        project.Project
	centrals []containerInfo
	leafs    []containerInfo
	rares    []containerInfo
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
		Scale: map[string]int{"central": *central, "leaf": 0, "rare": *rare},
		Wait:  *dockerTimeout,
	}))
	centrals, err := s.p.Containers(project.FilterOpts{SvcName: "central"})
	s.Require().NoError(err)
	s.centrals = makeContainerInfos(centrals)
	rares, err := s.p.Containers(project.FilterOpts{SvcName: "rare"})
	s.Require().NoError(err)
	s.rares = makeContainerInfos(rares)
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

func waitPeersConnected(peers []containerInfo, minPeers int) []error {
	return runConcurrent(peers, func(i int, c containerInfo) error {
		return runWithRetries(100, 1*time.Second, func() (err error) {
			client, err := rpc.Dial(c.RPC)
			if err != nil {
				return err
			}
			var info []*p2p.PeerInfo
			if err := client.CallContext(context.TODO(), &info, "admin_peers"); err != nil {
				return err
			}
			fmt.Println(c.Name, len(info))
			if len(info) >= minPeers {
				return nil
			}
			return errors.New("not enough peers connected")
		})
	})
}

// TestIdle meters discv5 network and cpu usage while peers are connected
// but in idle state.
func (s *V5TopologySuite) TestIdle() {
	// we will wait till tables of central nodes are filled with peers information
	s.NoErrors(waitPeersConnected(s.centrals, 2)) // 2 whisper peers
	s.NoErrors(waitPeersConnected(s.rares, 2))    // 2 whisper peers
	s.NoError(s.p.Up(project.UpOpts{
		Scale: map[string]int{"central": *central, "leaf": *leaf, "rare": *rare},
		Wait:  *dockerTimeout,
	}))
	leafs, err := s.p.Containers(project.FilterOpts{SvcName: "leaf"})
	s.Require().NoError(err)
	s.leafs = makeContainerInfos(leafs)
	s.waitConnectedAndGetMetrics()
	// restart containers to see how fast leafs will re-establish connections
	// with central nodes and rare nodes
	// NOTE don't restart by docker compsoe cause all discovery information will become invalid
	for _, cinfo := range s.leafs {
		s.Require().NoError(s.p.RestartContainer(cinfo.Name))
	}
	fmt.Println("containers restarted")
	leafs, err = s.p.Containers(project.FilterOpts{SvcName: "leaf"})
	s.Require().NoError(err)
	s.leafs = makeContainerInfos(leafs)
	s.waitConnectedAndGetMetrics()
}

func (s *V5TopologySuite) waitConnectedAndGetMetrics() {
	var mu sync.Mutex
	before := time.Now()
	s.NoErrors(waitPeersConnected(s.leafs, 3)) // 2 whisper + 1 mailserver
	after := time.Now()
	time.Sleep(*idleTime)
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
	fmt.Println(after.Sub(before).Seconds())
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
