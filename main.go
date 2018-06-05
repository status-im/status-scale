package main

import (
	"context"
	"fmt"
	"net"

	docker "docker.io/go-docker"
	"github.com/spf13/pflag"
	"github.com/status-im/status-scale/cluster"
	"github.com/status-im/status-scale/dockershim"
)

const (
	NETCIDR = "172.0.10.0/24"
)

var (
	name    = pflag.StringP("name", "n", "boot1", "prefix for containers")
	netname = pflag.StringP("network", "t", "boot", "isolated network")
	cidr    = pflag.StringP("cidr", "c", NETCIDR, "network cidr")
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	pflag.Parse()
	_, ipnet, err := net.ParseCIDR(*cidr)
	must(err)
	ip := ipnet.IP
	ip[3] = ip[3] + 5
	createBoot(ip.String())
}

func createBoot(ip string) {
	client, err := docker.NewEnvClient()
	must(err)
	b := dockershim.NewShim(client)
	netID, err := b.CreateNetwork(context.TODO(), dockershim.NetOpts{
		NetName: *netname,
		CIDR:    *cidr,
	})
	must(err)
	boot := cluster.NewBootnode(cluster.BootnodeConfig{
		Name:    *name,
		Network: netID,
		IP:      ip,
	}, b)
	must(boot.Create(context.TODO()))
	conf := cluster.DefaultConfig()
	conf.BootNodes = append(conf.BootNodes, boot.Self().String())
	conf.Name = *name + "_w"
	conf.NetID = netID
	p := cluster.NewPeer(conf, b)
	must(p.Create(context.TODO()))
}

func printBoot(ip string) {
	boot := cluster.NewBootnode(cluster.BootnodeConfig{
		Name:    *name,
		Network: "",
		IP:      ip,
	}, nil)
	fmt.Println(boot.Self().String())
}
