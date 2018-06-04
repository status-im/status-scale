package cluster

import (
	"context"

	"github.com/status-im/status-scale/network"
)

const (
	STATUSD = "statusteam/statusd-debug:latest"
)

type PeerBackend interface {
	Execute(ctx context.Context, cmd []string) error
	Create(ctx context.Context, cmd []string, image string) error
	Remove(ctx context.Context) error
}

func NewPeer(name string, config PeerConfig, backend PeerBackend) Peer {
	return Peer{name, config, backend}
}

type PeerConfig struct {
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
	return p.backend.Create(ctx, []string{
		"statusd", "-shh", "-http",
		"-httpmodules=admin,shh,debug",
		"-httphost=0.0.0.0", "httpport=8545"}, STATUSD)
}

func (p Peer) Remove(ctx context.Context) error {
	return p.backend.Remove(ctx)
}

func (p Peer) EnableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStart(p.backend, ctx, opts...)
}

func (p Peer) DisableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStop(p.backend, ctx, opts...)
}
