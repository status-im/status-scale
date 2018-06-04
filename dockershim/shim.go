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
	"docker.io/go-docker/api/types/swarm"
	"docker.io/go-docker/api/types/volume"
)

func NewPeer(client *docker.Client, name string) DockerPeer {
	return DockerPeer{
		client: client,
		name:   name,
	}
}

type DockerPeer struct {
	client *docker.Client
	name   string
}

func (p DockerPeer) Execute(ctx context.Context, cmd []string) error {
	resp, err := p.client.ContainerExecCreate(ctx, p.name, types.ExecConfig{
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

func (p DockerPeer) Create(ctx context.Context, cmd []string, image string) error {
	_, err := p.client.ContainerCreate(ctx, &container.Config{
		Cmd:   cmd,
		Image: image,
	}, nil, nil, p.name)
	return err
}

func (p DockerPeer) Remove(ctx context.Context) error {
	return p.client.ContainerRemove(ctx, p.name, types.ContainerRemoveOptions{Force: true})
}

func (p DockerPeer) CreateConfig(ctx context.Context, name string, data []byte) error {
	p.client.VolumeCreate(ctx, volume.VolumesCreateBody{})
	resp, err := p.client.ConfigCreate(ctx, swarm.ConfigSpec{
		Annotations: swarm.Annotations{Name: name},
		Data:        data,
	})
}
