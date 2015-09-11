// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package client

import (
	"fmt"
	"github.com/aerofs/lipwig/ssmp"
)

// Event represents a decoded SSMP server-sent event.
// All the fields are slices of the client input buffer.
// They MUST NOT be modified and copies MUST be made if the fields are
// to be used after the event handler returns.
type Event struct {
	From    []byte
	Name    []byte
	To      []byte
	Payload []byte
}

const (
	fieldTo = 1 << iota
	fieldPayload
	fieldOption

	noFields = -1
)

var events map[string]int = map[string]int{
	ssmp.SUBSCRIBE:   fieldTo | fieldOption,
	ssmp.UNSUBSCRIBE: fieldTo,
	ssmp.UCAST:       fieldTo | fieldPayload,
	ssmp.MCAST:       fieldTo | fieldPayload,
	ssmp.BCAST:       fieldPayload,
	ssmp.PING:        noFields,
	ssmp.PONG:        noFields,
}

var ErrInvalidEvent error = fmt.Errorf("invalid event")
