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
BenchmarkCoarseTimeNow                  364963537                3.31 ns/op            0 B/op          0 allocs/op
BenchmarkTimeNow                        25656691                45.5 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1                 498571              2455 ns/op         106.31 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10                486901              2492 ns/op         104.73 MB/s           4 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100               436976              2536 ns/op         102.93 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000              445292              2487 ns/op         104.93 MB/s          15 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K               466159              2616 ns/op          99.78 MB/s          82 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms                 559761              2169 ns/op         120.31 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms                 423051              2698 ns/op          96.73 MB/s          14 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms                 226353              6846 ns/op          38.12 MB/s          26 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms                 125736              9221 ns/op          28.31 MB/s          48 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms                 59787             17146 ns/op          15.22 MB/s         100 B/op          0 allocs/op
BenchmarkEndToEndCompressNone             580756              2100 ns/op         124.27 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndCompressFlate            157530              6533 ns/op          39.95 MB/s         125 B/op          0 allocs/op
BenchmarkEndToEndCompressSnappy           437868              2357 ns/op         110.71 MB/s          22 B/op          0 allocs/op
BenchmarkEndToEndPipeline1                544328              2232 ns/op         116.94 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10               491532              2253 ns/op         115.84 MB/s           4 B/op          0 allocs/op
BenchmarkEndToEndPipeline100              493140              2287 ns/op         114.11 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000             471315              2411 ns/op         108.25 MB/s          14 B/op          0 allocs/op
BenchmarkSendNowait                      4061835               290 ns/op               0 B/op          0 allocs/op

GOMAXPROCS=4 go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/iwasaki-kenta/fastrpc
BenchmarkCoarseTimeNow-4                1000000000               0.897 ns/op           0 B/op          0 allocs/op
BenchmarkTimeNow-4                      86661357                13.0 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1-4              1555634               793 ns/op         329.11 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10-4             1473580               835 ns/op         312.43 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100-4            1467427               827 ns/op         315.72 MB/s           2 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000-4           1409013               849 ns/op         307.30 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K-4            1294513               842 ns/op         309.89 MB/s          33 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms-4              1489090              1077 ns/op         242.24 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms-4              1033914              1592 ns/op         163.99 MB/s           8 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms-4               817815              1567 ns/op         166.59 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms-4               343420              3013 ns/op          86.62 MB/s          24 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms-4              229160              5570 ns/op          46.85 MB/s          35 B/op          0 allocs/op
BenchmarkEndToEndCompressNone-4          1326176               966 ns/op         270.20 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndCompressFlate-4          466648              2148 ns/op         121.48 MB/s          50 B/op          0 allocs/op
BenchmarkEndToEndCompressSnappy-4        1272913               945 ns/op         276.20 MB/s           9 B/op          0 allocs/op
BenchmarkEndToEndPipeline1-4             1606730               783 ns/op         333.34 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10-4            1545934               773 ns/op         337.64 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndPipeline100-4           1607368               767 ns/op         340.30 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000-4          1644616               746 ns/op         349.82 MB/s           5 B/op          0 allocs/op
BenchmarkSendNowait-4                    7888968               151 ns/op               0 B/op          0 allocs/op

```