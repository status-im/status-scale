package dockershim

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
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
