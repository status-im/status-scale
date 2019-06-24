package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
	"github.com/status-im/status-scale/utils"
)

// FIXME(dshulyak) implement single Peer interface
type Creatable interface {
	Create(context.Context) error
}

type Removable interface {
	Remove(context.Context) error
}

type Rebootable interface {
	Reboot(context.Context) error
}

type Enforsable interface {
	EnableConditions(ctx context.Context, opts ...network.Options) error
	DisableConditions(ctx context.Context, opts ...network.Options) error
}

type AssignedIP interface {
	IP() string
}

type PeerType string

type ScaleOpts struct {
	Boot            int
	Relay           int
	Users           int
	MVDS            int
	Rendezvous      int
	Mails           int
	Deploy          bool
	Enodes          []string
	RendezvousNodes []string
}

const (
	Boot           PeerType = "boot"
	Relay          PeerType = "relay"
	Mail           PeerType = "mail"
	User           PeerType = "user"
	MVDS           PeerType = "mvds"
	RendezvousBoot PeerType = "rendezvous"
)

func NewCluster(pref string, ipam *IPAM, b Backend, statusd, client, bootnode, rendezvous string, keep bool) Cluster {
	c := Cluster{
		Prefix:         pref,
		IPAM:           ipam,
		Backend:        b,
		Statusd:        statusd,
		Client:         client,
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
	Client         string
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
		"boot count", opts.Boot, "relay count", opts.Relay, "users count", opts.Users, "mvds count", opts.MVDS, "mailservers count", opts.Mails,
		"rendezvous count", opts.Rendezvous, "enodes", opts.Enodes, "rendezvous", opts.RendezvousNodes,
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
		mailservers     []string
	)
	for _, p := range c.running[Mail] {
		mailservers = append(mailservers, p.(*Peer).Enode())
	}

	if opts.Enodes != nil {
		enodes = opts.Enodes
	}
	if opts.RendezvousNodes != nil {
		rendezvousNodes = opts.RendezvousNodes
	}
	// FIXME(dshulyak) there is definitely reusable pattern.
	// note that bootnodes and mail servers have to be created before and passed to relays/users
	boot := c.totalOfType(Boot)
	relay := c.totalOfType(Relay)
	users := c.totalOfType(User)
	mails := c.totalOfType(Mail)
	mvds := c.totalOfType(MVDS)
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
		c.pending[RendezvousBoot] = append(c.pending[RendezvousBoot], r)
		rendezvousNodes = append(rendezvousNodes, r.Addr())
	}
	if opts.RendezvousNodes == nil {
		for _, p := range c.running[RendezvousBoot] {
			rendezvousNodes = append(rendezvousNodes, p.(Rendezvous).Addr())
		}
	}

	for i := mails; i < mails+opts.Mails; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName(string(Mail), strconv.Itoa(i))
		cfg.NetID = netID
		cfg.IP = c.IPAM.Take().String()
		cfg.BootNodes = enodes
		cfg.RendezvousNodes = rendezvousNodes
		cfg.Image = c.Statusd
		cfg.Mailserver = true
		cfg.TopicSearch = map[string]string{
			"whisper": "5,7",
		}
		cfg.TopicRegister = []string{"whisper", "mail"}
		p := NewStatusd(cfg, c.Backend)
		c.pending[Mail] = append(c.pending[Mail], p)
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
		p := NewStatusd(cfg, c.Backend)
		log.Trace("adding relay peer to pending", "name", cfg.Name, "ip", cfg.IP)
		c.pending[Relay] = append(c.pending[Relay], p)
	}
	for i := users; i < users+opts.Users; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName(string(User), strconv.Itoa(i))
		cfg.NetID = netID
		cfg.Image = c.Client
		cfg.IP = c.IPAM.Take().String()
		cfg.BootNodes = enodes
		cfg.RendezvousNodes = rendezvousNodes
		cfg.Mailservers = mailservers
		cfg.TopicSearch = map[string]string{
			"whisper": "2,2",
		}
		identity, err := crypto.GenerateKey()
		if err != nil {
			return err
		}
		p := NewClient(cfg, c.Backend, identity)
		log.Trace("adding user peer to pending", "name", cfg.Name, "ip", cfg.IP)
		c.pending[User] = append(c.pending[User], p)
	}
	for i := mvds; i < mvds+opts.MVDS; i++ {
		cfg := DefaultConfig()
		cfg.Name = c.getName(string(MVDS), strconv.Itoa(i))
		cfg.NetID = netID
		cfg.Image = c.Client
		cfg.IP = c.IPAM.Take().String()
		cfg.BootNodes = enodes
		cfg.RendezvousNodes = rendezvousNodes
		cfg.Mailservers = mailservers
		cfg.TopicSearch = map[string]string{
			"whisper": "2,2",
		}
		identity, err := crypto.GenerateKey()
		if err != nil {
			return err
		}
		p := NewClient(cfg, c.Backend, identity)
		log.Trace("adding mvds peer to pending", "name", cfg.Name, "ip", cfg.IP)
		c.pending[MVDS] = append(c.pending[MVDS], p)
	}
	if opts.Deploy {
		return c.DeployPending(ctx)
	}
	return nil
}

