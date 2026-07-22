package session

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// httpLoginTimeout bounds the whole login flow so a hung login server can't
// block connect forever. It is a package var so tests can shrink it.
var httpLoginTimeout = 15 * time.Second

// ComputeHash computes the MD5 hex digest of the concatenation of username,
// password, and secret. This is the hash format expected by the game server
// during authentication.
func ComputeHash(username, password, secret string) string {
	data := username + password + secret
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// BuildAuthMessages builds the SKOOT auth messages.
// passCookie is the token from the HTTP login, NOT the raw password.
func BuildAuthMessages(username, passCookie, secret string) []string {
	hash := ComputeHash(username, passCookie, secret)
	return []string{
		"USER " + username,
		"SECRET " + secret,
		"HASH " + hash,
	}
}

// HTTPLogin performs the HTTP login to get session cookies.
// Returns the user and pass cookie values.
//
// The server requires a two-step flow:
//  1. GET the login page to receive a "biscuit=test" cookie
//  2. POST credentials with that cookie to prove cookies are enabled
//
// The POST typically returns a 302 redirect with Set-Cookie: user=... and
// pass=... headers. The cookie jar captures these across the redirect.
func HTTPLogin(loginURL, username, password string) (userCookie, passCookie string, err error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// One deadline for the WHOLE two-step flow (not per-request): threading a
	// single context through both requests means a server that trickles just
	// under a per-request budget on both can't stretch login to ~2x the timeout.
	ctx, cancel := context.WithTimeout(context.Background(), httpLoginTimeout)
	defer cancel()

	// Step 1: GET the login page to pick up the biscuit cookie. Drain and close
	// the body so the persistent connection is reused instead of leaked.
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("HTTP login: building request: %w", err)
	}
	getResp, err := client.Do(getReq)
	if err != nil {
		return "", "", fmt.Errorf("HTTP login: fetching login page: %w", err)
	}
	_, _ = io.Copy(io.Discard, getResp.Body)
	getResp.Body.Close()

	// Step 2: POST credentials. The jar automatically sends the biscuit cookie.
	form := url.Values{}
	form.Set("submit", "true")
	form.Set("phrase", "")
	form.Set("uname", username)
	form.Set("pwd", password)

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("HTTP login: building request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(postReq)
	if err != nil {
		return "", "", fmt.Errorf("HTTP login: posting credentials: %w", err)
	}
	defer resp.Body.Close()

	// Extract user and pass cookies from the jar.
	// The server sets these on the 302 redirect response, and the jar
	// captures them as the client follows the redirect to overview.php.
	parsed, _ := url.Parse(loginURL)
	for _, cookie := range jar.Cookies(parsed) {
		switch cookie.Name {
		case "user":
			userCookie = cookie.Value
		case "pass":
			passCookie = cookie.Value
		}
	}

	if userCookie == "" || passCookie == "" {
		return "", "", fmt.Errorf("HTTP login: authentication failed (invalid credentials or server error)")
	}

	return userCookie, passCookie, nil
}
