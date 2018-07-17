package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/metrics"
)

// Fixme standardize access to bootnodes and peers

type Creatable interface {
	Create(context.Context) error
}

type Removable interface {
	Remove(context.Context) error
}

type Rebootable interface {
	Reboot(context.Context) error
}

type PeerType string

type ScaleOpts struct {
	Boot       int
	Relay      int
	Users      int
	Rendezvous int
	Deploy     bool
	Enodes     []string
}

const (
	Boot           PeerType = "boot"
	Relay          PeerType = "relay"
	User           PeerType = "user"
	RendezvousBoot PeerType = "rendezvous"
)

func NewCluster(pref string, ipam *IPAM, b Backend, statusd, bootnode, rendezvous string, keep bool) Cluster {
	c := Cluster{
		Prefix:         pref,
		IPAM:           ipam,
		Backend:        b,
		Statusd:        statusd,
		Bootnode:       bootnode,
		RendezvousBoot: rendezvous,
		Keep:           keep,

		pending: map[PeerType][]interface{}{},
		running: map[PeerType][]interface{}{},
	}
	return c
}

type Cluster struct {
	Prefix  string
	IPAM    *IPAM
	Backend Backend

	// images
	Statusd        string
	Bootnode       string
	RendezvousBoot string

	// dont remove cluster after tests are finished
	Keep bool

	mu      sync.Mutex
	netID   string
	pending map[PeerType][]interface{}
	running map[PeerType][]interface{}
}

func (c *Cluster) getName(parts ...string) string {
	fqn := []string{c.Prefix}
	fqn = append(fqn, parts...)
	return strings.Join(fqn, "_")
}

func (c *Cluster) Create(ctx context.Context, opts ScaleOpts) error {
	return c.create(ctx, opts)
}

func (c *Cluster) totalOfType(typ PeerType) int {
	return len(c.pending[typ]) + len(c.running[typ])
}

func (c *Cluster) create(ctx context.Context, opts ScaleOpts) error {
	log.Debug(
		"Adding nodes to cluster.", "name", c.Prefix, "cidr", c.IPAM,
		"boot count", opts.Boot, "relay count", opts.Relay, "users count", opts.Users,
		"enodes", opts.Enodes,
	)
	netID, err := c.Backend.EnsureNetwork(ctx, dockershim.NetOpts{
		NetName: c.getName("net"),
		CIDR:    c.IPAM.String(),
		NetID:   c.netID,
	})
	if err != nil {
		return err
	}
	c.netID = netID
	var (
		enodes          []string
		rendezvousNodes []string
	)
	if opts.Enodes != nil {
		enodes = opts.Enodes
	}
	boot := c.totalOfType(Boot)
	relay := c.totalOfType(Relay)
	users := c.totalOfType(User)
	rendezvous := c.totalOfType(RendezvousBoot)
	for i := boot; i < boot+opts.Boot; i++ {
		b := NewBootnode(BootnodeConfig{
			Name:    c.getName(string(Boot), strconv.Itoa(i)),
			Network: netID,
			IP:      c.IPAM.Take().String(),
			Enodes:  enodes,
			Image:   c.Bootnode,
		}, c.Backend)
		c.pending[Boot] = append(c.pending[Boot], b)
		if opts.Enodes == nil {
			enodes = append(enodes, b.Self().String())
		}
	}
	// if nil collect both running and pending
	if opts.Enodes == nil {
		for _, p := range c.running[Boot] {
			enodes = append(enodes, p.(Bootnode).Self().String())
		}
	}
	for i := rendezvous; i < rendezvous+opts.Rendezvous; i++ {
		r := Rendezvous(NewBootnode(BootnodeConfig{
			Name:    c.getName(string(RendezvousBoot), strconv.Itoa(i)),
			Network: netID,
			IP:      c.IPAM.Take().String(),
			Image:   c.RendezvousBoot,
		}, c.Backend))
		c.pending[Boot] = append(c.pending[RendezvousBoot], r)
		rendezvousNodes = append(rendezvousNodes, r.Addr())
	}
	for _, p := range c.running[RendezvousBoot] {
		rendezvousNodes = append(rendezvousNodes, p.(Rendezvous).Addr())
	}

	for i := relay; i < relay+opts.Relay; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName(string(Relay), strconv.Itoa(i))
		cfg.NetID = netID
		cfg.IP = c.IPAM.Take().String()
		cfg.BootNodes = enodes
		cfg.RendezvousNodes = rendezvousNodes
		cfg.Image = c.Statusd
		cfg.TopicSearch = map[string]string{
			"whisper": "5,7",
		}
		cfg.TopicRegister = []string{"whisper"}
		p := NewPeer(cfg, c.Backend)
		log.Trace("adding relay peer to pending", "name", cfg.Name, "ip", cfg.IP)
		c.pending[Relay] = append(c.pending[Relay], p)
	}
	for i := users; i < users+opts.Users; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName(string(User), strconv.Itoa(i))
		cfg.NetID = netID
		cfg.Image = c.Statusd
		cfg.IP = c.IPAM.Take().String()
		cfg.BootNodes = enodes
		cfg.RendezvousNodes = rendezvousNodes
		cfg.TopicSearch = map[string]string{
			"whisper": "2,2",
		}
		p := NewPeer(cfg, c.Backend)
		c.pending[User] = append(c.pending[User], p)
	}
	if opts.Deploy {
		return c.DeployPending(ctx)
	}
	return nil
}

