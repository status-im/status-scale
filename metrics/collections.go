package metrics

func P2PColumns() []interface{} {
	return []interface{}{
		RawColumn{[]string{"result", "p2p", "InboundTraffic", "Overall"}, "p2p/inbound"},
		RawColumn{[]string{"result", "p2p", "OutboundTraffic", "Overall"}, "p2p/outbound"},
		ComputeColumn{"p2p/total", func(r Row) (interface{}, error) {
			return r["p2p/inbound"].(int64) + r["p2p/outbound"].(int64), nil
		}},
		RawColumn{[]string{"result", "p2p", "Peers", "Overall"}, "p2p/peers"},
	}
}
