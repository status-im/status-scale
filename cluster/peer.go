package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
	"github.com/status-im/status-scale/whisper"
)

func DefaultConfig() PeerConfig {
	return PeerConfig{
		Whisper:   true,
		Metrics:   true,
		HTTP:      true,
		Host:      "0.0.0.0",
		Modules:   []string{"admin", "debug", "shh", "net"},
		Port:      8545,
		NetworkID: 100,
		Discovery: true,
	}
}

func NewPeer(config PeerConfig, backend Backend) *Peer {
	return &Peer{name: config.Name, config: config, backend: backend}
}

type PeerConfig struct {
	Name  string
	NetID string
	IP    string
	Image string

	Modules         []string
	Whisper         bool
	BootNodes       []string
	RendezvousNodes []string
	NetworkID       int
	HTTP            bool
	Port            int
	Host            string
	Metrics         bool
	TopicSearch     map[string]string
	TopicRegister   []string
	Discovery       bool
	Standalone      bool
}

type Peer struct {
	name   string
	config PeerConfig

	backend Backend

	client *rpc.Client
}

func (p *Peer) String() string {
	return fmt.Sprintf("peer %s %s", p.name, p.config.IP)
}

func (p *Peer) Create(ctx context.Context) error {
	cmd := []string{"statusd"}
	var exposed []string
	if p.config.Whisper {
		cmd = append(cmd, "-shh")
	}
	if p.config.HTTP {
		cmd = append(cmd, "-http")
		if len(p.config.Host) != 0 {
			cmd = append(cmd, "-httphost="+p.config.Host)
		}
		if p.config.Port != 0 {
			cmd = append(cmd, "-httpport="+strconv.Itoa(p.config.Port))
			exposed = append(exposed, strconv.Itoa(p.config.Port))
		}
		if len(p.config.Modules) != 0 {
			cmd = append(cmd, "-httpmodules="+strings.Join(p.config.Modules, ","))
		}
	}
	cmd = append(cmd, "-debug")
	if p.config.Metrics {
		cmd = append(cmd, "-metrics")
	}
	if len(p.config.BootNodes) != 0 {
		cmd = append(cmd, "-dtype=ethv5")
		cmd = append(cmd, "-bootnodes="+strings.Join(p.config.BootNodes, ","))
	}
	if len(p.config.RendezvousNodes) != 0 {
		cmd = append(cmd, "-dtype=ethvousv1")
		for _, n := range p.config.RendezvousNodes {
			cmd = append(cmd, "-rendnode="+n)
		}
	}
	if !p.config.Standalone {
		cmd = append(cmd, "-standalone=false")
	}
	if p.config.Discovery {
		cmd = append(cmd, "-discovery=true")
	}
	if p.config.NetworkID != 0 {
		cmd = append(cmd, "-networkid="+strconv.Itoa(p.config.NetworkID))
	}
	for _, topic := range p.config.TopicRegister {
		cmd = append(cmd, strings.Join([]string{"-topic.register", topic}, "="))
	}
	for topic, args := range p.config.TopicSearch {
		cmd = append(cmd, strings.Join([]string{"-topic.search", topic, args}, "="))
	}
	cmd = append(cmd, "-log", "trace")
	cmd = append(cmd, "-listenaddr", fmt.Sprintf("%s:30303", p.IP()))
	log.Debug("Create statusd", "name", p.name, "command", strings.Join(cmd, " "))
	err := p.backend.Create(ctx, p.name, dockershim.CreateOpts{
		Cmd:   cmd,
		Image: p.config.Image,
		Ports: exposed,
		IPs: map[string]dockershim.IpOpts{p.config.NetID: dockershim.IpOpts{
			IP:    p.config.IP,
			NetID: p.config.NetID,
		}},
	})
	if err != nil {
		return err
	}
	p.client, err = p.makeRPCClient(ctx)
	if err != nil {
		return err
	}
	return p.healthcheck(ctx, 20, time.Second)
}

func (p Peer) Remove(ctx context.Context) error {
	log.Debug("removing statusd", "name", p.name)
	return p.backend.Remove(ctx, p.name)
}

func (p Peer) EnableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStart(func(ctx context.Context, cmd []string) error {
		log.Debug("run command", "peer", p.name, "command", strings.Join(cmd, " "))
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opts...)
}

func (p Peer) IP() string {
	return p.config.IP
}

func (p Peer) DisableConditions(ctx context.Context, opts ...network.Options) error {
	return network.ComcastStop(func(ctx context.Context, cmd []string) error {
		log.Debug("run command", "peer", p.name, "command", strings.Join(cmd, " "))
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opts...)
}

func (p Peer) makeRPCClient(ctx context.Context) (*rpc.Client, error) {
	ports, err := p.backend.ConnectionInfo(ctx, p.name, p.config.Port)
	if err != nil {
		return nil, err
	}
	if len(ports) < 1 {
		return nil, fmt.Errorf("peer %s doesn't have any bindings", p.name)
	}
	// can use any
	rawurl := fmt.Sprintf("http://%s:%s", ports[0].HostIP, ports[0].HostPort)
	log.Debug("init rpc client", "name", p.name, "url", rawurl)
	return rpc.DialContext(ctx, rawurl)
}

func (p Peer) Admin() Admin {
	return Admin{client: p.client}
}

func (p Peer) Whisper() *whisper.Client {
	return whisper.New(p.client)
}

func (p Peer) UID() string {
	return p.name
}

func (p Peer) RawMetrics(ctx context.Context) ([]byte, error) {
	rst := json.RawMessage{}
	err := p.client.CallContext(ctx, &rst, "debug_metrics", true)
	log.Trace("fetched metrics", "peer", p.name, "metrics", string(rst))
	return []byte(rst), err
}

func (p Peer) healthcheck(ctx context.Context, retries int, interval time.Duration) error {
	log.Debug("running healthcheck", "peer", p.name)
	var ignored struct{}
	for i := retries; i > 0; i-- {
		// use deadline?
		err := p.client.CallContext(ctx, &ignored, "admin_nodeInfo")
		log.Trace("healtcheck", "peer", p.name, "tries", i, "error", err)
		if err != nil {
			time.Sleep(interval)
			continue
		}
		return nil
	}
	return fmt.Errorf("peer %s failed healthcheck", p.name)
}

func (p *Peer) Reboot(ctx context.Context) (err error) {
	log.Debug("reboot", "peer", p.name)
	if err = p.backend.Reboot(ctx, p.name); err != nil {
		return err
	}
	p.client, err = p.makeRPCClient(ctx)
	if err != nil {
		return err
	}
	return p.healthcheck(ctx, 20, time.Second)
}
