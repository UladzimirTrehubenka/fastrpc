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
BenchmarkCoarseTimeNow          347397258                3.39 ns/op            0 B/op          0 allocs/op
BenchmarkTimeNow                22361860                51.9 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1         363763              2831 ns/op          92.19 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10        415294              2820 ns/op          92.55 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100       423319              2915 ns/op          89.54 MB/s           6 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000      388750              2809 ns/op          92.91 MB/s          17 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K       400630              2903 ns/op          89.90 MB/s          91 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms         463946              2352 ns/op         110.95 MB/s          13 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms         381465              2832 ns/op          92.16 MB/s          15 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms         196458              5626 ns/op          46.39 MB/s          31 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms         124017              9416 ns/op          27.72 MB/s          47 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms         59052             17158 ns/op          15.21 MB/s         100 B/op          0 allocs/op
BenchmarkEndToEnd                 476716              2739 ns/op          95.30 MB/s          12 B/op          0 allocs/op
BenchmarkEndToEndPipeline1        479089              2745 ns/op          95.10 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10       445239              2660 ns/op          98.12 MB/s           4 B/op          0 allocs/op
BenchmarkEndToEndPipeline100      486576              2533 ns/op         103.02 MB/s           5 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000     391005              2862 ns/op          91.19 MB/s          16 B/op          0 allocs/op
BenchmarkSendNowait              3698457               320 ns/op               0 B/op          0 allocs/op

GOMAXPROCS=4 go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/iwasaki-kenta/fastrpc
BenchmarkCoarseTimeNow-4                1000000000               0.868 ns/op           0 B/op          0 allocs/op
BenchmarkTimeNow-4                      77040884                13.8 ns/op             0 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1-4              1000000              1040 ns/op         250.99 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10-4             1213870              1061 ns/op         245.99 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndNoDelay100-4            1225502               986 ns/op         264.69 MB/s           2 B/op          0 allocs/op
BenchmarkEndToEndNoDelay1000-4           1126610              1096 ns/op         238.07 MB/s           8 B/op          0 allocs/op
BenchmarkEndToEndNoDelay10K-4            1012008              1123 ns/op         232.42 MB/s          41 B/op          0 allocs/op
BenchmarkEndToEndDelay1ms-4              1150018              1024 ns/op         254.77 MB/s           7 B/op          0 allocs/op
BenchmarkEndToEndDelay2ms-4              1088492              1163 ns/op         224.48 MB/s           7 B/op          0 allocs/op
BenchmarkEndToEndDelay4ms-4               745650              1477 ns/op         176.70 MB/s          10 B/op          0 allocs/op
BenchmarkEndToEndDelay8ms-4               343885              2948 ns/op          88.52 MB/s          23 B/op          0 allocs/op
BenchmarkEndToEndDelay16ms-4              226666              5369 ns/op          48.61 MB/s          35 B/op          0 allocs/op
BenchmarkEndToEnd-4                      1204228               974 ns/op         267.85 MB/s           7 B/op          0 allocs/op
BenchmarkEndToEndPipeline1-4             1469049               857 ns/op         304.64 MB/s           0 B/op          0 allocs/op
BenchmarkEndToEndPipeline10-4            1440090               871 ns/op         299.54 MB/s           1 B/op          0 allocs/op
BenchmarkEndToEndPipeline100-4           1341830               864 ns/op         302.15 MB/s           2 B/op          0 allocs/op
BenchmarkEndToEndPipeline1000-4          1397589               901 ns/op         289.65 MB/s           6 B/op          0 allocs/op
BenchmarkSendNowait-4                    6482055               178 ns/op               0 B/op          0 allocs/op
```