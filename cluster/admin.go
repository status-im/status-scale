package cluster

import (
	"context"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
)

type Admin struct {
	client *rpc.Client
}

func (a Admin) Self(ctx context.Context) (rst *p2p.NodeInfo, err error) {
	return rst, a.client.CallContext(ctx, &rst, "admin_nodeInfo")
}

func (a Admin) Peers(ctx context.Context) (rst []*p2p.PeerInfo, err error) {
	return rst, a.client.CallContext(ctx, &rst, "admin_peers")
}

func (a Admin) AddPeer(ctx context.Context, enode string) error {
	return a.client.CallContext(ctx, nil, "admin_addPeer", enode)
}
