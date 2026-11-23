package main

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBindAddr(t *testing.T) {
	cases := []struct {
		raw         string
		defaultPort int
		want        string
		wantErr     bool
	}{
		{"127.0.0.1:7680", 7680, "127.0.0.1:7680", false},
		{"100.123.205.98:8080", 7680, "100.123.205.98:8080", false},
		{"100.123.205.98", 7680, "100.123.205.98:7680", false},
		{"localhost", 7680, "127.0.0.1:7680", false},
		{"localhost:9000", 7680, "127.0.0.1:9000", false},
		{":7680", 7680, "0.0.0.0:7680", false},
		{"::1", 7680, "[::1]:7680", false},
		{"[::1]:7680", 7680, "[::1]:7680", false},
		{"[fd7a:115c:a1e0::1]:9999", 7680, "[fd7a:115c:a1e0::1]:9999", false},
		{"", 7680, "", true},
		{"not-an-ip:7680", 7680, "", true},
		{"127.0.0.1:99999", 7680, "", true},
		{"127.0.0.1:0", 7680, "", true},
		{"127.0.0.1", 0, "", true},
	}
	for _, tc := range cases {
		got, err := normalizeBindAddr(tc.raw, tc.defaultPort)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeBindAddr(%q, %d) = %q, want error", tc.raw, tc.defaultPort, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeBindAddr(%q, %d) unexpected error: %v", tc.raw, tc.defaultPort, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeBindAddr(%q, %d) = %q, want %q", tc.raw, tc.defaultPort, got, tc.want)
		}
	}
}

func TestParseBindAddrs(t *testing.T) {
	got, err := parseBindAddrs([]string{"127.0.0.1, 100.123.205.98:7681", "127.0.0.1:7680"}, 7680)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"127.0.0.1:7680", "100.123.205.98:7681"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestHealthProbeAddr(t *testing.T) {
	if got := healthProbeAddr([]string{"100.123.205.98:7680", "127.0.0.1:7681"}, 7680); got != "127.0.0.1:7681" {
		t.Fatalf("got %q, want 127.0.0.1:7681", got)
	}
	if got := healthProbeAddr([]string{"0.0.0.0:9000"}, 7680); got != "127.0.0.1:9000" {
		t.Fatalf("got %q, want 127.0.0.1:9000", got)
	}
	if got := healthProbeAddr([]string{"100.123.205.98:7680"}, 7680); got != "100.123.205.98:7680" {
		t.Fatalf("got %q, want 100.123.205.98:7680", got)
	}
	if got := healthProbeAddr(nil, 7680); got != "127.0.0.1:7680" {
		t.Fatalf("got %q, want 127.0.0.1:7680", got)
	}
}

func TestSavedConfigBindRoundTripAndServeConfig(t *testing.T) {
	key := strings.Repeat("cd", 32)
	in := savedConfig{
		User:     "chiyo",
		Port:     7680,
		Binds:    []string{"127.0.0.1:7680", "100.123.205.98:7680"},
		Mux:      "tmux",
		Password: "secret",
	}
	data, err := formatUnixConfig(in, key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `TERMOTE_BIND="127.0.0.1:7680,100.123.205.98:7680"`) {
		t.Fatalf("TERMOTE_BIND missing from config:\n%s", data)
	}
	out, err := parseUnixConfig(data, func() string { return key })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out.Binds, in.Binds) {
		t.Fatalf("Binds round trip = %v, want %v", out.Binds, in.Binds)
	}

	sc := serveConfigFromSaved(*out, 7680)
	if !slices.Equal(sc.listenAddrs(), in.Binds) {
		t.Fatalf("listenAddrs = %v, want %v", sc.listenAddrs(), in.Binds)
	}
	if !sc.AllowLocalAddr {
		t.Fatal("expected AllowLocalAddr=true for non-loopback bind")
	}
}

func TestParseStartArgsBind(t *testing.T) {
	tc := newTestCLI(t, "linux")
	o, set, err := tc.parseStartArgs([]string{"--bind", "127.0.0.1:7680", "--bind", "100.123.205.98"})
	if err != nil {
		t.Fatal(err)
	}
	if !set["bind"] {
		t.Fatal("expected set[bind]=true")
	}
	mergeSaved(&o, set, nil)
	if err := tc.validateStart(&o); err != nil {
		t.Fatal(err)
	}
	want := []string{"127.0.0.1:7680", "100.123.205.98:7680"}
	if !slices.Equal(o.binds, want) {
		t.Fatalf("binds = %v, want %v", o.binds, want)
	}

	// --bind and --lan cannot be combined
	if _, _, err := tc.parseStartArgs([]string{"--bind", "127.0.0.1:7680", "--lan"}); err == nil {
		t.Fatal("expected error combining --bind and --lan")
	}

	// Explicit --lan clears previously saved binds on merge
	saved := &savedConfig{Binds: []string{"127.0.0.1:7680", "100.123.205.98:7680"}, Port: 7680}
	o2, set2, err := tc.parseStartArgs([]string{"--lan=false"})
	if err != nil {
		t.Fatal(err)
	}
	mergeSaved(&o2, set2, saved)
	if len(o2.binds) != 0 {
		t.Fatalf("expected --lan=false to clear saved binds, got %v", o2.binds)
	}

	// Omitting both --bind and --lan preserves saved binds
	o3, set3, err := tc.parseStartArgs([]string{"--allow-host", "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	mergeSaved(&o3, set3, saved)
	if !slices.Equal(o3.binds, saved.Binds) {
		t.Fatalf("expected saved binds preserved, got %v", o3.binds)
	}
}

func TestRunServerMultiListeners(t *testing.T) {
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ln1.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := serveConfig{NoAuth: true, MuxBackend: "tmux"}
	done := make(chan error, 1)
	go func() {
		done <- runServerMulti(ctx, cfg, &fakeMux{}, []net.Listener{ln1, ln2})
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	for _, ln := range []net.Listener{ln1, ln2} {
		var ok bool
		for range 20 {
			resp, err := client.Get("http://" + ln.Addr().String() + "/api/mux/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					ok = true
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
		}
		if !ok {
			t.Fatalf("listener %s did not answer healthy", ln.Addr())
		}
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runServerMulti shutdown error: %v", err)
	}
}
