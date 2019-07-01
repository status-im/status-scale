package cluster

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/status-im/status-go/params"
	"github.com/status-im/status-scale/dockershim"
	"github.com/status-im/status-scale/network"
)

func DefaultConfig() PeerConfig {
	return PeerConfig{
		Whisper:   true,
		Metrics:   true,
		HTTP:      true,
		Host:      "0.0.0.0",
		Modules:   []string{"admin", "debug", "shh", "net", "ssm"},
		Port:      8545,
		NetworkID: 100,
		Discovery: true,
	}
}

const (
	tmpSuffix       = "scale-peer-%s-"
	containerConfig = "/conf.json"

	DefaultMailServerPassword = "status-offline-inbox"
)

func NewStatusd(config PeerConfig, backend Backend) *Peer {
	return NewPeer(config, backend, []string{"statusd", "-c", containerConfig})
}

func NewClient(config PeerConfig, backend Backend, identity *ecdsa.PrivateKey) *Client {
	return &Client{Peer: NewPeer(config, backend,
		[]string{"status-term-client", "-no-ui", "-node-config", containerConfig, "-keyhex", hex.EncodeToString(crypto.FromECDSA(identity)), "-log-level", "TRACE"}),
		Identity: identity,
	}
}

func NewMVDS(config PeerConfig, backend Backend, identity *ecdsa.PrivateKey) *Client {
	return &Client{Peer: NewPeer(config, backend,
		[]string{"status-term-client", "-mvds", "-no-ui", "-node-config", containerConfig, "-keyhex", hex.EncodeToString(crypto.FromECDSA(identity))}),
		Identity: identity,
	}
}

type Client struct {
	*Peer
	Identity *ecdsa.PrivateKey
}

func NewPeer(config PeerConfig, backend Backend, cmd []string) *Peer {
	return &Peer{baseCmd: cmd, name: config.Name, config: config, backend: backend}
}

type PeerConfig struct {
	Name  string
	NetID string
	IP    string
	Image string

	Mailserver      bool
	Modules         []string
	Whisper         bool
	BootNodes       []string
	RendezvousNodes []string
	Mailservers     []string
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
	// copy before changing
	baseCmd []string
	name    string
	config  PeerConfig

	backend Backend

	client *rpc.Client
	enode  string

	hostConfig string
}

func (p *Peer) String() string {
	return fmt.Sprintf("peer %s %s", p.name, p.config.IP)
}

func (p *Peer) Enode() string {
	return p.enode
}

func (p *Peer) Create(ctx context.Context) error {
	cmd := make([]string, len(p.baseCmd))
	copy(cmd, p.baseCmd)
	cfg, err := params.NewNodeConfig("/status-data", 7777)
	if err != nil {
		return err
	}
	cfg.LogEnabled = true
	cfg.LogToStderr = true
	cfg.LogLevel = "INFO"
	var exposed []string
	if p.config.Whisper {
		cfg.WhisperConfig.Enabled = true
		if p.config.Mailserver {
			cfg.WhisperConfig.EnableMailServer = true
			cfg.WhisperConfig.DataDir = filepath.Join(cfg.DataDir, "mail")
			cfg.WhisperConfig.MailServerPassword = DefaultMailServerPassword
		}
	}
	if p.config.HTTP {
		cfg.HTTPEnabled = true
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
	cfg.ClusterConfig.Enabled = true
	cfg.NoDiscovery = true
	if len(p.config.BootNodes) != 0 {
		cfg.NoDiscovery = false
		cfg.ClusterConfig.BootNodes = p.config.BootNodes
	}
	if len(p.config.RendezvousNodes) != 0 {
		cfg.Rendezvous = true
		cfg.ClusterConfig.RendezvousNodes = p.config.RendezvousNodes
	}
	cfg.ClusterConfig.TrustedMailServers = p.config.Mailservers
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

func (p *Peer) Remove(ctx context.Context) error {
	log.Debug("removing statusd", "name", p.name)
	if len(p.hostConfig) > 0 {
		if err := os.Remove(p.hostConfig); err != nil {
			log.Error("error removing config file on host", "path", p.hostConfig, "error", err)
		}
	}
	return p.backend.Remove(ctx, p.name)
}

func (p *Peer) EnableConditions(ctx context.Context, opt network.Options) error {
	return network.ComcastStart(func(ctx context.Context, cmd []string) error {
		log.Debug("run command", "peer", p.name, "command", strings.Join(cmd, " "))
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opt)
}

func (p *Peer) DisableConditions(ctx context.Context, opt network.Options) error {
	err := network.ComcastStop(func(ctx context.Context, cmd []string) error {
		log.Debug("run command", "peer", p.name, "command", strings.Join(cmd, " "))
		return p.backend.Execute(ctx, p.name, cmd)
	}, ctx, opt)
	if err != nil {
		return fmt.Errorf("failed to stop comcast on a peer %s: %v", p.name, err)
	}
	return nil
}

func (p *Peer) IP() string {
	return p.config.IP
}

func (p Peer) makeRPCClient(ctx context.Context) (*rpc.Client, error) {
	ports, err := p.backend.ConnectionInfo(ctx, p.name, p.config.Port)
	if err != nil {
		return nil, err
	}
	for _, port := range ports {
		log.Debug("host port for container", "name", p.name, "port", port.HostPort)
	}

	if len(ports) < 1 {
		return nil, fmt.Errorf("peer %s doesn't have any bindings", p.name)
	}
	// can use any
	rawurl := fmt.Sprintf("http://%s:%s", ports[0].HostIP, ports[0].HostPort)
	log.Debug("init rpc client", "name", p.name, "url", rawurl)
	return rpc.DialContext(ctx, rawurl)
}

func (p Peer) UID() string {
	return p.name
}

func (p Peer) Rpc() *rpc.Client {
	return p.client
}

func (p Peer) RawMetrics(ctx context.Context) ([]byte, error) {
	rst := json.RawMessage{}
	err := p.client.CallContext(ctx, &rst, "debug_metrics", true)
	log.Trace("fetched metrics", "peer", p.name, "metrics", string(rst))
	return []byte(rst), err
}

func (p *Peer) healthcheck(ctx context.Context, retries int, interval time.Duration) error {
	log.Debug("running healthcheck", "peer", p.name)
	info := p2p.NodeInfo{}
	for i := retries; i > 0; i-- {
		err := p.client.CallContext(ctx, &info, "admin_nodeInfo")
		log.Trace("healtcheck", "peer", p.name, "tries", i, "error", err)
		if err != nil {
			time.Sleep(interval)
			continue
		}
		log.Debug("received response from node info", "info", info)
		// note(dshulyak) node can't discover its external ip and uses 127.0.0.1.
		// we have to use preconfigured listen addr
		node, err := enode.ParseV4(info.Enode)
		if err != nil {
			return err
		}
		parts := strings.Split(info.ListenAddr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("failed to split %s into two parts uins `:`", info.ListenAddr)
		}
		p.enode = enode.NewV4(node.Pubkey(), net.ParseIP(parts[0]), info.Ports.Listener, info.Ports.Discovery).String()
		log.Debug("received enode for", "name", p.name, "enode", p.enode)
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
