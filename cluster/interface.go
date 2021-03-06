package cluster

import (
	"context"

	"github.com/docker/go-connections/nat"
	"github.com/status-im/status-scale/dockershim"
)

// TODO(dshulyak) options must be defined in this module
type Backend interface {
	Execute(context.Context, string, []string) error
	Create(context.Context, string, dockershim.CreateOpts) error
	Remove(context.Context, string) error
	EnsureNetwork(context.Context, dockershim.NetOpts) (string, error)
	RemoveNetwork(context.Context, string) error
	ConnectionInfo(context.Context, string, int) ([]nat.PortBinding, error)
	Reboot(context.Context, string) error
}