func (c *Cluster) DeployPending(ctx context.Context) error {
	total := 0
	for _, peers := range c.pending {
		total += len(peers)
	}
	run := utils.NewGroup(ctx, total)
	for typ, peers := range c.pending {
		log.Info("pending", "type", typ, "len", len(peers))
		for i := range peers {
			typ := typ
			p := peers[i]
			run.Run(func(ctx context.Context) error {
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

func (c *Cluster) GetRelays() []*Peer {
	rst := make([]*Peer, len(c.running[Relay]))
	for i := range c.running[Relay] {
		rst[i] = c.running[Relay][i].(*Peer)
	}
	return rst
}

func (c *Cluster) GetUsers() []*Client {
	rst := make([]*Client, len(c.running[User]))
	for i := range c.running[User] {
		rst[i] = c.running[User][i].(*Client)
	}
	return rst
}

func (c *Cluster) GetMVDS(n int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[MVDS])-1 {
		return nil
	}
	return c.running[MVDS][n].(*Client)
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

func (c *Cluster) GetPendingUser(n int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.pending[User])-1 {
		return nil
	}
	return c.pending[User][n].(*Client)
}

func (c *Cluster) GetUser(n int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[User])-1 {
		return nil
	}
	return c.running[User][n].(*Client)
}

func (c *Cluster) GetBootnode(n int) Bootnode {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[Boot])-1 {
		return Bootnode{}
	}
	return c.running[Boot][n].(Bootnode)
}

func (c *Cluster) GetRendezvous(n int) Rendezvous {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.running[RendezvousBoot])-1 {
		return Rendezvous{}
	}
	return c.running[RendezvousBoot][n].(Rendezvous)
}

func (c *Cluster) Clean(ctx context.Context) {
	if c.Keep {
		return
	}
	log.Info("cleaning environment", "prefix", c.Prefix)
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

func (c *Cluster) EnableConditionsGloobally(ctx context.Context, opt network.Options) error {
	total := 0
	// FIXME(dshulyak) set interface for a peer. i am using set of methods in the cluster
	// that every peer should provide for correctness.
	for _, peers := range c.running {
		total += len(peers)
	}
	group := utils.NewGroup(ctx, total)
	for _, peers := range c.running {
		for _, p := range peers {
			typed := p.(Enforsable)
			group.Run(func(ctx context.Context) error {
				return typed.EnableConditions(ctx, opt)
			})
		}
	}
	return group.Error()
}

func (c *Cluster) AllIPs() (rst []string) {
	for _, group := range c.running {
		for _, p := range group {
			a, ok := p.(AssignedIP)
			if ok {
				rst = append(rst, a.IP())
			}
		}
	}
	return rst
}
