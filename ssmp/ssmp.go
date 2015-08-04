// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

// Package ssmp provides constants and utilities shared between client and server.
package ssmp

// Requests
const (
	LOGIN       = "LOGIN"
	SUBSCRIBE   = "SUBSCRIBE"
	UNSUBSCRIBE = "UNSUBSCRIBE"
	UCAST       = "UCAST"
	MCAST       = "MCAST"
	BCAST       = "BCAST"
	PING        = "PING"
	PONG        = "PONG"
	CLOSE       = "CLOSE"
)

// Options
const (
	PRESENCE = "PRESENCE"
)

// Response codes
const (
	CodeEvent        = 0
	CodeOk           = 200
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeNotFound     = 404
)

// Reserved identifier for anonymous login.
const Anonymous = "."

// IsValidIdentifier reports whether s is a valid SSMP IDENTIFIER field.
func IsValidIdentifier(s string) bool {
	for i := 0; i < len(s); i++ {
		if !ID_CHARSET.Contains(s[i]) {
			return false
		}
	}
	return true
}

// Equal compares a byte array to a string, to avoid unecessary
// conversions.
func Equal(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}
