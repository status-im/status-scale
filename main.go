package main

import (
	"context"
	"os"
	"strings"
	"time"

	docker "docker.io/go-docker"
	"github.com/ethereum/go-ethereum/log"
	"github.com/spf13/pflag"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

const (
	NETCIDR = "172.0.10.0/24"
)

var (
	prefix    = pflag.StringP("prefix", "p", "tests", "prefix for containers")
	cidr      = pflag.StringP("cidr", "c", "172.0.10.0/24", "network cidr")
	verbosity = pflag.StringP("verbosity", "v", "debug", "log level")
	keep      = pflag.BoolP("keep", "k", false, "keep cluster after tests")
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

	clusterf()
}

func clusterf() {
	client, err := docker.NewEnvClient()
	must(err)
	b := dockershim.NewShim(client)

	ipam, err := cluster.NewIPAM(*cidr)
	must(err)
	c := cluster.Cluster{
		Prefix:  *prefix,
		Backend: b,
		IPAM:    ipam,
		Boot:    1,
		Relay:   3,
	}
	if err := c.Create(context.TODO()); err != nil {
		log.Error("creating cluster failed", "error", err)
	}
	defer func() {
		err := recover()
		if !*keep {
			c.Clean(context.TODO())
		}
		if err != nil {
			log.Crit("panic", "error", err)
		}
	}()
	time.Sleep(10 * time.Second)
	peers, err := c.GetRelay(0).Admin().Peers(context.TODO())
	if err != nil {
		log.Error("unable to get info about self", "error", err)
	} else {
		for _, p := range peers {
			log.Info("connected", "ip", p.Network.RemoteAddress)
		}
	}
	c.GetRelay(0).EnableConditions(context.TODO(), network.Options{
		PacketLoss: 100,
		TargetAddr: strings.Split(peers[0].Network.RemoteAddress, ":")[0],
	})
	time.Sleep(10 * time.Second)
	peers, err = c.GetRelay(0).Admin().Peers(context.TODO())
	if err != nil {
		log.Error("unable to get info about self", "error", err)
	} else {
		for _, p := range peers {
			log.Info("connected", "ip", p.Network.RemoteAddress)
		}
	}
	c.GetRelay(0).DisableConditions(context.TODO())
	time.Sleep(10 * time.Second)
	peers, err = c.GetRelay(0).Admin().Peers(context.TODO())
	if err != nil {
		log.Error("unable to get info about self", "error", err)
	} else {
		for _, p := range peers {
			log.Info("connected", "ip", p.Network.RemoteAddress)
		}
	}
}
