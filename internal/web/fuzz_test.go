package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzDecodeJSON(f *testing.F) {
	f.Add(`{"input":"look"}`)
	f.Add(`{"input":""}`)
	f.Add(`{"unknown":true}`)
	f.Add(strings.Repeat("x", maxJSONBody+1))
	f.Fuzz(func(t *testing.T, body string) {
		req := httptest.NewRequest("POST", "http://praetor.test/api/v1/commands", strings.NewReader(body))
		var value struct {
			Input string `json:"input"`
		}
		_ = decodeJSON(req, &value)
	})
}

func FuzzSameOrigin(f *testing.F) {
	f.Add("praetor.test:8787", "http://praetor.test:8787", false)
	f.Add("praetor.test:8787", "https://praetor.test:8787", true)
	f.Add("praetor.test", "http://attacker.test", false)
	f.Fuzz(func(t *testing.T, host, origin string, tls bool) {
		scheme := "http"
		if tls {
			scheme = "https"
		}
		req := httptest.NewRequest("POST", scheme+"://praetor.test/api/v1/auth/login", nil)
		req.Host = host
		req.Header.Set("Origin", origin)
		_ = sameOrigin(req)
	})
}
