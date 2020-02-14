package tlv

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
)

func TestRequestMarshalUnmarshal(t *testing.T) {
	var buf bytes.Buffer

	req := AcquireRequest()
	bw := bufio.NewWriter(&buf)
	for i := 0; i < 50; i++ {
		value := fmt.Sprintf("value %d", i)
		req.SetOpcode(byte(i))
		req.SwapValue([]byte(value))
		if err := req.WriteRequest(bw); err != nil {
			t.Fatalf("unexpected error when writing request: %s", err)
		}
	}
	if err := bw.Flush(); err != nil {
		t.Fatalf("unexpected error when flushing request: %s", err)
	}
	ReleaseRequest(req)

	req1 := AcquireRequest()
	br := bufio.NewReader(&buf)
	for i := 0; i < 50; i++ {
		value := fmt.Sprintf("value %d", i)
		if err := req1.ReadRequest(br); err != nil {
			t.Fatalf("unexpected error when reading request: %s", err)
		}
		if req1.Opcode() != byte(i) {
			fmt.Println(req1.header)
			t.Fatalf("unexpected request opcode read: %d. Expecting %d", req1.Opcode(), i)
		}
		if string(req1.Value()) != value {
			t.Fatalf("unexpected request value read: %q. Expecting %q", req1.Value(), value)
		}
	}
	ReleaseRequest(req1)
}
