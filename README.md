How to run tests?
=================

Regular status-go image needs to be wrapped with a layer that has comcast (with all dependecies).
This is what Dockerfile for in this repository.

```bash
docker build . -t statusteam/statusd-debug:latest
```

Also we are using bootnode image in tests, but it is not wrapped at this point.

When you have docker installed and add comcast layer to status-go image you can run tests:

```
go test ./tests/ -v
```

There are no mandatory options in config, but you can explore them in `tests/config.go`.