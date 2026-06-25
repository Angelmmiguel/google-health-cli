// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeTokenSource struct {
	token string
	err   error
	calls int
}

func (f *fakeTokenSource) Token() (string, error) {
	f.calls++
	return f.token, f.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestClient(ts *fakeTokenSource, rt roundTripFunc) *Client {
	return &Client{
		httpClient:  &http.Client{Transport: rt},
		tokenSource: ts,
	}
}

func stubSleep(t *testing.T) {
	t.Helper()
	orig := sleep
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = orig })
}

// A deterministic auth failure (token acquisition) must fail fast with the
// documented exit code 2 — no retries, no backoff.
func TestDo_AuthErrorExitsTwoWithoutRetry(t *testing.T) {
	stubSleep(t)
	ts := &fakeTokenSource{err: errors.New("no stored credentials")}
	c := newTestClient(ts, func(*http.Request) (*http.Response, error) {
		t.Fatal("transport should never be reached on auth failure")
		return nil, nil
	})

	_, err := c.Do(&Request{Method: "GET", Path: "/x"})
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("got %T (%v), want *CLIError", err, err)
	}
	if cliErr.Code != ExitAuthError {
		t.Errorf("exit code = %d, want %d (auth)", cliErr.Code, ExitAuthError)
	}
	if cliErr.Type != "auth" {
		t.Errorf("type = %q, want auth", cliErr.Type)
	}
	if !strings.Contains(cliErr.Hint, "ghealth auth login") {
		t.Errorf("hint = %q, want a 'ghealth auth login' pointer", cliErr.Hint)
	}
	if ts.calls != 1 {
		t.Errorf("token source called %d times, want 1 (no retry on deterministic auth failure)", ts.calls)
	}
}

// A transport failure that survives all retries is a network error (exit 4),
// not the generic API exit 1.
func TestDo_TransportErrorExitsFourAfterRetries(t *testing.T) {
	stubSleep(t)
	attempts := 0
	c := newTestClient(&fakeTokenSource{token: "tok"}, func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, errors.New("dial tcp: connection refused")
	})

	_, err := c.Do(&Request{Method: "GET", Path: "/x"})
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("got %T (%v), want *CLIError", err, err)
	}
	if cliErr.Code != ExitNetworkError {
		t.Errorf("exit code = %d, want %d (network)", cliErr.Code, ExitNetworkError)
	}
	if attempts != MaxRetries+1 {
		t.Errorf("transport attempted %d times, want %d (network errors retry)", attempts, MaxRetries+1)
	}
}

// 5xx responses keep their retry-then-API-error classification (exit 1).
func TestDo_ServerErrorRetainsAPIClassification(t *testing.T) {
	stubSleep(t)
	attempts := 0
	c := newTestClient(&fakeTokenSource{token: "tok"}, func(*http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":503,"message":"backend unavailable"}}`)),
			Header:     http.Header{},
		}, nil
	})

	_, err := c.Do(&Request{Method: "GET", Path: "/x"})
	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("got %T (%v), want *CLIError", err, err)
	}
	if cliErr.Code != ExitAPIError {
		t.Errorf("exit code = %d, want %d (api)", cliErr.Code, ExitAPIError)
	}
	if cliErr.Status != 503 {
		t.Errorf("status = %d, want 503", cliErr.Status)
	}
	if attempts != MaxRetries+1 {
		t.Errorf("attempted %d times, want %d (5xx retries unchanged)", attempts, MaxRetries+1)
	}
}

// API messages about over-long rollup ranges get a hint naming the caps
// (backstop for --json bodies that bypass local range validation).
func TestHintForAPIError_RollupRangeCap(t *testing.T) {
	hint := hintForAPIError(400, "Requested range exceeds the maximum allowed for this data type")
	if !strings.Contains(hint, "14 days") || !strings.Contains(hint, "90 days") {
		t.Errorf("hint = %q, want the 14/90-day caps named", hint)
	}
	if hintForAPIError(400, "invalid argument") != "" {
		t.Errorf("unrelated 400 should not get the range hint")
	}
}
