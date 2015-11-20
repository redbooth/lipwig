// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package main

import (
	"github.com/aerofs/lipwig/client"
	"github.com/aerofs/lipwig/server"
	"github.com/aerofs/lipwig/ssmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

type test_auth struct{}

func (a *test_auth) Auth(c net.Conn, user, scheme, cred []byte) bool {
	return !ssmp.Equal(user, "reject")
}

func (a *test_auth) Unauthorized() []byte {
	return []byte("401\n")
}

type EventQueue struct {
	q chan client.Event
}

func (q *EventQueue) HandleEvent(ev client.Event) {
	q.q <- ev
}

type TestClient struct {
	client.Client
	h client.EventHandler
}

var ENDPOINT string

func NewServer() *server.Server {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := server.NewServer(l, &test_auth{}, nil)
	ENDPOINT = "127.0.0.1:" + strconv.Itoa(s.ListeningPort())
	return s
}

func NewClientWithHandler(h client.EventHandler) TestClient {
	c, err := net.Dial("tcp", ENDPOINT)
	if err != nil {
		panic(err)
	}
	return TestClient{
		Client: client.NewClient(c, h),
		h:      h,
	}
}

func NewClient() TestClient {
	return NewClientWithHandler(&EventQueue{
		q: make(chan client.Event, 20),
	})
}

func NewLoggedInClientWithHandler(user string, h client.EventHandler) TestClient {
	c := NewClientWithHandler(h)
	r, err := c.Login(user, "none", "")
	if err != nil || r.Code != ssmp.CodeOk {
		panic("failed to login")
	}
	return c
}

func NewLoggedInClient(user string) TestClient {
	return NewLoggedInClientWithHandler(user, &EventQueue{
		q: make(chan client.Event, 20),
	})
}

func NewDiscardingLoggedInClient(user string) TestClient {
	return NewLoggedInClientWithHandler(user, client.Discard)
}

func u(hack ...interface{}) []interface{} {
	return hack
}

func (c TestClient) expect(t *testing.T, events ...client.Event) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		q := c.h.(*EventQueue)
		for _, expected := range events {
			select {
			case ev := <-q.q:
				assert.Equal(t, expected.Name, ev.Name)
				assert.Equal(t, expected.From, ev.From)
				assert.Equal(t, expected.To, ev.To)
				assert.Equal(t, expected.Payload, ev.Payload)
			case _ = <-time.After(5 * time.Second):
				assert.Fail(t, "timed out waiting for event")
			}
		}
		wg.Done()
	}()
	return &wg
}

func expect(t *testing.T, code int, hack []interface{}) {
	require.Nil(t, hack[1])
	require.Equal(t, code, hack[0].(client.Response).Code)
}

////////////////////////////////////////////////////////////////////////////////

func TestClient_should_accept_login(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewClient()
	defer c.Close()

	expect(t, ssmp.CodeOk, u(c.Login("foo", "none", "")))
}

func TestClient_should_reject_login(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewClient()
	defer c.Close()

	expect(t, ssmp.CodeUnauthorized, u(c.Login("reject", "none", "")))
}

func TestClient_should_fail_unicast_to_invalid(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewLoggedInClient("foo")
	defer c.Close()

	expect(t, ssmp.CodeBadRequest, u(c.Ucast("!@#$%^&*", "hello")))
}

func TestClient_should_fail_unicast_to_non_existent(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewLoggedInClient("foo")
	defer c.Close()

	expect(t, ssmp.CodeNotFound, u(c.Ucast("bar", "hello")))
}

func TestClient_should_unicast_self(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewLoggedInClient("foo")
	defer c.Close()

	w := c.expect(t, client.Event{
		Name:    []byte(ssmp.UCAST),
		From:    []byte("foo"),
		To:      []byte("foo"),
		Payload: []byte("hello"),
	})

	expect(t, ssmp.CodeOk, u(c.Ucast("foo", "hello")))
	w.Wait()
}

