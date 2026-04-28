package server_test

import (
	"testing"
	"time"
	"filehub/server"
)

func TestHubBroadcastReceived(t *testing.T) {
	hub := server.NewHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	msg := []byte(`{"folder_id":"abc","changes":[]}`)
	hub.Broadcast(msg)

	select {
	case got := <-ch:
		if string(got) != string(msg) {
			t.Fatalf("want %s, got %s", msg, got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout: message not received")
	}
}

func TestHubUnsubscribeClosesChannel(t *testing.T) {
	hub := server.NewHub()
	ch := hub.Subscribe()
	hub.Unsubscribe(ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel not closed")
	}
}

func TestHubBroadcastToMultiple(t *testing.T) {
	hub := server.NewHub()
	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)

	hub.Broadcast([]byte("hello"))

	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case got := <-ch:
			if string(got) != "hello" {
				t.Fatalf("want hello, got %s", got)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout")
		}
	}
}
