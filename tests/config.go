package tests

import (
	"flag"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

var (
	CONF = Config{}
)

func init() {
	flag.StringVar(&CONF.Prefix, "prefix", "tests", "prefix for containers")
	flag.StringVar(&CONF.CIDR, "cidr", "10.0.170.0/24", "network cidr")
	flag.StringVar(&CONF.Verbosity, "ver", "info", "log level")
	flag.BoolVar(&CONF.Keep, "keep", false, "keep cluster after tests")
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
}
