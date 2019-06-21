images:
	docker build -f Dockerfile -t statusteam/statusd-debug:latest .
	docker build -f Dockerfile-boot -t statusteam/bootnode-debug:latest .
	docker build -f Dockerfile-rendezvous -t statusteam/rendezvous-debug:latest .
	docker build -f Dockerfile-client -t statusteam/client-debug:latest .
.PHONY: images

install-dev:
	GO111MODULE=off go get -u github.com/goware/modvendor
.PHONY: install-dev

vendor:
	go mod vendor
	modvendor -copy="**/*.c **/*.h" -v
.PHONY: vendor
