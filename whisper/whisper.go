package whisper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/whisper/whisperv6"
)

type Client struct {
	rpcClient *rpc.Client
}

func New(c *rpc.Client) *Client {
	return &Client{rpcClient: c}
}

func (c *Client) Post(msg whisperv6.NewMessage) (hash common.Hash, err error) {
	err = c.rpcClient.Call(&hash, "shhext_post", msg)
	return
}

func (c *Client) Drained(topic whisperv6.TopicType) (rst bool, err error) {
	return rst, c.rpcClient.Call(&rst, "shhext_drained", topic)
}

func (c *Client) DebugPost(msg whisperv6.NewMessage) (hash common.Hash, err error) {
	err = c.rpcClient.Call(&hash, "debug_postSync", msg)
	return
}

func (c *Client) NewSymKey() (symID string, err error) {
	err = c.rpcClient.Call(&symID, "shh_newSymKey")
	return
}
