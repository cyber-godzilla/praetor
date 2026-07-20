package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
		ok   bool
	}{
		{"0.2.7", [3]int{0, 2, 7}, true},
		{"v0.2.7", [3]int{0, 2, 7}, true},
		{"v1.10.3", [3]int{1, 10, 3}, true},
		{"v0.3.0-rc1", [3]int{0, 3, 0}, true},
		{" v0.2.7 ", [3]int{0, 2, 7}, true},
		{"dev", [3]int{}, false},
		{"", [3]int{}, false},
		{"1.2", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parseSemver(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseSemver(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current [3]int
		want            bool
	}{
		{[3]int{0, 2, 8}, [3]int{0, 2, 7}, true},
		{[3]int{0, 3, 0}, [3]int{0, 2, 9}, true},
		{[3]int{1, 0, 0}, [3]int{0, 9, 9}, true},
		{[3]int{0, 2, 7}, [3]int{0, 2, 7}, false},
		{[3]int{0, 2, 6}, [3]int{0, 2, 7}, false},
	}
	for _, c := range cases {
		if got := newer(c.latest, c.current); got != c.want {
			t.Errorf("newer(%v, %v) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func checkAgainst(t *testing.T, body string, status int, current string) (Info, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return Check(ctx, srv.Client(), srv.URL, current)
}

func TestCheck_NewerAvailable(t *testing.T) {
	info, err := checkAgainst(t, `{"tag_name":"v0.3.0","html_url":"https://example.com/rel"}`, 200, "0.2.7")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Available || info.Latest != "0.3.0" || info.URL != "https://example.com/rel" || info.Current != "0.2.7" {
		t.Fatalf("info = %+v", info)
	}
}

func TestCheck_UpToDate(t *testing.T) {
	info, err := checkAgainst(t, `{"tag_name":"v0.2.7","html_url":"u"}`, 200, "0.2.7")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Fatalf("same version should not be available: %+v", info)
	}
}

func TestCheck_RemoteOlder(t *testing.T) {
	info, err := checkAgainst(t, `{"tag_name":"v0.2.5","html_url":"u"}`, 200, "0.2.7")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Fatalf("older remote should not be available: %+v", info)
	}
}

func TestCheck_DevBuildSkipsNetwork(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	info, err := Check(context.Background(), srv.Client(), srv.URL, "dev")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if called {
		t.Error("dev build must not hit the network")
	}
	if info.Available {
		t.Errorf("dev build should never report an update: %+v", info)
	}
}

func TestCheck_ErrorPaths(t *testing.T) {
	if _, err := checkAgainst(t, `{}`, 500, "0.2.7"); err == nil {
		t.Error("HTTP 500 should error")
	}
	if _, err := checkAgainst(t, `not json`, 200, "0.2.7"); err == nil {
		t.Error("malformed JSON should error")
	}
	if _, err := checkAgainst(t, `{"tag_name":"banana"}`, 200, "0.2.7"); err == nil {
		t.Error("unparseable tag should error")
	}
}
