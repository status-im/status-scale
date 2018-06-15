How to run tests?
=================

Regular status-go image needs to be wrapped with a layer that has comcast (with all dependecies).
This is what Dockerfile for in this repository.

```bash
$ docker build -f Dockerfile -t statusteam/statusd-debug:latest .
$ docker build -f Dockerfile-boot -t statusteam/bootnode-debug:latest .
```

Also we are using bootnode image in tests, but it is not wrapped at this point.

Install go dependencies:
```bash
$ curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
$ dep ensure
```

When you have docker installed and add comcast layer to status-go image you can run tests:

```bash
$ go test ./tests/ -v
```

Alternatively you can use vagrant to have an environment set up

```
$ vagrant up
$ vagrant ssh
vagrant> cd /home/vagrant/go/src/github.com/status-im/status-scale
vagrant> docker build -f Dockerfile -t statusteam/statusd-debug:latest .
vagrant> docker build -f Dockerfile-boot -t statusteam/bootnode-debug:latest .
vagrant> $ go test ./tests/ -v
```

There are no mandatory options in config, but you can explore them in `tests/config.go`.
