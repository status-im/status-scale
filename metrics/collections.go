package metrics

func P2PColumns() []interface{} {
	return []interface{}{
		RawColumn{[]string{"p2p", "InboundTraffic", "Overall"}, "p2p/inbound"},
		RawColumn{[]string{"p2p", "OutboundTraffic", "Overall"}, "p2p/outbound"},
		ComputeColumn{"p2p/total", func(r Row) (interface{}, error) {
			return r["p2p/inbound"].(int64) + r["p2p/outbound"].(int64), nil
		}},
		RawColumn{[]string{"p2p", "Peers", "Overall"}, "p2p/peers"},
	}
}

func DiscoveryColumns() []interface{} {
	return []interface{}{
		RawColumn{[]string{"discv5", "InboundTraffic", "Overall"}, "discovery/inbound"},
		RawColumn{[]string{"discv5", "OutboundTraffic", "Overall"}, "discovery/outbound"},
	}
}

func RendezvousColumns() []interface{} {
	return []interface{}{
		RawColumn{[]string{"rendezvous", "InboundTraffic", "Overall"}, "rendezvous/inbound"},
		RawColumn{[]string{"rendezvous", "OutboundTraffic", "Overall"}, "rendezvous/outbound"},
	}
}

func OnlyPeers() []interface{} {
	return []interface{}{
		RawColumn{[]string{"p2p", "Peers", "Overall"}, "p2p/peers"},
	}
}
