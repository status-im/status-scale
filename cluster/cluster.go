package cluster

import (
	"context"
	"net"
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
		if err := p.Create(ctx); err != nil {
			return err
		}
		c.relays = append(c.relays, p)
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
		if err := p.Create(ctx); err != nil {
			return err
		}
		c.users = append(c.users, p)
	}
	return nil
}

func (c *Cluster) Clean(ctx context.Context) {
	for _, p := range c.relays {
		p.Remove(ctx)
	}
	for _, p := range c.users {
		p.Remove(ctx)
	}
	for _, b := range c.bootnodes {
		b.Remove(ctx)
	}
	log.Debug("removing network", "id", c.netID, "name", c.getName("net"))
	c.Backend.RemoveNetwork(ctx, c.netID)
}

func NewIPAM(cidr string) (*IPAM, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return &IPAM{
		cidr:  ipnet,
		given: 1, // start from 2
	}, err
}

type IPAM struct {
	cidr *net.IPNet

	mu    sync.Mutex
	given byte
}

// FIXME this will work only for 255 first ips
func (i *IPAM) Take() net.IP {
	i.mu.Lock()
	defer i.mu.Unlock()
	new := make(net.IP, 4)
	copy(new, i.cidr.IP)
	i.given++
	new[3] += i.given
	return new
}

func (i *IPAM) String() string {
	return i.cidr.String()
}
