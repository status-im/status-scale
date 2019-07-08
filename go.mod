module github.com/status-im/status-scale

go 1.12

require (
	docker.io/go-docker v1.0.0
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/btcsuite/btcd v0.0.0-20190614013741-962a206e94e9
	github.com/buger/jsonparser v0.0.0-20180318095312-2cac668e8456
	github.com/docker/go-connections v0.4.0
	github.com/ethereum/go-ethereum v1.8.27
	github.com/gxed/hashland v0.0.0-20180221191214-d9f6b97f8db2 // indirect
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-peer v0.2.0
	github.com/montanaflynn/stats v0.5.0
	github.com/olekukonko/tablewriter v0.0.0-20180506121414-d4647c9c7a84
	github.com/prometheus/prometheus v1.7.1-0.20170814170113-3101606756c5 // indirect
	github.com/status-im/status-console-client v0.0.0-20190701050511-1a3e62a7564f
	github.com/status-im/status-go v0.26.0-beta.0
	github.com/stretchr/testify v1.3.0
)

replace github.com/ethereum/go-ethereum v1.8.27 => github.com/status-im/go-ethereum v1.8.27-status
