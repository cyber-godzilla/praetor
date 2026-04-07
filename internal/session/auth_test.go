package session

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComputeHash_MatchesMD5(t *testing.T) {
	username := "testuser"
	password := "testpass"
	secret := "abc123secret"

	got := ComputeHash(username, password, secret)
	expected := fmt.Sprintf("%x", md5.Sum([]byte(username+password+secret)))

	if got != expected {
		t.Errorf("ComputeHash(%q, %q, %q) = %q, want %q",
			username, password, secret, got, expected)
	}
}

func TestComputeHash_EmptyInputs(t *testing.T) {
	got := ComputeHash("", "", "")
	expected := fmt.Sprintf("%x", md5.Sum([]byte("")))
	if got != expected {
		t.Errorf("ComputeHash('','','') = %q, want %q", got, expected)
	}
}

func TestComputeHash_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := ComputeHash("alice", "pass1", "sec1")
	h2 := ComputeHash("bob", "pass2", "sec2")
	if h1 == h2 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestComputeHash_KnownValue(t *testing.T) {
	// Verify against a known MD5: md5("adminpassword123secret456") = specific hash
	got := ComputeHash("admin", "password123", "secret456")
	expected := fmt.Sprintf("%x", md5.Sum([]byte("adminpassword123secret456")))
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestBuildAuthMessages_Format(t *testing.T) {
	msgs := BuildAuthMessages("testuser", "1166239322", "mysecret")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	if msgs[0] != "USER testuser" {
		t.Errorf("expected 'USER testuser', got %q", msgs[0])
	}

	if msgs[1] != "SECRET mysecret" {
		t.Errorf("expected 'SECRET mysecret', got %q", msgs[1])
	}

	expectedHash := ComputeHash("testuser", "1166239322", "mysecret")
	if msgs[2] != "HASH "+expectedHash {
		t.Errorf("expected 'HASH %s', got %q", expectedHash, msgs[2])
	}
}

func TestBuildAuthMessages_HashConsistency(t *testing.T) {
	msgs1 := BuildAuthMessages("user", "pass", "secret")
	msgs2 := BuildAuthMessages("user", "pass", "secret")
	if msgs1[2] != msgs2[2] {
		t.Errorf("same inputs should produce same hash: %q vs %q", msgs1[2], msgs2[2])
	}
}

func TestBuildAuthMessagesIncludesSecret(t *testing.T) {
	msgs := BuildAuthMessages("player", "9999999", "challenge_token_abc")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (USER, SECRET, HASH), got %d", len(msgs))
	}

	if msgs[0] != "USER player" {
		t.Errorf("msg[0]: expected 'USER player', got %q", msgs[0])
	}
	if msgs[1] != "SECRET challenge_token_abc" {
		t.Errorf("msg[1]: expected 'SECRET challenge_token_abc', got %q", msgs[1])
	}
	if msgs[2][:5] != "HASH " {
		t.Errorf("msg[2]: expected HASH prefix, got %q", msgs[2])
	}
}

func TestHTTPLogin(t *testing.T) {
	// Simulate the two-step flow: GET sets biscuit cookie, POST with biscuit returns user/pass cookies.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Step 1: set the biscuit cookie
			http.SetCookie(w, &http.Cookie{Name: "biscuit", Value: "test", Path: "/"})
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login form</html>"))
			return
		}

		// Step 2: POST — verify biscuit cookie was sent
		biscuit, err := r.Cookie("biscuit")
		if err != nil || biscuit.Value != "test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>cookies not accepted</html>"))
			return
		}

		r.ParseForm()
		if r.FormValue("uname") != "testplayer" || r.FormValue("pwd") != "mypassword" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>bad credentials</html>"))
			return
		}

		http.SetCookie(w, &http.Cookie{Name: "user", Value: "testplayer", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "pass", Value: "1166239322", Path: "/"})
		http.Redirect(w, r, "/overview.php", http.StatusFound)
	}))
	defer srv.Close()

	userCookie, passCookie, err := HTTPLogin(srv.URL, "testplayer", "mypassword")
	if err != nil {
		t.Fatalf("HTTPLogin returned error: %v", err)
	}
	if userCookie != "testplayer" {
		t.Errorf("expected userCookie 'testplayer', got %q", userCookie)
	}
	if passCookie != "1166239322" {
		t.Errorf("expected passCookie '1166239322', got %q", passCookie)
	}
}

func TestHTTPLoginBadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "biscuit", Value: "test", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}
		// Bad credentials: return 200 with no user/pass cookies.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>Login failed</html>"))
	}))
	defer srv.Close()

	_, _, err := HTTPLogin(srv.URL, "baduser", "badpass")
	if err == nil {
		t.Fatal("expected error for bad credentials, got nil")
	}
}

func TestHTTPLoginServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "biscuit", Value: "test", Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := HTTPLogin(srv.URL, "user", "pass")
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}
