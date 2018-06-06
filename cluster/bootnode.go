package cluster

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discv5"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

type BootnodeConfig struct {
	Name    string
	IP      string
	Network string
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
	}
}

type Bootnode struct {
	name    string
	ip      string
	port    int
	network string

	backend Backend
	key     *ecdsa.PrivateKey
}

func (b Bootnode) Create(ctx context.Context) error {
	log.Debug("creating bootnode", "name", b.name, "enode", b.Self().String())
	data := hex.EncodeToString(crypto.FromECDSA(b.key))
	return b.backend.Create(ctx, b.name, dockershim.CreateOpts{
		Entrypoint: "bootnode",
		Cmd:        []string{"-addr=" + fmt.Sprintf("%s:%d", b.ip, b.port), "-keydata=" + data},
		Image:      "statusteam/bootnode:latest",
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

func (b Bootnode) EnableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStart(func(ctx context.Context, cmd []string) error {
		return b.backend.Execute(ctx, b.name, cmd)
	}, ctx, opts...)
}

func (b Bootnode) DisableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStop(func(ctx context.Context, cmd []string) error {
		return b.backend.Execute(ctx, b.name, cmd)
	}, ctx, opts...)
}
