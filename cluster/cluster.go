package cluster

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"

	"github.com/status-im/status-scale/dockershim"
)

type Cluster struct {
	Prefix  string
	IPAM    *IPAM
	Backend Backend

	Boot, Relay, Users int

	mu        sync.Mutex
	netID     string
	bootnodes []Bootnode
	relays    []Peer
	users     []Peer
}

func (c *Cluster) getName(parts ...string) string {
	fqn := []string{c.Prefix}
	fqn = append(fqn, parts...)
	return strings.Join(fqn, "_")
}

func (c *Cluster) Create(ctx context.Context) error {
	log.Debug(
		"Creating cluster.", "name", c.Prefix, "cidr", c.IPAM,
		"boot count", c.Boot, "relay count", c.Relay, "users count", c.Users)
	netID, err := c.Backend.CreateNetwork(ctx, dockershim.NetOpts{
		NetName: c.getName("net"),
		CIDR:    c.IPAM.String(),
	})
	if err != nil {
		return err
	}
	c.netID = netID

	var enodes []string
	for i := 0; i < c.Boot; i++ {
		b := NewBootnode(BootnodeConfig{
			Name:    c.getName("boot", strconv.Itoa(i)),
			Network: netID,
			IP:      c.IPAM.Take().String(),
		}, c.Backend)
		c.bootnodes = append(c.bootnodes, b)
		if err := b.Create(ctx); err != nil {
			return err
		}
		enodes = append(enodes, b.Self().String())
	}

	run := newRunner(c.Relay + c.Users)
	for i := 0; i < c.Relay; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName("relay", strconv.Itoa(i))
		cfg.NetID = netID
		cfg.BootNodes = enodes
		cfg.TopicSearch = map[string]string{
			"whisper": "5,7",
		}
		cfg.TopicRegister = []string{"whisper"}
		p := NewPeer(cfg, c.Backend)
		run.Run(func() error {
			err := p.Create(ctx)
			if err == nil {
				c.mu.Lock()
				c.relays = append(c.relays, p)
				c.mu.Unlock()
			}
			return err
		})
	}
	for i := 0; i < c.Users; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName("user", strconv.Itoa(i))
		cfg.NetID = netID
		cfg.BootNodes = enodes
		cfg.TopicSearch = map[string]string{
			"whisper": "2,3",
		}
		p := NewPeer(cfg, c.Backend)
		run.Run(func() error {
			err := p.Create(ctx)
			if err == nil {
				c.mu.Lock()
				c.users = append(c.users, p)
				c.mu.Unlock()
			}
			return err
		})
	}
	err = run.Error()
	log.Debug("finished cluster deployment", "error", err)
	return err
}

func (c *Cluster) GetRelay(n int) Peer {
	return c.relays[n]
}

func (c *Cluster) Clean(ctx context.Context) {
	for i := 0; i < c.Boot; i++ {
		c.Backend.Remove(ctx, c.getName("boot", strconv.Itoa(i)))
	}
	for i := 0; i < c.Relay; i++ {
		c.Backend.Remove(ctx, c.getName("relay", strconv.Itoa(i)))
	}
	for i := 0; i < c.Users; i++ {
		c.Backend.Remove(ctx, c.getName("user", strconv.Itoa(i)))
	}
	log.Debug("removing network", "id", c.netID, "name", c.getName("net"))
	c.Backend.RemoveNetwork(ctx, c.netID)
}

func newRunner(n int) *runner {
	r := &runner{}
	r.wg.Add(n)
	r.errors = make(chan error, n)
	return r
}

type runner struct {
	wg     sync.WaitGroup
	errors chan error
}

func (r *runner) Run(f func() error) {
	go func() {
		r.errors <- f()
		r.wg.Done()
	}()
}

func (r *runner) Error() error {
	r.wg.Wait()
	var b bytes.Buffer
	for {
		select {
		case err := <-r.errors:
			if err != nil {
				b.WriteString(err.Error())
				b.WriteString("\n")
			}
		default:
			if len(b.String()) != 0 {
				return errors.New(b.String())
			}
			return nil
		}
	}
}
