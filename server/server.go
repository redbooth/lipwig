// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

import (
	"crypto/tls"
	"fmt"
	"github.com/aerofs/lipwig/ssmp"
	"io"
	"net"
	"sync"
)

// A ConnectionManager manages a set of Connection.
// All methods are safe to call from multiple goroutines simultaneously.
type ConnectionManager struct {
	connection  sync.Mutex
	anonymous   map[*Connection]*Connection
	connections map[string]*Connection
}

// A TopicManager manages a set of Topic.
// All methods are safe to call from multiple goroutines simultaneously.
type TopicManager struct {
	topic  sync.Mutex
	topics map[string]*Topic
}

////////////////////////////////////////////////////////////////////////////////

// server implements Server, ConnectionManager and TopicManager
type Server struct {
	ConnectionManager
	TopicManager

	l    *net.TCPListener
	cfg  *tls.Config
	auth Authenticator

	// used to cleanly Stop the goroutine spawned by Start
	w sync.WaitGroup

	dispatcher *Dispatcher
}

// NewServer creates a new SSMP server from a TCP Listener, an Authenticator
// and a TLS configuration.
func NewServer(l net.Listener, auth Authenticator, cfg *tls.Config) *Server {
	s := &Server{
		l:    l.(*net.TCPListener),
		cfg:  cfg,
		auth: auth,
		ConnectionManager: ConnectionManager{
			anonymous:   make(map[*Connection]*Connection),
			connections: make(map[string]*Connection),
		},
		TopicManager: TopicManager{
			topics: make(map[string]*Topic),
		},
	}
	s.dispatcher = NewDispatcher(&s.TopicManager, &s.ConnectionManager)
	return s
}

// Serve accept connections in the calling goroutine and only returns
// in case of error.
func (s *Server) Serve() error {
	s.w.Add(1)
	return s.serve()
}

// Start accepts connection in a new goroutine and returns the Server
// This allows the following terse idiom:
//		defer s.Start().Stop()
func (s *Server) Start() *Server {
	s.w.Add(1)
	go s.serve()
	return s
}

// ListeningPort returns the TCP port to which the underlying Listener is bound.
func (s *Server) ListeningPort() int {
	return s.l.Addr().(*net.TCPAddr).Port
}

// Stop stops accepting new connections and immediately closes all existing
// connections. Serve
func (s *Server) Stop() {
	s.l.Close()
	s.connection.Lock()
	for _, c := range s.connections {
		c.Close()
	}
	for c := range s.anonymous {
		c.Close()
	}
	s.connection.Unlock()
	s.w.Wait()
}

// DumpStats writes some internal stats to the given Writer.
func (s *Server) DumpStats(w io.Writer) {
	io.WriteString(w, "------- server stats -------\n")
	s.connection.Lock()
	fmt.Fprintf(w, "%5d anonymous connections\n", len(s.anonymous))
	for c := range s.anonymous {
		fmt.Fprintf(w, "\t%p %v\n", c, c.c.RemoteAddr())
	}
	fmt.Fprintf(w, "%5d named connections\n", len(s.connections))
	for u, c := range s.connections {
		fmt.Fprintf(w, "\t%p %v %s %s\n", c, c.c.RemoteAddr(), u, c.User)
		// FIXME: synchronization to prevent race with SUB/UNSUB handling
		for n, t := range c.sub {
			fmt.Fprintf(w, "\t\t%s %p\n", n, t)
		}
	}
	s.connection.Unlock()
	s.topic.Lock()
	fmt.Fprintf(w, "%5d active topics\n", len(s.topics))
	for n, t := range s.topics {
		fmt.Fprintf(w, "\t%p %s %s\n", t, n, t.Name)
		for c, p := range t.c {
			fmt.Fprintf(w, "\t\t%p %v %s\n", c, p, c.User)
		}
	}
	s.topic.Unlock()
	io.WriteString(w, "----------------------------\n")
}

func (s *Server) serve() error {
	defer s.w.Done()
	for {
		c, err := s.l.AcceptTCP()
		if err != nil {
			// TODO: handle "too many open files"?
			return err
		}
		go s.connect(s.configure(c))
	}
}

func (s *Server) configure(c *net.TCPConn) net.Conn {
	c.SetNoDelay(true)
	if s.cfg == nil {
		return c
	}
	return tls.Server(c, s.cfg)
}

func (s *Server) connect(c net.Conn) {
	cc, err := NewConnection(c, s.auth, s.dispatcher)
	if err != nil {
		fmt.Println("connect rejected:", err)
		if err == ErrUnauthorized {
			c.Write(s.auth.Unauthorized())
		} else if err == ErrInvalidLogin {
			c.Write(respBadRequest)
		}
		c.Close()
		return
	}
	var old *Connection
	u := cc.User
	s.connection.Lock()
	if u == ssmp.Anonymous {
		s.anonymous[cc] = cc
	} else {
		old = s.connections[u]
		s.connections[u] = cc
	}
	s.connection.Unlock()
	if old != nil {
		old.Close()
	}
}

func (s *ConnectionManager) GetConnection(user []byte) *Connection {
	s.connection.Lock()
	c := s.connections[string(user)]
	s.connection.Unlock()
	return c
}

func (s *ConnectionManager) RemoveConnection(c *Connection) {
	s.connection.Lock()
	if c.User == ssmp.Anonymous {
		delete(s.anonymous, c)
	} else if s.connections[c.User] == c {
		delete(s.connections, c.User)
	} else {
		fmt.Println("mismatching connection closed", c.User)
	}
	s.connection.Unlock()
}

func (s *TopicManager) GetOrCreateTopic(name []byte) *Topic {
	s.topic.Lock()
	t := s.topics[string(name)]
	if t == nil {
		t = NewTopic(string(name), s)
		s.topics[string(name)] = t
	}
	s.topic.Unlock()
	return t
}

func (s *TopicManager) GetTopic(name []byte) *Topic {
	s.topic.Lock()
	t := s.topics[string(name)]
	s.topic.Unlock()
	return t
}

func (s *TopicManager) RemoveTopic(name string) {
	s.topic.Lock()
	delete(s.topics, name)
	s.topic.Unlock()
}
