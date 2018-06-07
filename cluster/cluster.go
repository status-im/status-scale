package cluster

import (
	"context"
	"strconv"
	"strings"

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
