package tlv

import (
	"bufio"
	"fmt"
	"sync"
)

// Request is a TLV request.
type Request struct {
	value  []byte
	header [5]byte
}

// Reset resets the given request.
func (req *Request) Reset() {
	req.value = req.value[:0]
}

// SetOpcode sets request opcode.
func (req *Request) SetOpcode(opcode byte) {
	req.header[4] = opcode
}

// Opcode returns request opcode.
//
// The returned value is valid until the next Request method call
// or until ReleaseRequest is called.
func (req *Request) Opcode() byte {
	return req.header[4]
}

// Write appends p to the request value.
//
// It implements io.Writer.
func (req *Request) Write(p []byte) (int, error) {
	req.Append(p)
	return len(p), nil
}

// Append appends p to the request value.
func (req *Request) Append(p []byte) {
	req.value = append(req.value, p...)
}

// SwapValue swaps the given value with the request's value.
//
// It is forbidden accessing the swapped value after the call.
func (req *Request) SwapValue(value []byte) []byte {
	v := req.value
	req.SetValue(value)
	return v
}

// SetValue sets the requests value to the given value.
func (req *Request) SetValue(value []byte) {
	req.value = append(req.value[:0], value...)
	req.value = req.value[:len(value)]
}

// Value returns request value.
//
// The returned value is valid until the next Request method call.
// or until ReleaseRequest is called.
func (req *Request) Value() []byte {
	return req.value
}

// WriteRequest writes the request to bw.
//
// It implements fastrpc.RequestWriter
func (req *Request) WriteRequest(bw *bufio.Writer) error {
	if err := writeBytes(bw, req.value, req.header[:]); err != nil {
		return fmt.Errorf("cannot write request value: %s", err)
	}
	return nil
}

// ReadRequest reads the request from br.
func (req *Request) ReadRequest(br *bufio.Reader) error {
	var err error
	req.value, err = readBytes(br, req.value[:0], req.header[:])
	if err != nil {
		return fmt.Errorf("cannot read request value: %s", err)
	}

	return nil
}

// AcquireRequest acquires new request.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		v = &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest releases the given request.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

var requestPool sync.Pool
