package main

import (
	"context"
	"os"
	"strings"

	docker "docker.io/go-docker"
	"github.com/ethereum/go-ethereum/log"
	"github.com/spf13/pflag"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/dockershim"
)

const (
	NETCIDR = "172.0.10.0/24"
)

var (
	prefix    = pflag.StringP("prefix", "p", "tests", "prefix for containers")
	cidr      = pflag.StringP("cidr", "c", "172.0.10.0/24", "network cidr")
	verbosity = pflag.StringP("verbosity", "v", "debug", "log level")
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	pflag.Parse()

	handler := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	level, err := log.LvlFromString(strings.ToLower(*verbosity))
	if err != nil {
		panic(err)
	}
	log.Root().SetHandler(log.LvlFilterHandler(level, handler))

	client, err := docker.NewEnvClient()
	must(err)
	b := dockershim.NewShim(client)

	ipam, err := cluster.NewIPAM(*cidr)
	must(err)
	c := cluster.Cluster{
		Prefix:  *prefix,
		Backend: b,
		IPAM:    ipam,
		Boot:    2,
		Relay:   5,
		Users:   10,
	}
	must(c.Create(context.TODO()))
	c.Clean(context.TODO())
}
