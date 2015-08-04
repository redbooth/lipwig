// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package ssmp

// A ByteSet is a compact representation of an immutable set of bytes,
// which offers constant-time membership queries.
type ByteSet struct {
	s [4]uint64
}

type ByteSetInitializer func(*ByteSet)

// Creates a new ByteSet as the union of the given initializers.
func NewByteSet(initializers ...ByteSetInitializer) *ByteSet {
	s := &ByteSet{}
	for _, init := range initializers {
		init(s)
	}
	return s
}

// Initializer for a single byte.
func Byte(c byte) ByteSetInitializer {
	return func(s *ByteSet) {
		s.set(c)
	}
}

// Initializer for a list of bytes.
func All(cs string) ByteSetInitializer {
	return func(s *ByteSet) {
		for i := range cs {
			s.set(cs[i])
		}
	}
}

// Initializer for a range of bytes.
func Range(a, b byte) ByteSetInitializer {
	return func(s *ByteSet) {
		for c := a; c <= b; c++ {
			s.set(c)
		}
	}
}

func (s *ByteSet) set(c byte) {
	s.s[c/64] |= (uint64(1) << (c & 63))
}

// Contains reports whether byte c is in the set.
func (s *ByteSet) Contains(c byte) bool {
	return (s.s[c/64] & (uint64(1) << (c & 63))) != 0
}
