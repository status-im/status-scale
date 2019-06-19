package cluster

import (
	"context"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-console-client/protocol/v1"
)

type Chat struct {
	client *rpc.Client
}

func (c Chat) AddContact(ctx context.Context, contact gethservice.Contact) error {
	return c.client.CallContext(ctx, nil, "ssm_addContact", contact)
}

func (c Chat) Send(ctx context.Context, contact gethservice.Contact, msg string) error {
	return c.client.CallContext(ctx, nil, "ssm_sendToContact", contact, msg)
}

func (c Chat) Messages(ctx context.Context, contact gethservice.Contact, offset int64) (rst []*protocol.Message, err error) {
	return rst, c.client.CallContext(ctx, &rst, "ssm_readContactMessages", contact, offset)
}
