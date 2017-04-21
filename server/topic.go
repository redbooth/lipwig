// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

import (
	"sync"
)

type TopicVisitor func(c *Connection, wantsPresence bool)

// Topic represents a SSMP multicast topic.
//
// All methods can be safely called from multiple goroutines simultaneously.
type Topic struct {
	Name string
	tm   *TopicManager
	l    sync.RWMutex
	c    map[*Connection]bool
}

// NewTopic creates a new Topic with a given name.
// The topic keeps track of the TopicManager to self-harvest when the last
// subscriber set becomes empty.
func NewTopic(name string, tm *TopicManager) *Topic {
	return &Topic{
		Name: name,
		tm:   tm,
		c:    make(map[*Connection]bool),
	}
}

// Subscribe adds a connection to the set of subscribers.
// The presence flag indicates whether the connection is interested in
// receiving presence events about other subscribers.
// It returns true if a new subscription was made, or false if the
// connection was already subscribed to the topic.
func (t *Topic) Subscribe(c *Connection, presence bool) bool {
	t.l.Lock()
	_, subscribed := t.c[c]
	if !subscribed {
		t.c[c] = presence
	}
	t.l.Unlock()
	return !subscribed
}

// Unsubscribe removes a connection from the set of subscribers.
// It returns true if the connection was unsubscribed, or false it it
// wasn't subscribed to the topic.
func (t *Topic) Unsubscribe(c *Connection) bool {
	t.l.Lock()
	_, subscribed := t.c[c]
	delete(t.c, c)
	if len(t.c) == 0 {
		t.tm.RemoveTopic(t.Name)
	}
	t.l.Unlock()
	return subscribed
}

// ForAll executes v once for every subscribers.
func (t *Topic) ForAll(v TopicVisitor) {
	t.l.RLock()
	defer t.l.RUnlock()
	for c, presence := range t.c {
		if !c.isClosed() {
			v(c, presence)
		}
	}
}
