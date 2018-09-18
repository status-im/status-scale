package dockershim

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	docker "docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
	"docker.io/go-docker/api/types/mount"
	"docker.io/go-docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

func NewShim(client *docker.Client) DockerShim {
	return DockerShim{
		client: client,
	}
}

// TODO move structs to cluster pkg
type IpOpts struct {
	NetID string
	IP    string
}

type CreateOpts struct {
	Entrypoint          string
	Cmd                 []string
	HostConfigPath      string
	ContainerConfigPath string
	Image               string
	IPs                 map[string]IpOpts
	Ports               []string
}

type NetOpts struct {
	NetName string
	CIDR    string
	Driver  string
	NetID   string
}

type DockerShim struct {
	client *docker.Client
}

func (p DockerShim) Execute(ctx context.Context, id string, cmd []string) error {
	resp, err := p.client.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd:          cmd,
		Privileged:   true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	hj, err := p.client.ContainerExecAttach(context.TODO(), resp.ID, types.ExecConfig{})
	if err != nil {
		return err
	}
	defer hj.Close()
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		inspect, err := p.client.ContainerExecInspect(ctx, resp.ID)
		if err != nil {
			return err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				data, err := ioutil.ReadAll(hj.Conn)
				errmsg := string(data)
				if err != nil {
					errmsg = err.Error()
				}
				return fmt.Errorf("command `%+v` failed:\n%v", strings.Join(cmd, " "), errmsg)
			}
			return nil
		}
	}
	return fmt.Errorf("command `%+v` timed out", strings.Join(cmd, " "))
}

func (p DockerShim) Create(ctx context.Context, id string, opts CreateOpts) error {
	endpoints := map[string]*network.EndpointSettings{}
	for iface, opts := range opts.IPs {
		endpoints[iface] = &network.EndpointSettings{
			IPAMConfig: &network.EndpointIPAMConfig{
				IPv4Address: opts.IP,
			},
			IPAddress: opts.IP,
			NetworkID: opts.NetID,
		}

	}
	ports, portsMap, err := nat.ParsePortSpecs(opts.Ports)
	if err != nil {
		return err
	}
	_, err = p.client.ContainerCreate(ctx, &container.Config{
		Entrypoint:   []string{opts.Entrypoint},
		Cmd:          opts.Cmd,
		Image:        opts.Image,
		ExposedPorts: ports,
	}, &container.HostConfig{
		PortBindings: portsMap,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: opts.HostConfigPath,
				Target: opts.ContainerConfigPath,
			},
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: endpoints,
	}, id)
	if err != nil {
		return err
	}
	return p.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
}

func (p DockerShim) Reboot(ctx context.Context, id string) error {
	timeout := 5 * time.Second
	return p.client.ContainerRestart(ctx, id, &timeout)
}

func (p DockerShim) EnsureNetwork(ctx context.Context, opts NetOpts) (string, error) {
	// check that cidr intersects
	net, err := p.client.NetworkInspect(ctx, opts.NetID, types.NetworkInspectOptions{})
	if err == nil {
		return net.ID, err
	}
	rst, err := p.client.NetworkCreate(ctx, opts.NetName, types.NetworkCreate{
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{{Subnet: opts.CIDR}},
		},
	})
	return rst.ID, err
}

func (p DockerShim) RemoveNetwork(ctx context.Context, netID string) error {
	return p.client.NetworkRemove(ctx, netID)
}

func (p DockerShim) Remove(ctx context.Context, id string) error {
	return p.client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
}

func (p DockerShim) ConnectionInfo(ctx context.Context, name string, target int) ([]nat.PortBinding, error) {
	info, err := p.client.ContainerInspect(ctx, name)
	if err != nil {
		return nil, err
	}
	for port, binding := range info.NetworkSettings.Ports {
		if port.Int() == target {
			return binding, nil
		}
	}
	return nil, fmt.Errorf("no bindings for port %d", target)
}
