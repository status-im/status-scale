package cluster

type Cluster struct {
	Bootnodes int
	Relay     int
	Users     int

	bootnodes []Bootnode
	relays    []Peer
	users     []Peer
}
