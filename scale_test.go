package scale

import (
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/docker/docker/api/types/swarm"
)

const (
	defaultRetries  = 10
	defaultInterval = 500 * time.Millisecond
)

var (
	keep          = flag.Bool("keep", false, "keep the cluster after tests are finished.")
	wnodeScale    = flag.Int("wnode-scale", 12, "size of the whisper cluster.")
	dockerTimeout = flag.Duration("docker-timeout", 5*time.Second, "Docker cluster startup timeout.")
)

type containerInfo struct {
	Name    string
	RPC     string
	Metrics string
}

func makeContainerInfos(service string, containers []swarm.Task) ([]containerInfo, error) {
	whisps := []containerInfo{}
	for _, container := range containers {
		hostname := fmt.Sprintf("%s.%d.%s", service, container.Slot, container.ID)
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return whisps, err
		}
		whisps = append(whisps, containerInfo{
			Name:    hostname,
			RPC:     fmt.Sprintf("http://%s:%d", ips[0].String(), 8545),
			Metrics: fmt.Sprintf("http://%s:%d", ips[0].String(), 8080),
		})
	}
	return whisps, nil
}

func runConcurrent(whisps []containerInfo, f func(i int, w containerInfo) error) []error {
	var wg sync.WaitGroup
	errs := make([]error, len(whisps))
	for i, w := range whisps {
		wg.Add(1)
		go func(i int, w containerInfo) {
			defer wg.Done()
			if err := f(i, w); err != nil {
				errs[i] = err
			}
		}(i, w)
	}
	wg.Wait()
	return errs
}

func runWithRetries(retries int, interval time.Duration, f func() error) error { // nolint (unparam)
	for {
		if err := f(); err != nil {
			retries--
			if retries == 0 {
				return err
			}
			time.Sleep(interval)
			continue
		}
		return nil
	}
}
