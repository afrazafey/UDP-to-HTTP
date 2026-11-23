package stream

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentCreateOnlyImportsCannotOverwrite(t *testing.T) {
	m, err := NewManager(t.Context(), testOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	a := testChannel(t)
	b := a
	for b.SourcePort == a.SourcePort {
		b.SourcePort = freeUDP(t)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, channel := range []Channel{a, b} {
		wg.Add(1)
		go func(c Channel) {
			defer wg.Done()
			<-start
			results <- m.ImportChannels(t.Context(), []Channel{c}, false)
		}(channel)
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 || len(m.List()) != 1 {
		t.Fatal("concurrent import intent was not preserved", success, conflict)
	}
}

func TestBulkDeleteCancelsAllReceiversBeforeJoining(t *testing.T) {
	m, err := NewManager(t.Context(), testOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ids := []string{}
	ports := map[int]bool{}
	for i := 0; i < 20; i++ {
		c := testChannel(t)
		for ports[c.SourcePort] {
			c.SourcePort = freeUDP(t)
		}
		ports[c.SourcePort] = true
		c.ID = fmt.Sprintf("bulk-%d", i)
		c.Startup = "always"
		if err = m.Put(t.Context(), c, true); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	eventually(t, func() bool {
		bound := 0
		for _, v := range m.List() {
			if v.SocketBound {
				bound++
			}
		}
		return bound == len(ids)
	})
	started := time.Now()
	if err = m.BulkChannels(t.Context(), ids, "delete"); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 4*time.Second {
		t.Fatal("bulk cancellation serialized read deadline waits")
	}
	if m.ReceiverCount() != 0 || len(m.List()) != 0 {
		t.Fatal("bulk receivers leaked")
	}
}

func TestConcurrentBulkToggleDoesNotLoseChannelEdits(t *testing.T) {
	m, c := testManager(t)
	for i := 0; i < 20; i++ {
		changed := c
		changed.Name = fmt.Sprintf("Updated %d", i)
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; results <- m.Put(t.Context(), changed, false) }()
		go func() { defer wg.Done(); <-start; results <- m.BulkChannels(t.Context(), []string{c.ID}, "disable") }()
		close(start)
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		current, err := m.Get(c.ID)
		if err != nil || current.Channel.Name != changed.Name {
			t.Fatal("bulk toggle overwrote metadata", current.Channel, err)
		}
	}
}
