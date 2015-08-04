// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package ssmp

import (
	"fmt"
)

// Command is a helper structure for request/event parsing.
type Command struct {
	s []byte
	i int
}

// Create a new Command to parse input.
// Command does not make a copy of input. That slice MUST NOT be modified
// during while the Command is used.
func NewCommand(input []byte) *Command {
	return &Command{s: input}
}

// AtEnd reports whether the end of the input was reached.
func (c *Command) AtEnd() bool {
	return c.i == len(c.s)
}

// Trailing returns the remaining unparsed input.
func (c *Command) Trailing() []byte {
	return c.s[c.i:]
}

// SkipSpaces moves the input index to the next non-space byte.
// It returns true if at least one space was skipped or the end
// of the input was reached.
func (c *Command) SkipSpaces() bool {
	start := c.i
	for c.i < len(c.s) && c.s[c.i] == ' ' {
		c.i++
	}
	return c.AtEnd() || c.i > start
}

// Consume reads input bytes contained in cs.
// It returns a slice with the bytes read, which might be empty,
// and updates the input index to point after the last byte read.
// The returned slice MUST NOT be modified.
func (c *Command) Consume(cs *ByteSet) []byte {
	if c.i == len(c.s) {
		return []byte{}
	}
	start := c.i
	for c.i < len(c.s) && cs.Contains(c.s[c.i]) {
		c.i++
	}
	return c.s[start:c.i]
}

type FieldExtractor func(c *Command) ([]byte, error)

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

// VerbField extracts a VERB field from c.
// The returned slice MUST NOT be modified.
func VerbField(c *Command) ([]byte, error) {
	r := c.Consume(VERB_CHARSET)
	if len(r) == 0 {
		return r, fmt.Errorf("unexpected EOL")
	}
	if !c.SkipSpaces() {
		return r, fmt.Errorf("invalid verb character: 0x%02x", c.s[c.i])
	}
	return r, nil
}

// IdField extracts an IDENTIFIER field from c.
// The returned slice MUST NOT be modified.
func IdField(c *Command) ([]byte, error) {
	r := c.Consume(ID_CHARSET)
	if len(r) == 0 {
		return r, fmt.Errorf("unexpected EOL")
	}
	if !c.SkipSpaces() {
		return r, fmt.Errorf("invalid id character: 0x%02x", c.s[c.i])
	}
	return r, nil
}

// PayloadField extracts a PAYLOAD field from c.
// The returned slice MUST NOT be modified.
func PayloadField(c *Command) ([]byte, error) {
	r := c.s[c.i:]
	if len(r) == 0 {
		return r, fmt.Errorf("unexpected EOL")
	}
	c.i = len(c.s)
	return r, nil
}

// OptionField extracts an optional IDENTIFIER field from c.
// It differs from VerbField in that the returned slice may be empty.
func OptionField(c *Command) ([]byte, error) {
	r := c.Consume(VERB_CHARSET)
	if !c.SkipSpaces() {
		return []byte{}, fmt.Errorf("invalid verb character: 0x%02x", c.s[c.i])
	}
	return r, nil
}
