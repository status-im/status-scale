package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/status-im/status-go/params"
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

const (
	tmpSuffix       = "scale-peer-%s-"
	containerConfig = "/conf.json"
)

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

	hostConfig string
}

func (p *Peer) String() string {
	return fmt.Sprintf("peer %s %s", p.name, p.config.IP)
}

func (p *Peer) Create(ctx context.Context) error {
	cmd := []string{"statusd", "-c", containerConfig, "-log", "debug"}
	if p.config.Metrics {
		cmd = append(cmd, "-metrics")
	}
	cfg, err := params.NewNodeConfig("", "", 0)
	if err != nil {
		return err
	}
	cfg.LogEnabled = true
	cfg.LogToStderr = true
	cfg.LogLevel = "DEBUG"
	// Should go to file
	var exposed []string
	if p.config.Whisper {
		cfg.WhisperConfig.Enabled = true
		cfg.WhisperConfig.EnableNTPSync = true
	}
	if p.config.HTTP {
		if len(p.config.Host) != 0 {
			cfg.HTTPHost = p.config.Host
		}
		if p.config.Port != 0 {
			cfg.HTTPPort = p.config.Port
			exposed = append(exposed, strconv.Itoa(p.config.Port))
		}
		if len(p.config.Modules) != 0 {
			cfg.APIModules = strings.Join(p.config.Modules, ",")
		}
	}
	cfg.DebugAPIEnabled = true
	cfg.ClusterConfig.Enabled = true
	if len(p.config.BootNodes) != 0 {
		cfg.NoDiscovery = false
		cfg.ClusterConfig.BootNodes = p.config.BootNodes
	}
	if len(p.config.RendezvousNodes) != 0 {
		cfg.Rendezvous = true
		cfg.ClusterConfig.RendezvousNodes = p.config.RendezvousNodes
	}
	for _, topic := range p.config.TopicRegister {
		cfg.RegisterTopics = append(cfg.RegisterTopics, discv5.Topic(topic))
	}
	for topic, args := range p.config.TopicSearch {
		limits := strings.Split(args, ",")
		min, _ := strconv.Atoi(limits[0])
		max, _ := strconv.Atoi(limits[1])
		cfg.RequireTopics[discv5.Topic(topic)] = params.Limits{Min: min, Max: max}
	}
	cfg.ListenAddr = fmt.Sprintf("%s:30303", p.IP())
	log.Debug("Create statusd", "name", p.name, "command", strings.Join(cmd, " "))
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config file to json: %v", err)
	}
	f, err := ioutil.TempFile("", fmt.Sprintf(tmpSuffix, p.name))
	if err != nil {
		return fmt.Errorf("error creating temp file for container %s: %v", p.name, err)
	}
	defer f.Close()
	p.hostConfig = f.Name()
	if _, err := f.Write(bytes); err != nil {
		return fmt.Errorf("error writing config file to %s: %v", f.Name(), err)
	}
	log.Debug("Create statusd", "name", p.name, "command", strings.Join(cmd, " "), "config", f.Name())
	err = p.backend.Create(ctx, p.name, dockershim.CreateOpts{
		Cmd:   cmd,
		Image: p.config.Image,
		Ports: exposed,
		IPs: map[string]dockershim.IpOpts{p.config.NetID: dockershim.IpOpts{
			IP:    p.config.IP,
			NetID: p.config.NetID,
		}},
		HostConfigPath:      p.hostConfig,
		ContainerConfigPath: containerConfig,
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
	if len(p.hostConfig) > 0 {
		if err := os.Remove(p.hostConfig); err != nil {
			log.Error("error removing config file on host", "path", p.hostConfig, "error", err)
		}
	}
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
