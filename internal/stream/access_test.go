package stream

import (
	"errors"
	"net/netip"
	"sync"
	"testing"

	"github.com/udphttp-proxy/udphttp-proxy/internal/netpolicy"
)

func TestLiveAccessChangesKeepAuthorizedClients(t *testing.T) {
	m, c := testManager(t)
	other := testChannel(t)
	other.ID = "other"
	if err := m.Put(t.Context(), other, true); err != nil {
		t.Fatal(err)
	}
	compile := func(list string) netpolicy.Prefixes {
		t.Helper()
		clients, err := netpolicy.Compile("clients", netpolicy.List{list}, netpolicy.Client, false)
		if err != nil {
			t.Fatal(err)
		}
		return clients
	}
	sources := m.Sources()
	allowBoth := compile("10.14.0.0/24")
	jellyfinOnly := compile("10.14.0.5")
	wanted := netip.MustParseAddr("::ffff:10.14.0.5")
	excluded := netip.MustParseAddr("10.14.0.6")
	m.UpdateAccess(sources, allowBoth)
	keep, err := m.SubscribePlayback(c.ID, wanted)
	if err != nil {
		t.Fatal(err)
	}
	defer keep.Close()
	drop, err := m.SubscribePlayback(other.ID, excluded)
	if err != nil {
		t.Fatal(err)
	}
	defer drop.Close()
	m.UpdateAccess(sources, jellyfinOnly)
	select {
	case <-drop.Done():
	default:
		t.Fatal("unauthorized client was not disconnected")
	}
	select {
	case <-keep.Done():
		t.Fatal("authorized client was disconnected")
	default:
	}
	var concurrent sync.WaitGroup
	for range 8 {
		concurrent.Go(func() {
			m.UpdateAccess(sources, jellyfinOnly)
			if !m.ClientAllowed(wanted) {
				t.Error("authorized peer removed by concurrent update")
			}
		})
	}
	concurrent.Wait()
	m.UpdateAccess(netpolicy.Sources{}, jellyfinOnly)
	select {
	case <-keep.Done():
	default:
		t.Fatal("source tightening did not close the receiver's client")
	}
	if _, err := m.SubscribePlayback(c.ID, wanted); !errors.Is(err, ErrSourceDenied) {
		t.Fatalf("blocked source subscription: %v", err)
	}
	if state, err := m.Get(c.ID); err != nil || state.State != "error" || state.SocketBound {
		t.Fatalf("receiver was not stopped: %+v %v", state, err)
	}
	m.UpdateAccess(sources, jellyfinOnly)
	restored, err := m.SubscribePlayback(c.ID, wanted)
	if err != nil {
		t.Fatal(err)
	}
	restored.Close()
}
