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

// ParseEvent parse s into an Event struct.
// The input must be single event line, stripped of the "000 " prefix.
func ParseEvent(s []byte) (Event, error) {
	var e Event
	cmd := ssmp.NewCommand(s)
	from, err := ssmp.IdField(cmd)
	if err != nil {
		return e, ErrInvalidEvent
	}
	e.From = from
	ev, err := ssmp.VerbField(cmd)
	if err != nil {
		return e, ErrInvalidEvent
	}
	fields := events[string(ev)]
	if fields == 0 {
		return e, ErrInvalidEvent
	}
	e.Name = ev
	if fields == noFields {
		return e, nil
	}
	if (fields & fieldTo) != 0 {
		to, err := ssmp.IdField(cmd)
		if err != nil {
			return e, ErrInvalidEvent
		}
		e.To = to
	}
	if (fields & fieldOption) != 0 {
		payload, err := ssmp.OptionField(cmd)
		if err != nil {
			return e, ErrInvalidEvent
		}
		e.Payload = payload
	} else if (fields & fieldPayload) != 0 {
		payload, err := ssmp.PayloadField(cmd)
		if err != nil {
			return e, ErrInvalidEvent
		}
		e.Payload = payload
	}
	return e, nil
}
