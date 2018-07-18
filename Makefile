images:
	docker build -f Dockerfile -t statusteam/statusd-debug:latest .
	docker build -f Dockerfile-boot -t statusteam/bootnode-debug:latest .
	docker build -f Dockerfile-rendezvous -t statusteam/rendezvous-debug:latest .
