package dockershim

import (
	"context"
	"errors"
	"time"

	docker "docker.io/go-docker"
	"docker.io/go-docker/api/types"
)

func NewPeer(client *docker.Client, name string) DockerPeer {
	return DockerPeer{
		client: client,
		name:   name,
		id:     name,
	}
}

type DockerPeer struct {
	client *docker.Client
	name   string
	id     string
}

func (p DockerPeer) Execute(ctx context.Context, cmd []string) error {
	resp, err := p.client.ContainerExecCreate(ctx, p.id, types.ExecConfig{
		Cmd:        cmd,
		Privileged: true,
	})
	if err != nil {
		return err
	}
	err = p.client.ContainerExecStart(context.TODO(), resp.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		inspect, err := p.client.ContainerExecInspect(ctx, resp.ID)
		if err != nil {
			return err
		}
		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return errors.New("failed")
			}
			return nil
		}
	}
	return errors.New("timed out")
}
