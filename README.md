Scale tests
===========

The goal of these tests is to collect and verify metrics obtained from status-go application
under load.

To run these tests you need:
- install docker-compose
- build status-go container with metrics and prometheus tags
```bash
make docker-image BUILD_TAGS="metrics prometheus
```

Tests can be run with:

```
go test -v -timeout=20m ./ -wnode-scale=20
```

Wnode-scale is optional and 12 will be used by default. Timeout is also
optional but you need to be aware of it if you are extending or changing parameters of tests.
Most of the tests should print summary table after they are finished, if you are not interested
in it - remove verbosity flag.

Example of summary:

|HEADERS	|ingress	|egress		|dups	|new	|dups/new|
|-		|-		|-		|-	|-	|-|
|0		|0.123857 mb	|0.149817 mb	|100	|40	|2.500000|
|1		|0.188950 mb	|0.146632 mb	|174	|41	|4.243902|
|2		|0.150337 mb	|0.212955 mb	|129	|40	|3.225000|
|3		|0.135966 mb	|0.207945 mb	|111	|40	|2.775000|
|4		|0.221158 mb	|0.231804 mb	|212	|40	|5.300000|
|5		|0.224313 mb	|0.197873 mb	|205	|50	|4.100000|
|6		|0.234121 mb	|0.119251 mb	|217	|51	|4.254902|
|7		|0.206933 mb	|0.203265 mb	|186	|50	|3.720000|
|8		|0.172408 mb	|0.192818 mb	|148	|50	|2.960000|
|9		|0.191420 mb	|0.193930 mb	|168	|50	|3.360000|
|10		|0.211752 mb	|0.206160 mb	|191	|50	|3.820000|
|11		|0.193108 mb	|0.197458 mb	|173	|50	|3.460000|
|TOTAL		|2.254323 mb	|2.259908 mb	|2014	|552	|3.643234|