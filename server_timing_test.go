package fastrpc

import (
	"bytes"
	"github.com/iwasaki-kenta/fastrpc/tlv"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkEndToEndNoDelay1(b *testing.B) {
	benchmarkEndToEnd(b, 1, 0, false)
}

func BenchmarkEndToEndNoDelay10(b *testing.B) {
	benchmarkEndToEnd(b, 10, 0, false)
}

func BenchmarkEndToEndNoDelay100(b *testing.B) {
	benchmarkEndToEnd(b, 100, 0, false)
}

func BenchmarkEndToEndNoDelay1000(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 0, false)
}

func BenchmarkEndToEndNoDelay10K(b *testing.B) {
	benchmarkEndToEnd(b, 10000, 0, false)
}

func BenchmarkEndToEndDelay1ms(b *testing.B) {
	benchmarkEndToEnd(b, 1000, time.Millisecond, false)
}

func BenchmarkEndToEndDelay2ms(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 2*time.Millisecond, false)
}

func BenchmarkEndToEndDelay4ms(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 4*time.Millisecond, false)
}

func BenchmarkEndToEndDelay8ms(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 8*time.Millisecond, false)
}

func BenchmarkEndToEndDelay16ms(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 16*time.Millisecond, false)
}

func BenchmarkEndToEnd(b *testing.B) {
	benchmarkEndToEnd(b, 1000, time.Millisecond, false)
}

func BenchmarkEndToEndPipeline1(b *testing.B) {
	benchmarkEndToEnd(b, 1, 0, true)
}

func BenchmarkEndToEndPipeline10(b *testing.B) {
	benchmarkEndToEnd(b, 10, 0, true)
}

func BenchmarkEndToEndPipeline100(b *testing.B) {
	benchmarkEndToEnd(b, 100, 0, true)
}

func BenchmarkEndToEndPipeline1000(b *testing.B) {
	benchmarkEndToEnd(b, 1000, 0, true)
}

func BenchmarkSendNowait(b *testing.B) {
	bN := uint64(b.N)
	var n uint64
	doneCh := make(chan struct{})
	s := &Server{
		NewHandlerCtx: newTestHandlerCtx,
		Handler: func(ctxv HandlerCtx) HandlerCtx {
			x := atomic.AddUint64(&n, 1)
			if x == bN {
				close(doneCh)
			}
			return ctxv
		},
		Concurrency:      runtime.GOMAXPROCS(-1) + 1,
		PipelineRequests: true,
	}
	serverStop, ln := newTestServerExt(s)

	value := []byte("foobar")
	b.RunParallel(func(pb *testing.PB) {
		c := newTestClient(ln)
		c.MaxPendingRequests = 1e2
		for pb.Next() {
			for {
				req := acquireTestRequest()
				req.Append(value)
				if c.SendNowait(req, releaseTestRequest) {
					break
				}
				runtime.Gosched()
			}
		}
	})
	runtime.Gosched()

	// Add skipped requests.
	// Requests may be skipped by cleaners.
	c := newTestClient(ln)
	for {
		x := atomic.LoadUint64(&n)
		if x >= bN {
			break
		}
		req := acquireTestRequest()
		req.Append(value)
		c.SendNowait(req, releaseTestRequest)
		runtime.Gosched()
	}

	select {
	case <-doneCh:
	case <-time.After(10 * time.Second):
		b.Fatalf("timeout. n=%d, b.N=%d", n, b.N)
	}

	if err := serverStop(); err != nil {
		b.Fatalf("cannot shutdown server: %s", err)
	}
}

func benchmarkEndToEnd(b *testing.B, parallelism int, batchDelay time.Duration, pipelineRequests bool) {
	var serverBatchDelay time.Duration
	if batchDelay > 0 {
		serverBatchDelay = 100 * time.Microsecond
	}
	expectedBody := []byte("Hello world foobar baz aaa bbb ccc ddd eee gklj kljsdfsdf" +
		"sdfasdaf asdf asdf dsa fasd fdasf afsgfdsg ertytrshdsf fds gf" +
		"dfagsf asglsdkflaskdflkqowqiot asdkljlp 0293 4u09u0sd9fulksj lksfj lksdfj sdf" +
		"sfjkko9u iodjsf-[9j lksdjf;lkasdj02r fsd fhjas;klfj asd;lfjwjfsd; ")
	s := &Server{
		NewHandlerCtx: newTestHandlerCtx,
		Handler: func(ctxv HandlerCtx) HandlerCtx {
			ctx := ctxv.(*tlv.RequestCtx)
			ctx.Response.Append(expectedBody)
			return ctx
		},
		Concurrency:      parallelism * runtime.NumCPU(),
		MaxBatchDelay:    serverBatchDelay,
		PipelineRequests: pipelineRequests,
	}
	serverStop, ln := newTestServerExt(s)

	var cc []*Client
	for i := 0; i < runtime.NumCPU(); i++ {
		c := newTestClient(ln)
		c.MaxPendingRequests = s.Concurrency
		c.MaxBatchDelay = batchDelay
		cc = append(cc, c)
	}
	var clientIdx uint32

	deadline := time.Now().Add(time.Hour)
	b.SetParallelism(parallelism)
	b.SetBytes(int64(len(expectedBody)))
	b.RunParallel(func(pb *testing.PB) {
		n := atomic.AddUint32(&clientIdx, 1)
		c := cc[int(n)%len(cc)]
		var req tlv.Request
		var resp tlv.Response
		req.SwapValue([]byte("foobar"))
		for pb.Next() {
			if err := c.DoDeadline(&req, &resp, deadline); err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
			if !bytes.Equal(resp.Value(), expectedBody) {
				b.Fatalf("unexpected body: %q. Expecting %q", resp.Value(), expectedBody)
			}
		}
	})

	if err := serverStop(); err != nil {
		b.Fatalf("cannot shutdown server: %s", err)
	}
}
