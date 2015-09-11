// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package ssmp

import (
	"fmt"
	"io"
)

type Decoder struct {
	rd      io.Reader
	buf     []byte
	s, r, w int
	lastErr error
}

var ErrInvalidMessage error = fmt.Errorf("invalid message")

func NewDecoder(rd io.Reader) *Decoder {
	return &Decoder{
		rd:  rd,
		buf: make([]byte, bufferSize),
	}
}

const (
	CodeLength          = 3
	MaxVerbLength       = 16
	MaxIdentifierLength = 64
	MaxPayloadLength    = 1024
	BinaryPayloadPrefix = 2

	MaxMessageLength = CodeLength + 5 + MaxVerbLength + 2*MaxIdentifierLength + BinaryPayloadPrefix + MaxPayloadLength

	bufferSize = 2048
)

// VERB_CHARSRT is a ByteSet matching SSMP VERB fields.
var VERB_CHARSET *ByteSet = NewByteSet(
	Range('A', 'Z'),
)

// ID_CHARSET is a ByteSet matching SSMP IDENTIFIER fields.
var ID_CHARSET *ByteSet = NewByteSet(
	Range('a', 'z'),
	Range('A', 'Z'),
	Range('0', '9'),
	All(".:@/-_+=~"),
)

func (d *Decoder) ensureBuffered(n int) error {
	var read int
	err := d.lastErr
	for d.w-d.r < n {
		if err != nil {
			d.lastErr = nil
			return err
		}
		read, err = d.rd.Read(d.buf[d.w:])
		if read > 0 {
			d.w += read
		}
		if err != nil {
			d.lastErr = err
		}
	}
	return nil
}

// Called after a message was decoded, before decoding the next one
func (d *Decoder) Reset() {
	if !d.AtEnd() {
		panic(ErrInvalidMessage)
	}
	// make sure the buffer has room for an entire message
	if d.r >= len(d.buf)-MaxMessageLength {
		copy(d.buf, d.buf[d.r:d.w])
		d.w -= d.r
		d.r = 0
	}
	// mark start of raw message
	d.s = d.r
}

func (d *Decoder) RawMessage() []byte {
	if !d.AtEnd() {
		panic("not a full message")
	}
	return d.buf[d.s:d.r]
}

func (d *Decoder) AtEnd() bool {
	return d.r > d.s && d.buf[d.r-1] == '\n'
}

func (d *Decoder) DecodeCode() (int, error) {
	code := 0
	for i := 0; i < CodeLength; i++ {
		if err := d.ensureBuffered(i + 1); err != nil {
			return -1, err
		}
		c := d.buf[d.r+i]
		if c < '0' || c > '9' {
			return -1, ErrInvalidMessage
		}
		code = 10*code + int(c-'0')
	}
	if err := d.ensureBuffered(CodeLength + 1); err != nil {
		return -1, err
	}
	c := d.buf[d.r+3]
	if c != ' ' && c != '\n' {
		return -1, ErrInvalidMessage
	}
	d.r += 4
	return code, nil
}

func (d *Decoder) DecodeVerb() ([]byte, error) {
	if d.AtEnd() {
		return nil, ErrInvalidMessage
	}
	n := 0
	for n < MaxVerbLength {
		if err := d.ensureBuffered(n + 1); err != nil {
			return nil, err
		}
		c := d.buf[d.r+n]
		n++
		if c == ' ' || c == '\n' {
			if n == 1 {
				break
			}
			d.r += n
			return d.buf[d.r-n : d.r-1], nil
		} else if c < 'A' || c > 'Z' {
			break
		}
	}
	return nil, ErrInvalidMessage
}

func (d *Decoder) DecodeId() ([]byte, error) {
	if d.AtEnd() {
		return nil, ErrInvalidMessage
	}
	n := 0
	for n < MaxIdentifierLength {
		if err := d.ensureBuffered(n + 1); err != nil {
			return nil, err
		}
		c := d.buf[d.r+n]
		n++
		if c == ' ' || c == '\n' {
			if n == 1 {
				break
			}
			d.r += n
			return d.buf[d.r-n : d.r-1], nil
		} else if !ID_CHARSET.Contains(c) {
			break
		}
	}
	return nil, ErrInvalidMessage
}

func (d *Decoder) DecodePayload() ([]byte, error) {
	if d.AtEnd() {
		return nil, ErrInvalidMessage
	}
	if err := d.ensureBuffered(1); err != nil {
		return nil, err
	}
	c := d.buf[d.r]
	// detect binary payload
	if c >= 0 && c <= 3 {
		n, err := d.decodeBinaryPayload()
		if err != nil {
			return nil, err
		}
		d.r += n + BinaryPayloadPrefix + 1
		return d.buf[d.r-n-1 : d.r-1], nil
	}
	return d.decodeTextPayload()
}

// optional id and optional payload
func (d *Decoder) DecodeCompat() ([]byte, error) {
	if d.AtEnd() {
		return d.buf[d.r:d.r], nil
	}
	s := d.r
	if _, err := d.DecodeId(); err != nil && err != ErrInvalidMessage {
		return nil, err
	}
	if d.AtEnd() {
		return d.buf[s : d.r-1], nil
	}
	if _, err := d.DecodePayload(); err != nil {
		d.r = s
		return nil, err
	}
	return d.buf[s : d.r-1], nil
}

func (d *Decoder) decodeTextPayload() ([]byte, error) {
	n := 0
	for n < MaxPayloadLength {
		if err := d.ensureBuffered(n + 1); err != nil {
			return nil, err
		}
		c := d.buf[d.r+n]
		n++
		if c == '\n' {
			// text payload cannot be empty
			if n == 1 {
				break
			}
			d.r += n
			return d.buf[d.r-n : d.r-1], nil
		} else if c >= 0 && c <= 3 {
			break
		}
	}
	return nil, ErrInvalidMessage
}

func (d *Decoder) decodeBinaryPayload() (int, error) {
	d.ensureBuffered(BinaryPayloadPrefix)
	n := 1 + int(uint(d.buf[d.r])<<8+uint(d.buf[d.r+1]))
	if n > MaxPayloadLength {
		return -1, ErrInvalidMessage
	}
	if err := d.ensureBuffered(n + BinaryPayloadPrefix + 1); err != nil {
		return -1, err
	}
	if d.buf[d.r+n+BinaryPayloadPrefix] != '\n' {
		return -1, ErrInvalidMessage
	}
	return n, nil
}
