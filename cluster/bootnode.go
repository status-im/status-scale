package cluster

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/discv5"

	"github.com/status-im/status-scale/network"
)

func NewBootnode(name string, ip string, backend PeerBackend) Bootnode {
	key, err := crypto.GenerateKey() // it can fail only if rand.Reader will return err on read all
	if err != nil {
		panic(err)
	}
	return Bootnode{
		name:    name,
		ip:      ip,
		port:    30404,
		backend: backend,
		key:     key,
	}
}

type Bootnode struct {
	name    string
	ip      string
	port    int
	backend PeerBackend
	key     *ecdsa.PrivateKey
}

func (b Bootnode) Create(ctx context.Context) error {
	data, err := hex.EncodeToString(crypto.FromECDSA(b.key))
	if err != nil {
		return err
	}
	return b.backend.Create(ctx,
		[]string{"bootnode", "-addr", fmt.Sprintf("%s:%d", b.ip, b.port), "-keydata", data},
		"status-go/bootnode:latest")
}

func (b Bootnode) Self() *discv5.Node {
	return discv5.NewNode(
		discv5.PubkeyID(b.key.PublicKey),
		net.ParseIP(b.ip), uint16(b.port), uint16(b.port))
}

func (b Bootnode) Remove(ctx context.Context) error {
	return p.backend.Remove(ctx)
}

func (b Bootnode) EnableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStart(p.backend, ctx, opts...)
}

func (b Bootnode) DisableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStop(p.backend, ctx, opts...)
}
