package main

import (
	"os"
	"strings"
	"testing"
)

func TestAuthFromEnvironmentRequiresPasswordAndUnsetsIt(t *testing.T) {
	t.Setenv("PRAETOR_WEB_PASSWORD", "")
	if _, err := authFromEnvironment(); err == nil {
		t.Fatal("empty password was accepted")
	}

	t.Setenv("PRAETOR_WEB_PASSWORD", "test-only-secret")
	if _, err := authFromEnvironment(); err != nil {
		t.Fatalf("valid password: %v", err)
	}
	if _, present := os.LookupEnv("PRAETOR_WEB_PASSWORD"); present {
		t.Fatal("startup password remained in the process environment")
	}
}

func TestResolveTLSMode(t *testing.T) {
	tests := []struct {
		name         string
		certificate  string
		privateKey   string
		insecureHTTP bool
		want         tlsMode
		wantError    string
	}{
		{name: "automatic default", want: tlsModeAutomatic},
		{name: "explicit certificate", certificate: "server.crt", privateKey: "server.key", want: tlsModeExplicit},
		{name: "explicit insecure HTTP", insecureHTTP: true, want: tlsModeInsecureHTTP},
		{name: "missing private key", certificate: "server.crt", wantError: "provided together"},
		{name: "missing certificate", privateKey: "server.key", wantError: "provided together"},
		{name: "conflicting insecure HTTP", certificate: "server.crt", privateKey: "server.key", insecureHTTP: true, wantError: "cannot be combined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveTLSMode(test.certificate, test.privateKey, test.insecureHTTP)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want containing %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != test.want {
				t.Fatalf("mode = %v, want %v", got, test.want)
			}
		})
	}
}
