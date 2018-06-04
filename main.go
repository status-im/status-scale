package main

import (
	"context"
	"time"

	docker "docker.io/go-docker"
	"github.com/spf13/pflag"
	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

var (
	base = pflag.StringP("base", "n", "test_", "prefix for containers")
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	pflag.Parse()
	client, err := docker.NewEnvClient()
	must(err)
	peer := dockershim.NewPeer(client, *name)
	comcast := network.ComcastExecutor{peer}
	must(comcast.Start(context.TODO(), network.Options{
		Latency:    200,
		TargetAddr: "8.8.8.8",
	}))
	time.Sleep(5 * time.Second)
	must(comcast.Stop(context.TODO()))
}
