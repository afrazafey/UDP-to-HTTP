package cli

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
	"github.com/udphttp-proxy/udphttp-proxy/internal/storage"
)

func tcpPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
func TestStagedPortFailureRestoresPlaybackAndKeepsManagement(t *testing.T) {
	base := config.Default()
	base.Management.Port = tcpPort(t)
	base.Network.HTTPPort = tcpPort(t)
	for base.Network.HTTPPort == base.Management.Port {
		base.Network.HTTPPort = tcpPort(t)
	}
	base.Network.SecondaryHTTPPort = 0
	base.Management.ShutdownTimeout = config.Duration(time.Second)
	base.Storage.Path = filepath.Join(t.TempDir(), "state.db")
	blocker, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	candidate := operations.FromConfig(base)
	candidate.Network.HTTPPort = blocker.Addr().(*net.TCPAddr).Port
	s, err := storage.Open(context.Background(), base.Storage.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.InitServerSettings(context.Background(), operations.FromConfig(base)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteServerSettings(context.Background(), 0, candidate); err != nil {
		t.Fatal(err)
	}
	s.Close()
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, []string{"--config", path}, io.Discard, func(string) string { return "" }) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve shutdown: %v", err)
			}
		case <-time.After(4 * time.Second):
			t.Error("serve did not shut down")
		}
	}()
	// Own the connection pool so test requests cannot leave idle or speculative
	// connections behind when the server's one-second shutdown deadline starts.
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 500 * time.Millisecond}
	url := "http://127.0.0.1:" + strconv.Itoa(base.Management.Port)
	deadline := time.Now().Add(4 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		response, e := client.Get(url + "/health")
		if e == nil {
			ready = response.StatusCode == 200
			_, readErr := io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if ready {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("management unavailable after failed staged bind")
	}
	response, err := client.Get(url + "/api/v1/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var view struct {
		Active    operations.Settings `json:"active"`
		Settings  operations.Settings `json:"settings"`
		LastError string              `json:"last_error"`
	}
	if err = json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
	if view.Active.Network.HTTPPort != base.Network.HTTPPort || view.Settings.Network.HTTPPort != base.Network.HTTPPort || view.LastError == "" {
		t.Fatalf("rollback failed: %+v", view)
	}
	l, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(base.Network.HTTPPort)), time.Second)
	if err != nil {
		t.Fatal("accepted playback listener not restored", err)
	}
	l.Close()
}
func TestConfigurationPathFlags(t *testing.T) {
	for _, args := range [][]string{{"--config", "config.yaml"}, {"-config", "config.yaml"}, {"--config=config.yaml"}, {"-config=config.yaml"}} {
		got := configurationPath(args, nil)
		if !filepath.IsAbs(got) || filepath.Base(got) != "config.yaml" {
			t.Fatal(got)
		}
	}
}
