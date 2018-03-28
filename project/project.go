/*
Package project provides light-weight wrapper around docker-compose and docker client.

Main purpose of the package is to bootstrap a docker-compose cluster with parametrized parameters,
wait till containers in cluster are ready, get containers ip addresses and tear down a cluster.
*/
package project

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

var (
	defaultDockerTimeout = 5 * time.Second
)

// Project is a wrapper around docker-compose project.
type Project struct {
	Path string
	Name string

	client *client.Client
}

// UpOpts used to provide options for docker-compose up.
type UpOpts struct {
	Scale map[string]int
	Wait  time.Duration
}

// New initializes a Project.
func New(fullpath, name string, client *client.Client) Project {
	return Project{
		Path:   fullpath,
		Name:   strings.ToLower(name),
		client: client,
	}
}

// Up runs docker-compose up with options and waits till containers are running.
func (p Project) Up(opts UpOpts) error {
	cmd := exec.Command("docker", "stack", "deploy", "--compose-file", p.Path, p.Name) // nolint (gas)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}
	return p.Scale(opts)

}

func (p Project) Scale(opts UpOpts) error {
	args := []string{"service", "scale"}
	for service, value := range opts.Scale {
		args = append(args, fmt.Sprintf("%s_%s=%d", p.Name, service, value))
	}
	cmd := exec.Command("docker", args...) // nolint (gas)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}
	return nil
}

// Down runs docker-compose down.
func (p Project) Down() error {
	out, err := exec.Command("docker", "stack", "rm", p.Name).CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}
	return nil
}

// FilterOpts used to parametrize a query for a list of containers.
type FilterOpts struct {
	SvcName string
}

// Containers queries docker for containers and filters results according to FiltersOpts.
func (p Project) Containers(f FilterOpts) (rst []swarm.Task, err error) {
	args := filters.NewArgs()
	args.Add("service", fmt.Sprintf("%s_%s", p.Name, f.SvcName))
	args.Add("desired-state", "running")
	return p.client.TaskList(context.TODO(), types.TaskListOptions{
		Filters: args,
	})
}
