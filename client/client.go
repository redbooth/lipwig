// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package client

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/aerofs/lipwig/ssmp"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidPayload    error = fmt.Errorf("invalid payload")
	ErrInvalidIdentifier error = fmt.Errorf("invalid identifier")
	ErrRequestTooLarge   error = fmt.Errorf("request too large")
)

// Response represents an SSMP response received by a client.
type Response struct {
	// Code specifies the response code (200, 400, ...)
	Code int

	// Message is the optional response payload.
	Message string
}

// The EventHandler interface is used to react to asynchronous server-sent events.
type EventHandler interface {
	HandleEvent(event Event)
}

// Client is a simple SSMP client wrapper over a network connection.
//
// All requests are blocking and request pipelining is not currently supported.
//
// Unless otherwise specified, it is not safe to invoke methods on a
// Client from multiple goroutines simultaneously.
type Client interface {
	// EventHandler retrieves the current EventHandler.
	// This method is safe to call from multiple goroutines simultaneously.
	EventHandler() EventHandler

	// SetEventHandler makes h the current EventHandler.
	// This method is safe to call from multiple goroutines simultaneously.
	SetEventHandler(h EventHandler)

	// Close closes the SSMP client.
	// A CLOSE message is sent to the server before closing the underlying
	// network connection.
	Close()

	// Login makes a LOGIN request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Login(user string, scheme string, credential string) (Response, error)

	// Subscribe makes a SUBSCRIBE request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Subscribe(topic string) (Response, error)

	// SubscribeWithPresence makes a SUBSCRIBE request with the PRESENCE flag.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	SubscribeWithPresence(topic string) (Response, error)

	// Unsubscribe makes a UNSUBSCRIBE request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Unsubscribe(topic string) (Response, error)

	// Ucast makes a UCAST request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Ucast(user string, payload string) (Response, error)

	// Mcast makes a MCAST request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Mcast(topic string, payload string) (Response, error)

	// Bcast makes a BCAST request.
	// An error is returned in case of network or protocol error. A non-2xx
	// response doesn't cause an error.
	Bcast(payload string) (Response, error)
}

type client struct {
	RequestChecks bool

	c  net.Conn
	h  atomic.Value
	wg sync.WaitGroup

	responses chan Response
}

type DiscardHandler struct{}

func (h *DiscardHandler) HandleEvent(_ Event) {}

var Discard = &DiscardHandler{}

var bufPool *sync.Pool = &sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// NewClient creates a new SSMP client using the given network connection
// and event handler.
func NewClient(c net.Conn, h EventHandler) Client {
	cc := &client{
		c:         c,
		responses: make(chan Response),
	}
	cc.SetEventHandler(h)
	cc.wg.Add(1)
	go cc.readLoop()
	return cc
}

func (c *client) Close() {
	_, _ = c.request(ssmp.CLOSE, "", "")
	c.c.Close()
	c.wg.Wait()
}

func (c *client) EventHandler() EventHandler {
	return c.h.Load().(EventHandler)
}

func (c *client) SetEventHandler(h EventHandler) {
	if h == nil {
		// Value doesn't accept nil
		c.h.Store(Discard)
	} else {
		c.h.Store(h)
	}
}

func (c *client) Login(user string, scheme string, cred string) (Response, error) {
	return c.request(ssmp.LOGIN, user, scheme+" "+cred)
}

func (c *client) Subscribe(topic string) (Response, error) {
	return c.request(ssmp.SUBSCRIBE, topic, "")
}

func (c *client) SubscribeWithPresence(topic string) (Response, error) {
	return c.request(ssmp.SUBSCRIBE, topic, ssmp.PRESENCE)
}

func (c *client) Unsubscribe(topic string) (Response, error) {
	return c.request(ssmp.UNSUBSCRIBE, topic, "")
}

func (c *client) Ucast(user string, payload string) (Response, error) {
	return c.request(ssmp.UCAST, user, payload)
}

func (c *client) Mcast(topic string, payload string) (Response, error) {
	return c.request(ssmp.MCAST, topic, payload)
}

func (c *client) Bcast(payload string) (Response, error) {
	return c.request(ssmp.BCAST, "", payload)
}

func (c *client) request(cmd string, to string, payload string) (Response, error) {
	var r Response
	if c.RequestChecks {
		if !ssmp.IsValidIdentifier(to) {
			return r, ErrInvalidIdentifier
		}
		// quick size check, before allocating buffer
		if len(cmd)+len(to)+len(payload) > 1022 {
			return r, ErrRequestTooLarge
		}
		if strings.Contains(payload, "\n") {
			return r, ErrInvalidPayload
		}
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.WriteString(cmd)
	if len(to) > 0 {
		buf.WriteByte(' ')
		buf.WriteString(to)
	}
	if len(payload) > 0 {
		buf.WriteByte(' ')
		buf.WriteString(payload)
	}
	buf.WriteByte('\n')
	if c.RequestChecks && buf.Len() > 1024 {
		return r, ErrRequestTooLarge
	}
	_, err := c.c.Write(buf.Bytes())
	bufPool.Put(buf)
	if err != nil {
		c.c.Close()
		return r, err
	}
	r = <-c.responses
	if r.Code == 0 {
		return r, fmt.Errorf("connection closed")
	}
	return r, nil
}

var ping []byte = []byte(ssmp.PING + "\n")
var pong []byte = []byte(ssmp.PONG + "\n")

func (c *client) readLoop() {
	defer c.wg.Done()
	defer close(c.responses)

	idle := false
	r := bufio.NewReaderSize(c.c, 1024)
	for {
		c.c.SetReadDeadline(time.Now().Add(30 * time.Second))
		l, err := r.ReadSlice('\n')
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() && !idle {
				idle = true
				c.c.Write(ping)
				continue
			}
			// unwrap network error
			if oerr, ok := err.(*net.OpError); ok {
				err = oerr.Err
			}
			if err != io.EOF && err.Error() != "use of closed network connection" {
				fmt.Printf("Client[%p] Failed to read: %v\n", c, err)
			}
			break
		}
		idle = false
		// strip LF delimiter
		l = l[0 : len(l)-1]
		if len(l) < 3 || (len(l) > 3 && l[3] != ' ') {
			if err != io.EOF {
				fmt.Printf("Client[%p] Invalid server message: %v\n", c, err)
			}
			break
		}
		code := responseCode(l)
		if code < 0 {
			fmt.Printf("Client[%p] Invalid server message: %v\n", c, l)
			break
		}
		var payload []byte
		if len(l) >= 4 {
			payload = l[4:]
		}
		if code == ssmp.CodeEvent {
			ev, err := ParseEvent(payload)
			if err != nil {
				fmt.Printf("Client[%p] Invalid event: %v\n", c, err)
				break
			}
			if ssmp.Equal(ev.Name, ssmp.PING) {
				c.c.Write(pong)
				continue
			}
			if ssmp.Equal(ev.Name, ssmp.PONG) {
				continue
			}
			h := c.EventHandler()
			if h == nil {
				continue
			}
			h.HandleEvent(ev)
			continue
		}
		c.responses <- Response{
			Code:    code,
			Message: string(payload),
		}
	}
	c.c.Close()
}

func responseCode(s []byte) int {
	n := 0
	for i := 0; i < 3; i++ {
		if s[i] < '0' || s[i] > '9' {
			return -1
		}
		n = 10*n + int(s[i]-'0')
	}
	return n
}