func TestClient_should_unicast_self_binary(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewLoggedInClient("foo")
	defer c.Close()

	w := c.expect(t, client.Event{
		Name:    []byte(ssmp.UCAST),
		From:    []byte("foo"),
		To:      []byte("foo"),
		Payload: []byte("hello"),
	})

	expect(t, ssmp.CodeOk, u(c.Ucast("foo", string([]byte{0, 4})+"hello")))
	w.Wait()
}

func TestClient_should_reject_unicast_binary_short(t *testing.T) {
	defer NewServer().Start().Stop()
	c := NewLoggedInClient("foo")
	defer c.Close()

	expect(t, ssmp.CodeBadRequest, u(c.Ucast("foo", string([]byte{0, 3})+"hello")))
}

func TestClient_should_unicast_other(t *testing.T) {
	defer NewServer().Start().Stop()
	foo := NewLoggedInClient("foo")
	defer foo.Close()
	bar := NewLoggedInClient("bar")
	defer bar.Close()

	w := bar.expect(t, client.Event{
		Name:    []byte(ssmp.UCAST),
		From:    []byte("foo"),
		To:      []byte("bar"),
		Payload: []byte("hello"),
	})

	expect(t, ssmp.CodeOk, u(foo.Ucast("bar", "hello")))
	w.Wait()

	w = foo.expect(t, client.Event{
		Name:    []byte(ssmp.UCAST),
		From:    []byte("bar"),
		To:      []byte("foo"),
		Payload: []byte("world"),
	})

	expect(t, ssmp.CodeOk, u(bar.Ucast("foo", "world")))
	w.Wait()
}

func TestClient_should_multicast(t *testing.T) {
	defer NewServer().Start().Stop()
	foo := NewLoggedInClient("foo")
	defer foo.Close()
	bar := NewLoggedInClient("bar")
	defer bar.Close()

	expect(t, ssmp.CodeOk, u(foo.Subscribe("chat")))
	expect(t, ssmp.CodeOk, u(bar.Subscribe("chat")))

	w := bar.expect(t, client.Event{
		Name:    []byte(ssmp.MCAST),
		From:    []byte("foo"),
		To:      []byte("chat"),
		Payload: []byte("hello"),
	})
	expect(t, ssmp.CodeOk, u(foo.Mcast("chat", "hello")))
	w.Wait()

	w = foo.expect(t, client.Event{
		Name:    []byte(ssmp.MCAST),
		From:    []byte("bar"),
		To:      []byte("chat"),
		Payload: []byte("world"),
	})
	expect(t, ssmp.CodeOk, u(bar.Mcast("chat", "world")))
	w.Wait()
}

func TestClient_should_get_presence(t *testing.T) {
	defer NewServer().Start().Stop()
	foo := NewLoggedInClient("foo")
	defer foo.Close()
	bar := NewLoggedInClient("bar")
	defer bar.Close()

	w1 := foo.expect(t, client.Event{
		Name:    []byte(ssmp.SUBSCRIBE),
		From:    []byte("bar"),
		To:      []byte("chat"),
		Payload: []byte("PRESENCE"),
	})

	w2 := bar.expect(t, client.Event{
		Name:    []byte(ssmp.SUBSCRIBE),
		From:    []byte("foo"),
		To:      []byte("chat"),
		Payload: []byte("PRESENCE"),
	})

	expect(t, ssmp.CodeOk, u(foo.SubscribeWithPresence("chat")))
	expect(t, ssmp.CodeOk, u(bar.SubscribeWithPresence("chat")))

	w1.Wait()
	w2.Wait()

	w1 = bar.expect(t, client.Event{
		Name: []byte(ssmp.UNSUBSCRIBE),
		From: []byte("foo"),
		To:   []byte("chat"),
	})
	expect(t, ssmp.CodeOk, u(foo.Unsubscribe("chat")))
	w1.Wait()
}