func (c *Cluster) DeployPending(ctx context.Context) error {
	run := newRunner(len(c.pending[Boot]) + len(c.pending[Relay]) + len(c.pending[User]))
	for typ, peers := range c.pending {
		for i := range peers {
			typ := typ
			p := peers[i]
			run.Run(func() error {
				err := p.(Creatable).Create(ctx)
				c.mu.Lock()
				c.running[typ] = append(c.running[typ], p)
				c.mu.Unlock()
				if err != nil {
					return fmt.Errorf("error creating %v: %v", p, err)
				}
				return nil
			})
		}
	}
	c.pending = map[PeerType][]interface{}{}
	err := run.Error()
	log.Debug("finished cluster deployment", "error", err)
	return err
}

func (c *Cluster) GetPendingRelay(n int) *Peer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.pending[Relay])-1 {
		return nil
	}
	return c.pending[Relay][n].(*Peer)
}

func (c *Cluster) GetRelay(n int) *Peer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[Relay])-1 {
		return nil
	}
	return c.running[Relay][n].(*Peer)
}

func (c *Cluster) GetBootnode(n int) Bootnode {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[Boot])-1 {
		return Bootnode{}
	}
	return c.running[Boot][n].(Bootnode)
}

func (c *Cluster) Reboot(ctx context.Context, t PeerType) error {
	if _, ok := c.running[t]; !ok {
		return fmt.Errorf("type %v not found in running", t)
	}
	r := newRunner(len(c.running[t]))
	for i := range c.running[t] {
		p := c.running[t][i].(Rebootable)
		r.Run(func() error {
			return p.Reboot(ctx)
		})
	}
	return r.Error()

}

func (c *Cluster) Clean(ctx context.Context) {
	if c.Keep {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, peers := range c.running {
		for _, p := range peers {
			if err := p.(Removable).Remove(ctx); err != nil {
				log.Error("error removing", "peer", p, "error", err)
			}
		}
	}
	log.Debug("removing network", "id", c.netID, "name", c.getName("net"))
	if err := c.Backend.RemoveNetwork(ctx, c.netID); err != nil {
		log.Error("error removing", "network", c.getName("net"), "error", err)
	}
}

type MetricsOpts struct {
	NoRelay bool
	NoUsers bool
}

func (c *Cluster) FillMetrics(ctx context.Context, tab *metrics.Table, opts MetricsOpts) error {
	groups := []PeerType{}
	n := 0
	if !opts.NoRelay {
		groups = append(groups, Relay)
		n += len(c.running[Relay])
	}
	if !opts.NoUsers {
		groups = append(groups, User)
		n += len(c.running[User])
	}
	r := newRunner(n)
	for _, g := range groups {
		for i := range c.running[g] {
			g := g
			i := i
			r.Run(func() error {
				p := c.running[g][i].(*Peer)
				log.Debug("fetching metrics for", "peer", p.UID())
				data, err := p.RawMetrics(ctx)
				if err != nil {
					return err
				}
				return tab.Append(p.UID(), data)
			})
		}
	}
	return r.Error()
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
