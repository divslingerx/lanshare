package discovery

import (
	"context"
	"log"
	"strings"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_filehub._tcp"
const domain = "local."

type Peer struct {
	Hostname    string
	DisplayName string
	Addr        string
	Port        int
}

// Advertiser announces this device on the LAN.
type Advertiser struct {
	server *zeroconf.Server
}

func NewAdvertiser(hostname, displayName string, port int) (*Advertiser, error) {
	txt := []string{
		"hostname=" + hostname,
		"v=1",
	}
	srv, err := zeroconf.Register(displayName, serviceType, domain, port, txt, nil)
	if err != nil {
		return nil, err
	}
	return &Advertiser{server: srv}, nil
}

func (a *Advertiser) Stop() {
	a.server.Shutdown()
}

// Browser discovers filehub peers on the LAN.
// onLost is reserved for future TTL-based expiry; grandcat/zeroconf does not emit removal events.
type Browser struct {
	cancel context.CancelFunc
}

func NewBrowser(onFound func(Peer), onLost func(hostname string)) *Browser {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Browser{cancel: cancel}
	go b.run(ctx, onFound, onLost)
	return b
}

func (b *Browser) Stop() { b.cancel() }

func (b *Browser) run(ctx context.Context, onFound func(Peer), onLost func(string)) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Printf("mdns: resolver error: %v", err)
		return
	}
	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		if err := resolver.Browse(ctx, serviceType, domain, entries); err != nil {
			log.Printf("mdns: browse error: %v", err)
		}
	}()

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				return
			}
			if entry == nil {
				continue
			}
			p := entryToPeer(entry)
			if p.Addr == "" {
				continue
			}
			onFound(p)
		case <-ctx.Done():
			return
		}
	}
}

func entryToPeer(e *zeroconf.ServiceEntry) Peer {
	hostname := e.HostName
	displayName := e.Instance
	for _, txt := range e.Text {
		if strings.HasPrefix(txt, "hostname=") {
			hostname = strings.TrimPrefix(txt, "hostname=")
		}
	}
	var addr string
	if len(e.AddrIPv4) > 0 {
		addr = e.AddrIPv4[0].String()
	} else if len(e.AddrIPv6) > 0 {
		addr = e.AddrIPv6[0].String()
	}
	return Peer{
		Hostname:    hostname,
		DisplayName: displayName,
		Addr:        addr,
		Port:        e.Port,
	}
}
