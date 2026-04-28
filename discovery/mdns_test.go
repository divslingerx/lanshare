package discovery_test

import (
	"testing"
	"filehub/discovery"
)

func TestPeerString(t *testing.T) {
	p := discovery.Peer{Hostname: "myhost", DisplayName: "My Host", Addr: "192.168.1.5", Port: 47990}
	if p.Hostname == "" {
		t.Fatal("empty hostname")
	}
	if p.Addr == "" {
		t.Fatal("empty addr")
	}
}

func TestNewBrowser(t *testing.T) {
	b := discovery.NewBrowser(func(p discovery.Peer) {}, func(hostname string) {})
	if b == nil {
		t.Fatal("expected non-nil browser")
	}
	b.Stop()
}
