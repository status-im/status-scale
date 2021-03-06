package tests

import (
	"flag"
	"os"
	"strings"

	docker "docker.io/go-docker"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/dockershim"
)

var (
	CONF = Config{}
)

func init() {
	flag.StringVar(&CONF.Prefix, "prefix", "tests", "prefix for containers")
	flag.StringVar(&CONF.CIDR, "cidr", "10.0.200.0/24", "network cidr")
	flag.StringVar(&CONF.Verbosity, "log", "info", "log level")
	flag.BoolVar(&CONF.Keep, "keep", false, "keep cluster after tests")
	flag.StringVar(&CONF.Statusd, "statusd", "statusteam/statusd-debug:latest", "image for status go with comcast")
	flag.StringVar(&CONF.Bootnode, "bootnode", "statusteam/bootnode-debug:latest", "image for bootnode with comcast")
	flag.StringVar(&CONF.Rendezvous, "rendezvous", "statusteam/rendezvous-debug:latest", "image for rendezvous with comcast")
	flag.StringVar(&CONF.Client, "client", "statusteam/client-debug:latest", "image for status client with comcast")
	flag.Parse()

	handler := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	level, err := log.LvlFromString(strings.ToLower(CONF.Verbosity))
	if err != nil {
		panic(err)
	}
	log.Root().SetHandler(log.LvlFilterHandler(level, handler))
}

type Config struct {
	Prefix    string
	CIDR      string
	Verbosity string
	Keep      bool

	// images
	Statusd    string
	Client     string
	Bootnode   string
	Rendezvous string
}

func ClusterFromConfig() cluster.Cluster {
	client, err := docker.NewEnvClient()
	if err != nil {
		panic(err)
	}
	ipam, err := cluster.NewIPAM(CONF.CIDR)
	if err != nil {
		panic(err)
	}
	return cluster.NewCluster(
		CONF.Prefix, ipam, dockershim.NewShim(client),
		CONF.Statusd, CONF.Client, CONF.Bootnode, CONF.Rendezvous, CONF.Keep,
	)
}
