package main

import (
	"os"
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