func TestClient_should_unsubscribe_on_close(t *testing.T) {
	defer NewServer().Start().Stop()
	foo := NewLoggedInClient("foo")
	bar := NewLoggedInClient("bar")
	defer bar.Close()

	w := bar.expect(t, client.Event{
		Name:    []byte(ssmp.SUBSCRIBE),
		From:    []byte("foo"),
		To:      []byte("chat"),
		Payload: []byte{},
	})

	expect(t, ssmp.CodeOk, u(foo.Subscribe("chat")))
	expect(t, ssmp.CodeOk, u(bar.SubscribeWithPresence("chat")))

	w.Wait()

	w = bar.expect(t, client.Event{
		Name: []byte(ssmp.UNSUBSCRIBE),
		From: []byte("foo"),
		To:   []byte("chat"),
	})
	foo.Close()
	w.Wait()
}

func TestClient_should_broadcast(t *testing.T) {
	defer NewServer().Start().Stop()
	foo := NewLoggedInClient("foo")
	defer foo.Close()
	bar := NewLoggedInClient("bar")
	defer bar.Close()
	baz := NewLoggedInClient("baz")
	defer baz.Close()

	expect(t, ssmp.CodeOk, u(foo.Subscribe("foo:bar")))
	expect(t, ssmp.CodeOk, u(bar.Subscribe("foo:bar")))

	expect(t, ssmp.CodeOk, u(foo.Subscribe("foo:baz")))
	expect(t, ssmp.CodeOk, u(baz.Subscribe("foo:baz")))

	expect(t, ssmp.CodeOk, u(bar.Subscribe("bar:baz")))
	expect(t, ssmp.CodeOk, u(baz.Subscribe("bar:baz")))

	w1 := foo.expect(t, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("bar"),
		Payload: []byte("bart"),
	}, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("baz"),
		Payload: []byte("baza"),
	})
	w2 := bar.expect(t, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("foo"),
		Payload: []byte("fool"),
	}, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("baz"),
		Payload: []byte("baza"),
	})
	w3 := baz.expect(t, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("foo"),
		Payload: []byte("fool"),
	}, client.Event{
		Name:    []byte(ssmp.BCAST),
		From:    []byte("bar"),
		Payload: []byte("bart"),
	})

	expect(t, ssmp.CodeOk, u(foo.Bcast("fool")))
	expect(t, ssmp.CodeOk, u(bar.Bcast("bart")))
	expect(t, ssmp.CodeOk, u(baz.Bcast("baza")))

	w1.Wait()
	w2.Wait()
	w3.Wait()
}

func BenchmarkUCAST_self(b *testing.B) {
	defer NewServer().Start().Stop()
	foo := NewDiscardingLoggedInClient("foo")
	defer foo.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		foo.Ucast("foo", "hello world")
	}
	b.StopTimer()
}

func BenchmarkParallelUCAST_self(b *testing.B) {
	defer NewServer().Start().Stop()
	foo := NewDiscardingLoggedInClient("foo")
	defer foo.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			foo.Ucast("foo", "hello world")
		}
	})
	b.StopTimer()
}

func BenchmarkMCAST_100(b *testing.B) {
	defer NewServer().Start().Stop()
	var c [100]TestClient
	for i := 0; i < len(c); i++ {
		c[i] = NewDiscardingLoggedInClient("foo" + strconv.Itoa(i))
		c[i].Subscribe("topic")
		defer c[i].Close()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c[i%len(c)].Mcast("topic", "hello world")
	}
	b.StopTimer()
}

func BenchmarkPRESENCE_100(b *testing.B) {
	defer NewServer().Start().Stop()
	var c [100]TestClient
	for i := 0; i < len(c); i++ {
		c[i] = NewDiscardingLoggedInClient("foo" + strconv.Itoa(i))
		c[i].SubscribeWithPresence("topic")
		defer c[i].Close()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c[i%len(c)].Unsubscribe("topic")
		c[i%len(c)].SubscribeWithPresence("topic")
	}
	b.StopTimer()
}
