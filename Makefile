images:
	docker build -f Dockerfile -t statusteam/statusd-debug:latest .
	docker build -f Dockerfile -t statusteam/bootnode-debug:latest .
	docker build -f Dockerfile -t statusteam/rendezvous-debug:latest .
