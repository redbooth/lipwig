// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package ssmp

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

var errArbitrary error = fmt.Errorf("arbitrary")

type testReader struct {
	reads []string
	i, j  int
	err   error
}

func newReader(err error, reads ...string) *Decoder {
	return NewDecoder(&testReader{
		reads: reads,
		err:   err,
	})
}

func (r *testReader) Read(p []byte) (int, error) {
	if r.i == len(r.reads) {
		return 0, r.err
	}
	n := copy(p, r.reads[r.i][r.j:])
	r.j += n
	if r.j == len(r.reads[r.i]) {
		r.j = 0
		r.i++
	}
	return n, nil
}

func u(r ...interface{}) []interface{} {
	return r
}

func expectError(t *testing.T, expected error, h []interface{}) {
	assert.Equal(t, expected, h[1])
}

func expectInt(t *testing.T, expected int, h []interface{}) {
	assert.Nil(t, h[1])
	assert.Equal(t, expected, h[0])
}

func expectData(t *testing.T, expected string, h []interface{}) {
	assert.Nil(t, h[1])
	assert.Equal(t, []byte(expected), h[0])
}

func TestDecoder_should_return_err_empty(t *testing.T) {
	r := newReader(io.EOF)
	expectError(t, io.EOF, u(r.DecodeVerb()))
	expectError(t, io.EOF, u(r.DecodeCode()))
	expectError(t, io.EOF, u(r.DecodeId()))
	expectError(t, io.EOF, u(r.DecodePayload()))
	expectError(t, io.EOF, u(r.DecodeCompat()))
}

func TestDecoder_should_return_err_incomplete(t *testing.T) {
	r := newReader(errArbitrary, "VERB")
	expectError(t, errArbitrary, u(r.DecodeVerb()))
	expectError(t, errArbitrary, u(r.DecodeId()))
	expectError(t, errArbitrary, u(r.DecodePayload()))
	expectError(t, errArbitrary, u(r.DecodeCompat()))
}

func TestDecoder_should_reject_leading_space(t *testing.T) {
	r := newReader(io.EOF, " ")
	expectError(t, ErrInvalidMessage, u(r.DecodeVerb()))
	expectError(t, ErrInvalidMessage, u(r.DecodeCode()))
	expectError(t, ErrInvalidMessage, u(r.DecodeId()))
}

func TestDecoder_should_reject_eol(t *testing.T) {
	r := newReader(io.EOF, "\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeVerb()))
	expectError(t, ErrInvalidMessage, u(r.DecodeCode()))
	expectError(t, ErrInvalidMessage, u(r.DecodeId()))
	expectError(t, ErrInvalidMessage, u(r.DecodePayload()))
	expectError(t, ErrInvalidMessage, u(r.DecodeCompat()))
}

func TestDecoder_should_decode_verb(t *testing.T) {
	r := newReader(io.EOF, "VERB ")
	expectData(t, "VERB", u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_verb_longest(t *testing.T) {
	r := newReader(io.EOF, "ABCDEFGHIJKLMNOP ")
	expectData(t, "ABCDEFGHIJKLMNOP", u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_verb_atend(t *testing.T) {
	r := newReader(io.EOF, "VERB\n")
	expectData(t, "VERB", u(r.DecodeVerb()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_verb_split(t *testing.T) {
	r := newReader(io.EOF, "VE", "RB", " ")
	expectData(t, "VERB", u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_verb_lower(t *testing.T) {
	r := newReader(io.EOF, "Verb\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_verb_num(t *testing.T) {
	r := newReader(io.EOF, "VERB123\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_verb_length(t *testing.T) {
	r := newReader(io.EOF, "ABCDEFGHIJKLMNOPQ\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeVerb()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_code(t *testing.T) {
	r := newReader(io.EOF, "123 ")
	expectInt(t, 123, u(r.DecodeCode()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_code_atend(t *testing.T) {
	r := newReader(io.EOF, "123\n")
	expectInt(t, 123, u(r.DecodeCode()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_code_split(t *testing.T) {
	r := newReader(io.EOF, "1", "2", "3 ")
	expectInt(t, 123, u(r.DecodeCode()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_code_alpha(t *testing.T) {
	r := newReader(io.EOF, "1F2\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeCode()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_code_short(t *testing.T) {
	r := newReader(io.EOF, "12\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeCode()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_code_long(t *testing.T) {
	r := newReader(io.EOF, "1234\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeCode()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_id(t *testing.T) {
	r := newReader(io.EOF, "UPPER.lower@123:/_-+=~ ....")
	expectData(t, "UPPER.lower@123:/_-+=~", u(r.DecodeId()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_id_longest(t *testing.T) {
	r := newReader(io.EOF, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/ ....")
	expectData(t, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/", u(r.DecodeId()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_id_atend(t *testing.T) {
	r := newReader(io.EOF, "UPPER.lower@123:/_-+=~\n")
	expectData(t, "UPPER.lower@123:/_-+=~", u(r.DecodeId()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_id_split(t *testing.T) {
	r := newReader(io.EOF, "UPPER", ".lower", "@123:/_-+=~ ....")
	expectData(t, "UPPER.lower@123:/_-+=~", u(r.DecodeId()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_id_length(t *testing.T) {
	r := newReader(io.EOF, "abcdefghijklmnopqrstuvwxyz@ABCDEFGHIJKLMNOPQRSTUVWXYZ.0123456789/\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeId()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_reject_id_charset(t *testing.T) {
	r := newReader(io.EOF, "test$\n")
	expectError(t, ErrInvalidMessage, u(r.DecodeId()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_text_payload(t *testing.T) {
	r := newReader(io.EOF, "test 123 \t$#%<>[]{}\n\n")
	expectData(t, "test 123 \t$#%<>[]{}", u(r.DecodePayload()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_text_payload_split(t *testing.T) {
	r := newReader(io.EOF, "test ", "123 \t$#", "%<>[]{}\n\n")
	expectData(t, "test 123 \t$#%<>[]{}", u(r.DecodePayload()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_text_payload_0_3(t *testing.T) {
	for i := 0; i <= 3; i++ {
		r := newReader(io.EOF, "hello "+string([]byte{byte(i)})+"\n")
		expectData(t, "hello "+string([]byte{byte(i)}), u(r.DecodePayload()))
		assert.True(t, r.AtEnd())
	}
}

func TestDecoder_should_reject_text_length(t *testing.T) {
	long := "0123456789ABCDEF"
	for i := 0; i < 6; i++ {
		long = long + long
	}
	r := newReader(io.EOF, long+".\n")
	expectError(t, ErrInvalidMessage, u(r.DecodePayload()))
	assert.False(t, r.AtEnd())
}

func TestDecoder_should_decode_binary_payload(t *testing.T) {
	var d [259]byte
	d[0] = 0
	d[1] = 0xff
	for i := 0; i < 256; i++ {
		d[i+2] = byte(i)
	}
	d[258] = '\n'
	r := newReader(io.EOF, string(d[:]))
	expectData(t, string(d[2:258]), u(r.DecodePayload()))
	assert.True(t, r.AtEnd())
}

func TestDecoder_should_decode_binary_payload_split(t *testing.T) {
	var d [259]byte
	d[0] = 0
	d[1] = 0xff
	for i := 0; i < 256; i++ {
		d[i+2] = byte(i)
	}
	d[258] = '\n'
	r := newReader(io.EOF, string(d[0:1]), string(d[1:15]), string(d[15:]))
	expectData(t, string(d[2:258]), u(r.DecodePayload()))
	assert.True(t, r.AtEnd())
}
