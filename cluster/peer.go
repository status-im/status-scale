package cluster

import (
	"context"
	"strconv"
	"strings"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

const (
	STATUSD = "statusteam/statusd-debug:latest"
)

func DefaultConfig() PeerConfig {
	return PeerConfig{
		Whisper:   true,
		Metrics:   true,
		HTTP:      true,
		Host:      "0.0.0.0",
		Port:      8545,
		NetworkID: 100,
	}
}

type PeerBackend interface {
	Execute(context.Context, string, []string) error
	Create(context.Context, string, dockershim.CreateOpts) error
	Remove(context.Context, string) error
}

func NewPeer(config PeerConfig, backend PeerBackend) Peer {
	return Peer{config.Name, config, backend}
}

type PeerConfig struct {
	Name  string
	NetID string

	Modules   []string
	Whisper   bool
	BootNodes []string
	NetworkID int
	HTTP      bool
	Port      int
	Host      string
	Metrics   bool
}

type Peer struct {
	name   string
	config PeerConfig

	backend PeerBackend
}

func (p Peer) Create(ctx context.Context) error {
	cmd := []string{"statusd"}
	if p.config.Whisper {
		cmd = append(cmd, "-shh")
	}
	if p.config.HTTP {
		cmd = append(cmd, "-http")
		if len(p.config.Host) != 0 {
			cmd = append(cmd, "-httphost="+p.config.Host)
		}
		if p.config.Port != 0 {
			cmd = append(cmd, "-httpport="+strconv.Itoa(p.config.Port))
		}
		if len(p.config.Modules) != 0 {
			cmd = append(cmd, "-httpmodules="+strings.Join(p.config.Modules, ","))
		}
	}
	if p.config.Metrics {
		cmd = append(cmd, "-metrics")
	}
	if len(p.config.BootNodes) != 0 {
		cmd = append(cmd, "-bootnodes="+strings.Join(p.config.BootNodes, ","))
	}
	if p.config.NetworkID != 0 {
		cmd = append(cmd, "-networkid="+strconv.Itoa(p.config.NetworkID))
	}
	return p.backend.Create(ctx, p.name, dockershim.CreateOpts{
		Cmd:   cmd,
		Image: STATUSD,
		IPs: map[string]dockershim.IpOpts{p.config.NetID: dockershim.IpOpts{
			NetID: p.config.NetID,
		}},
	})
}

func (p Peer) Remove(ctx context.Context) error {
	return p.backend.Remove(ctx, p.name)
}

func (p Peer) EnableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStart(func(ctx context.Context, cmd []string) error {
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opts...)
}

func (p Peer) DisableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStop(func(ctx context.Context, cmd []string) error {
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opts...)
}
