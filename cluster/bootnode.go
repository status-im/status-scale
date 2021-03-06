package cluster

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discv5"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

// FIXME config is redundant, either use config everywhere as with Peer or remove it
type BootnodeConfig struct {
	Name    string
	IP      string
	Network string
	Enodes  []string
	Image   string
}

func NewBootnode(cfg BootnodeConfig, backend Backend) Bootnode {
	key, err := crypto.GenerateKey() // it can fail only if rand.Reader will return err on read all
	if err != nil {
		panic(err)
	}
	return Bootnode{
		name:    cfg.Name,
		ip:      cfg.IP,
		network: cfg.Network,
		port:    30404,
		backend: backend,
		key:     key,
		enodes:  cfg.Enodes,
		image:   cfg.Image,
	}
}

// TODO bootnode should store parts reusable between rendezvous and discv5. currently
// we patch this struct in rendezvous. which is ok but not clean!
type Bootnode struct {
	name    string
	ip      string
	port    int
	network string
	enodes  []string
	image   string

	backend Backend
	key     *ecdsa.PrivateKey
}

func (b Bootnode) IP() string {
	return b.ip
}

func (b Bootnode) String() string {
	return fmt.Sprintf("bootnode %s %s", b.name, b.ip)
}

func (b Bootnode) Create(ctx context.Context) error {
	data := hex.EncodeToString(crypto.FromECDSA(b.key))
	cmd := []string{"-addr=" + fmt.Sprintf("%s:%d", b.ip, b.port), "-keydata=" + data}
	for _, e := range b.enodes {
		cmd = append(cmd, "-n="+e)
	}
	log.Debug("creating bootnode", "name", b.name, "enode", b.Self().String(), "cmd", strings.Join(cmd, " "))
	return b.backend.Create(ctx, b.name, dockershim.CreateOpts{
		Entrypoint: "bootnode",
		Cmd:        cmd,
		Image:      b.image,
		IPs: map[string]dockershim.IpOpts{b.network: dockershim.IpOpts{
			IP:    b.ip,
			NetID: b.network,
		}},
	},
	)
}

func (b Bootnode) Self() *discv5.Node {
	return discv5.NewNode(
		discv5.PubkeyID(&b.key.PublicKey),
		net.ParseIP(b.ip), uint16(b.port), uint16(b.port))
}

func (b Bootnode) Remove(ctx context.Context) error {
	log.Debug("remove bootnode", "name", b.name)
	return b.backend.Remove(ctx, b.name)
}

func (b Bootnode) EnableConditions(ctx context.Context, opt network.Options) error {
	return network.ComcastStart(func(ctx context.Context, cmd []string) error {
		return b.backend.Execute(ctx, b.name, cmd)
	}, ctx, opt)
}

func (b Bootnode) DisableConditions(ctx context.Context, opt network.Options) error {
	return network.ComcastStop(func(ctx context.Context, cmd []string) error {
		return b.backend.Execute(ctx, b.name, cmd)
	}, ctx, opt)
}

func (b Bootnode) Reboot(ctx context.Context) error {
	log.Debug("reboot", "bootnode", b.name)
	return b.backend.Reboot(ctx, b.name)
}
