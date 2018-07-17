package cluster

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	lcrypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"

	"github.com/status-im/status-scale/dockershim"
)

type Rendezvous Bootnode

func (r Rendezvous) String() string {
	return fmt.Sprintf("rendezvous %s: %s", r.name, r.Addr())
}

func (r Rendezvous) Create(ctx context.Context) error {
	data := hex.EncodeToString(crypto.FromECDSA(r.key))
	cmd := []string{"-a=" + r.ip, "-p=" + strconv.Itoa(r.port), "-keyhex=" + data}
	log.Debug("creating rendezvous", "name", r.name, "address", r.String(), "cmd", strings.Join(cmd, " "))
	return r.backend.Create(ctx, r.name, dockershim.CreateOpts{
		Entrypoint: "rendezvous",
		Cmd:        cmd,
		Image:      r.image,
		IPs: map[string]dockershim.IpOpts{r.network: dockershim.IpOpts{
			IP:    r.ip,
			NetID: r.network,
		}},
	},
	)
}

func (r Rendezvous) Addr() string {
	key := lcrypto.Secp256k1PublicKey(btcec.PublicKey(r.key.PublicKey))
	id, err := peer.IDFromPublicKey(lcrypto.PubKey(&key))
	if err != nil {
		log.Error("unable to convert public key to pid", "error", err)
		return ""
	}
	return fmt.Sprintf("/ipv4/%s/tcp/%d/ethv4/%s", r.ip, r.port, id.Pretty())
}
