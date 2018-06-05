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
	"docker.io/go-docker/api/types/network"
)

func NewShim(client *docker.Client) DockerPeer {
	return DockerPeer{
		client: client,
	}
}

type IpOpts struct {
	NetID string
	IP    string
}

type CreateOpts struct {
	Entrypoint string
	Cmd        []string
	Image      string
	IPs        map[string]IpOpts
}

type NetOpts struct {
	NetName string
	CIDR    string
	Driver  string
}

type DockerPeer struct {
	client *docker.Client
}

func (p DockerPeer) Execute(ctx context.Context, id string, cmd []string) error {
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

func (p DockerPeer) Create(ctx context.Context, id string, opts CreateOpts) error {
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
	_, err := p.client.ContainerCreate(ctx, &container.Config{
		Entrypoint: []string{opts.Entrypoint},
		Cmd:        opts.Cmd,
		Image:      opts.Image,
	}, nil, &network.NetworkingConfig{
		EndpointsConfig: endpoints,
	}, id)
	if err != nil {
		return err
	}
	return p.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
}

func (p DockerPeer) CreateNetwork(ctx context.Context, opts NetOpts) (string, error) {
	rst, err := p.client.NetworkCreate(ctx, opts.NetName, types.NetworkCreate{
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{{Subnet: opts.CIDR}},
		},
	})
	return rst.ID, err
}

func (p DockerPeer) RemoveNetwork(ctx context.Context, netID string) error {
	return p.client.NetworkRemove(ctx, netID)
}

func (p DockerPeer) Remove(ctx context.Context, id string) error {
	return p.client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
}
