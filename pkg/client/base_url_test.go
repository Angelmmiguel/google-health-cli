package client

import (
	"os"
	"testing"
)

func TestResolveBaseURLDefault(t *testing.T) {
	os.Unsetenv("GHEALTH_BASE_URL")
	if got := resolveBaseURL(); got != "https://health.googleapis.com/v4" {
		t.Fatalf("default = %q", got)
	}
}

func TestResolveBaseURLOverride(t *testing.T) {
	t.Setenv("GHEALTH_BASE_URL", "http://127.0.0.1:8787/v4")
	if got := resolveBaseURL(); got != "http://127.0.0.1:8787/v4" {
		t.Fatalf("override = %q", got)
	}
}
