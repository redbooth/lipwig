// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

import (
	"bytes"
	"fmt"
	"github.com/aerofs/lipwig/ssmp"
	"sync"
)

// A Dispatcher parses incoming requests and reacts to them appropriately.
// All methods are safe to call from multiple goroutines simultaneously.
type Dispatcher struct {
	topics      *TopicManager
	connections *ConnectionManager
	handlers    map[string]handler

	bufPool sync.Pool
}

// NewDispatcher creates a SSMP dispatcher using the given TopicManager and ConnectionManager.
func NewDispatcher(topics *TopicManager, connections *ConnectionManager) *Dispatcher {
	return &Dispatcher{
		topics:      topics,
		connections: connections,
		handlers: map[string]handler{
			ssmp.SUBSCRIBE:   h(onSubscribe, fieldTo|fieldOption),
			ssmp.UNSUBSCRIBE: h(onUnsubscribe, fieldTo),
			ssmp.UCAST:       h(onUcast, fieldTo|fieldPayload),
			ssmp.MCAST:       h(onMcast, fieldTo|fieldPayload),
			ssmp.BCAST:       h(onBcast, fieldPayload),
			ssmp.PING:        h(onPing, 0),
			ssmp.PONG:        h(onPong, 0),
			ssmp.CLOSE:       h(onClose, 0),
		},
		bufPool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Dispatch parses req, reacts appropriately and sends a response to c.
func (d *Dispatcher) Dispatch(c *Connection, verb []byte) bool {
	if ssmp.Equal(verb, ssmp.LOGIN) {
		fmt.Println("attempted re-login")
		c.Write(respNotAllowed)
		return false
	}
	h := d.handlers[string(verb)]
	if h.h == nil {
		// discard unknown command
		if _, err := c.r.DecodeCompat(); err != nil {
			return false
		}
		fmt.Println("unsupported command:", verb)
		c.Write(respNotImplemented)
		return true
	}
	var err error
	var to []byte
	var payload []byte
	if (h.f & fieldTo) != 0 {
		if to, err = c.r.DecodeId(); err != nil {
			return false
		}
	}
	if (h.f & fieldPayload) != 0 {
		if (h.f&fieldOption) == fieldOption && c.r.AtEnd() {
			payload = []byte{}
		} else if payload, err = c.r.DecodePayload(); err != nil {
			return false
		}
	}
	if !c.r.AtEnd() {
		return false
	}
	h.h(c, to, payload, c.r.RawMessage(), d)
	return true
}

func (d *Dispatcher) GetConnection(user []byte) *Connection {
	return d.connections.GetConnection(user)
}

func (d *Dispatcher) RemoveConnection(c *Connection) {
	d.connections.RemoveConnection(c)
}

func (d *Dispatcher) buffer() *bytes.Buffer {
	buf := d.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (d *Dispatcher) release(b *bytes.Buffer) {
	d.bufPool.Put(b)
}

////////////////////////////////////////////////////////////////////////////////

type handlerFunc func(*Connection, []byte, []byte, []byte, *Dispatcher)

const (
	fieldTo      = 1
	fieldPayload = 2
	fieldOption  = 6
)

type handler struct {
	f int32
	h handlerFunc
}

func h(h handlerFunc, f int32) handler {
	return handler{f: f, h: h}
}

func onSubscribe(c *Connection, n, option, s []byte, d *Dispatcher) {
	from := c.User
	if from == ssmp.Anonymous {
		c.Write(respNotAllowed)
		return
	}
	presence := ssmp.Equal(option, ssmp.PRESENCE)
	if len(option) > 0 && !presence {
		fmt.Println("unrecognized option:", option)
		c.Write(respBadRequest)
		return
	}
	t := d.topics.GetOrCreateTopic(n)
	if !t.Subscribe(c, presence) {
		// already subscribed
		c.Write(respConflict)
		return
	}

	c.Subscribe(t)
	c.Write(respOk)

	// notify existing subscribers of new sub
	buf := d.buffer()
	buf.Grow(5 + len(from) + len(s))
	buf.WriteString(respEvent)
	buf.WriteString(from)
	buf.WriteByte(' ')
	buf.Write(s)
	event := buf.Bytes()
	batch := event[4+len(from) : 15+len(from)+len(n)]

	var buf2 *bytes.Buffer = nil
	if presence {
		buf2 = d.buffer()
	}

	t.ForAll(func(cc *Connection, wantsPresence bool) {
		if c == cc {
			return
		}
		if wantsPresence {
			cc.Write(event)
		}
		if presence {
			buf2.WriteString(respEvent)
			buf2.WriteString(cc.User)
			buf2.Write(batch)
			if wantsPresence {
				buf2.WriteString(" PRESENCE\n")
			} else {
				buf2.WriteByte('\n')
			}
			if buf2.Len() > 512 {
				c.Write(buf2.Bytes())
				buf2.Reset()
			}
		}
	})
	d.release(buf)
	if buf2 != nil {
		if buf2.Len() > 0 {
			c.Write(buf2.Bytes())
		}
		d.release(buf2)
	}
}

func onUnsubscribe(c *Connection, n, _, s []byte, d *Dispatcher) {
	from := c.User
	if from == ssmp.Anonymous {
		c.Write(respNotAllowed)
		return
	}
	t := d.topics.GetTopic(n)
	if t == nil || !t.Unsubscribe(c) {
		c.Write(respNotFound)
		return
	}
	c.Unsubscribe(n)
	buf := d.buffer()
	buf.Grow(5 + len(from) + len(s))
	buf.WriteString(respEvent)
	buf.WriteString(from)
	buf.WriteByte(' ')
	buf.Write(s)
	event := buf.Bytes()
	t.ForAll(func(cc *Connection, wantsPresence bool) {
		if wantsPresence {
			cc.Write(event)
		}
	})
	d.release(buf)
	c.Write(respOk)
}

func onBcast(c *Connection, _, _, s []byte, d *Dispatcher) {
	from := c.User
	if from == ssmp.Anonymous {
		c.Write(respNotAllowed)
		return
	}
	buf := d.buffer()
	buf.Grow(5 + len(from) + len(s))
	buf.WriteString(respEvent)
	buf.WriteString(from)
	buf.WriteByte(' ')
	buf.Write(s)
	c.Broadcast(buf.Bytes())
	d.release(buf)
	c.Write(respOk)
}

func onUcast(c *Connection, u, _, s []byte, d *Dispatcher) {
	from := c.User
	cc := d.connections.GetConnection(u)
	if cc == nil {
		c.Write(respNotFound)
	} else {
		buf := d.buffer()
		buf.Grow(5 + len(from) + len(s))
		buf.WriteString(respEvent)
		buf.WriteString(from)
		buf.WriteByte(' ')
		buf.Write(s)
		cc.Write(buf.Bytes())
		d.release(buf)
		c.Write(respOk)
	}
}

func onMcast(c *Connection, n, _, s []byte, d *Dispatcher) {
	from := c.User
	t := d.topics.GetTopic(n)
	if t != nil {
		buf := d.buffer()
		buf.Grow(5 + len(from) + len(s))
		buf.WriteString(respEvent)
		buf.WriteString(from)
		buf.WriteByte(' ')
		buf.Write(s)
		msg := buf.Bytes()
		t.ForAll(func(cc *Connection, _ bool) {
			if c != cc {
				cc.Write(msg)
			}
		})
		d.release(buf)
	}
	c.Write(respOk)
}

var pong []byte = []byte(respEvent + ". " + ssmp.PONG + "\n")

func onPing(c *Connection, _, _, _ []byte, _ *Dispatcher) {
	c.Write(pong)
}

func onPong(c *Connection, _, _, _ []byte, _ *Dispatcher) {
	// nothing to see here...
}

func onClose(c *Connection, _, _, _ []byte, _ *Dispatcher) {
	c.Write(respOk)
	c.Close()
}
