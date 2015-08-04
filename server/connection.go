// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

import (
	"bufio"
	"fmt"
	"github.com/aerofs/lipwig/ssmp"
	"io"
	"net"
	"sync/atomic"
	"time"
)

// Connection represents an open client connection to an SSMP server after
// a successful LOGIN.
type Connection struct {
	c net.Conn
	r *bufio.Reader

	User string

	sub map[string]*Topic

	closed int32
}

var (
	ErrInvalidLogin error = fmt.Errorf("invalid LOGIN")
	ErrUnauthorized error = fmt.Errorf("unauthorized")
)

// NewConnection creates a SSMP connection out of a streaming netwrok connection.
//
// This method blocks until either a first message is received or a 10s timeout
// elapses.
//
// Each accepted connection spawns a goroutine continuously reading from the
// underlying network connection and triggering the Dispatcher. The caller must
// keep track of the returned Connection and call the Close method to stop the
// read goroutine and close the udnerlying netwrok connection.
//
// errInvalidLogin is returned if the first message is not a well-formed LOGIN
// request.
// errUnauthorized is returned if the authenticator doesn't accept the provided
// credentials.
func NewConnection(c net.Conn, a Authenticator, d *Dispatcher) (*Connection, error) {
	r := bufio.NewReaderSize(c, 1024)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	l, err := r.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	// strip LF delimiter
	l = l[0 : len(l)-1]

	cmd := ssmp.NewCommand(l)
	verb, err := ssmp.VerbField(cmd)
	if err != nil || !ssmp.Equal(verb, ssmp.LOGIN) {
		return nil, ErrInvalidLogin
	}
	user, err := ssmp.IdField(cmd)
	if err != nil {
		return nil, ErrInvalidLogin
	}
	scheme, err := ssmp.IdField(cmd)
	if err != nil {
		return nil, ErrInvalidLogin
	}
	cred := cmd.Trailing()
	if !a.Auth(c, user, scheme, cred) {
		return nil, ErrUnauthorized
	}
	cc := &Connection{
		c:    c,
		r:    r,
		User: string(user),
	}
	go cc.readLoop(d)
	cc.Write(respOk)
	return cc, nil
}

// Subscribe adds a Topic to the list of subscriptions for the connection.
// This method is not safe to call from multiple goroutines simultaneously.
// It should only be called from the connection's read goroutine.
func (c *Connection) Subscribe(t *Topic) {
	if c.sub == nil {
		c.sub = make(map[string]*Topic)
	}
	c.sub[t.Name] = t
}

// Unsubscribe removes a topic from the list of subscriptions for the connection.
// This method is not safe to call from multiple goroutines simultaneously.
// It should only be called from the connection's read goroutine.
func (c *Connection) Unsubscribe(n []byte) {
	if c.sub != nil {
		delete(c.sub, string(n))
	}
}

// Broadcast sends an identical payload to all users sharing at least one topic.
// This method is not safe to call from multiple goroutines simultaneously.
// It should only be called from the connection's read goroutine.
func (c *Connection) Broadcast(payload []byte) {
	v := make(map[*Connection]bool)
	for _, t := range c.sub {
		t.ForAll(func(cc *Connection, _ bool) {
			if cc != c && !v[cc] {
				v[cc] = true
				cc.Write(payload)
			}
		})
	}
}

var ping []byte = []byte(respEvent + ". " + ssmp.PING + "\n")

func (c *Connection) readLoop(d *Dispatcher) {
	defer d.RemoveConnection(c)
	idle := false
	for {
		if c.isClosed() {
			break
		}
		c.c.SetReadDeadline(time.Now().Add(30 * time.Second))
		l, err := c.r.ReadSlice('\n')
		if c.isClosed() {
			break
		}
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() && !idle {
				idle = true
				c.Write(ping)
				continue
			}
			if err != io.EOF {
				fmt.Println("read failed", c.User, err)
			}
			c.Close()
			break
		}
		idle = false
		d.Dispatch(c, l)
	}
}

func (c *Connection) isClosed() bool {
	return atomic.LoadInt32(&c.closed) != 0
}

// Write writes an arbitrary payload to the underlying network connection.
// The payload MUST be a valid encoding of a SSMP response or event.
// This method us safe to call from multiple goroutines simultaneously.
func (c *Connection) Write(payload []byte) error {
	if c.isClosed() {
		return fmt.Errorf("connection closed", c.User)
	}
	n := len(payload)
	if n < 2 || n > 1024 {
		return fmt.Errorf("invalid message size:", n)
	}
	if payload[n-1] != '\n' {
		return fmt.Errorf("missing message delimiter")
	}
	if _, err := c.c.Write(payload); err != nil {
		c.c.Close()
		return err
	}
	return nil
}

// Close unsubscribes from all topics and closes the underlying network connection.
// This method us safe to call from multiple goroutines simultaneously.
func (c *Connection) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}
	for _, t := range c.sub {
		t.Unsubscribe(c)
	}
	c.c.Close()
}
