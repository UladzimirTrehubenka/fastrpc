[![GoDoc](https://godoc.org/github.com/iwasaki-kenta/fastrpc?status.svg)](http://godoc.org/github.com/iwasaki-kenta/fastrpc)
[![Go Report](https://goreportcard.com/badge/github.com/iwasaki-kenta/fastrpc)](https://goreportcard.com/report/github.com/iwasaki-kenta/fastrpc)


# fastrpc

Experimental fork of [valyala/fastrpc](https://github.com/valyala/fastrpc). Meant to be tested against high-latency p2p networks.

# Features

- Optimized for speed.
- Zero memory allocations in hot paths.
- Compression saves network bandwidth.

# How does it work?

It just sends batched rpc requests and responses over a single compressed
connection. This solves the following issues:

- High network bandwidth usage.
- High network packets rate.
- A lot of open TCP connections.

# Benchmark results

```
GOMAXPROCS=1 go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/iwasaki-kenta/fastrpc
BenchmarkCoarseTimeNow                  399328861                3.01 ns/op            0 B/op          0 allocs/op
BenchmarkTimeNow                        26539466                45.6 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1                 492326              2436 ns/op         107.15 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10                488337              2522 ns/op         103.47 MB/s           4 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100               441424              2474 ns/op         105.50 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000              446696              2477 ns/op         105.35 MB/s          15 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K               460237              2986 ns/op          87.42 MB/s          83 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms                 499604              2072 ns/op         125.96 MB/s          12 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms                 399772              2659 ns/op          98.17 MB/s          14 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms                 223335              6441 ns/op          40.52 MB/s          26 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms                 123615              9435 ns/op          27.66 MB/s          48 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms                 59798             17078 ns/op          15.28 MB/s          98 B/op          0 allocs/op
BenchmarkEndToEndCompressNone             501861              2362 ns/op         110.49 MB/s          12 B/op          0 allocs/op
BenchmarkEndToEndCompressFlate            159049              6651 ns/op          39.24 MB/s         124 B/op          0 allocs/op
BenchmarkEndToEndCompressSnappy           533713              2435 ns/op         107.17 MB/s          18 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressNone          558248              2257 ns/op         115.63 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressFlate         179523              5635 ns/op          46.32 MB/s         110 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressSnappy        556590              2529 ns/op         103.21 MB/s          17 B/op          0 allocs/op
BenchmarkEndToEndPipeline1                528733              2246 ns/op         116.20 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10               527016              2380 ns/op         109.65 MB/s           4 B/op          0 allocs/op
BenchmarkEndToEndPipeline100              478672              2255 ns/op         115.76 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000             492486              2351 ns/op         111.02 MB/s          13 B/op          0 allocs/op
BenchmarkSendNowait                      4239966               279 ns/op               0 B/op          0 allocs/op

GOMAXPROCS=4 go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/iwasaki-kenta/fastrpc
BenchmarkCoarseTimeNow-4                1000000000               4.28 ns/op            0 B/op          0 allocs/op
BenchmarkTimeNow-4                      81932486                12.5 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1-4              1438008               804 ns/op         324.72 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10-4             1439896               813 ns/op         321.20 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100-4            1387754               906 ns/op         288.00 MB/s           2 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000-4           1412473               813 ns/op         321.21 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K-4            1271354               895 ns/op         291.56 MB/s          33 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms-4              1328835              1015 ns/op         257.09 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms-4              1139629              1227 ns/op         212.71 MB/s           7 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms-4               788870              1605 ns/op         162.63 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms-4               430866              3110 ns/op          83.92 MB/s          20 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms-4              220934              5454 ns/op          47.86 MB/s          35 B/op          0 allocs/op
BenchmarkEndToEndCompressNone-4          1106846              1048 ns/op         248.96 MB/s           8 B/op          0 allocs/op
BenchmarkEndToEndCompressFlate-4          604020              2070 ns/op         126.08 MB/s          39 B/op          0 allocs/op
BenchmarkEndToEndCompressSnappy-4        1284912               943 ns/op         276.74 MB/s           9 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressNone-4       1446034               909 ns/op         287.14 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressFlate-4       610010              2035 ns/op         128.26 MB/s          39 B/op          0 allocs/op
BenchmarkEndToEndTLSCompressSnappy-4     1284763              1020 ns/op         255.94 MB/s           9 B/op          0 allocs/op
BenchmarkEndToEndPipeline1-4             1487145               791 ns/op         330.05 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10-4            1553846               756 ns/op         345.40 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndPipeline100-4           1576508               752 ns/op         346.90 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000-4          1548182               740 ns/op         352.93 MB/s           5 B/op          0 allocs/op
BenchmarkSendNowait-4                    7657404               149 ns/op               0 B/op          0 allocs/op
```