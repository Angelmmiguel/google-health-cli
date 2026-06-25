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
	token         string
	err           error
	calls         int
	invalidations int
}

func (f *fakeTokenSource) Token() (string, error) {
	f.calls++
	return f.token, f.err
}

func (f *fakeTokenSource) Invalidate() {
	f.invalidations++
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

// A server-side-stale access token can surface as a 403 "insufficient
// authentication scopes" rather than a 401 (the oauth2 library's ~10s
// expiry-skew window, observed live). The stale-scope variant must earn exactly
// one token refresh + retry, like a 401.
func TestDo_StaleScopeForbiddenRefreshesOnce(t *testing.T) {
	stubSleep(t)
	ts := &fakeTokenSource{token: "tok"}
	attempts := 0
	c := newTestClient(ts, func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: 403,
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"code":403,"message":"Request had insufficient authentication scopes.","status":"PERMISSION_DENIED"}}`)),
				Header: http.Header{},
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     http.Header{},
		}, nil
	})

	resp, err := c.Do(&Request{Method: "GET", Path: "/x"})
	if err != nil {
		t.Fatalf("expected success after refresh+retry, got %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ts.invalidations != 1 {
		t.Errorf("token invalidated %d times, want 1", ts.invalidations)
	}
	if attempts != 2 {
		t.Errorf("transport attempted %d times, want 2", attempts)
	}
}

// The refresh is one-shot across BOTH the 401 and 403 handlers: a 401 followed
// by a persistent stale-scope 403 must refresh exactly once, then surface the
// 403 (not loop). Guards the shared `refreshed` flag.
func TestDo_RefreshIsOneShotAcross401Then403(t *testing.T) {
	stubSleep(t)
	ts := &fakeTokenSource{token: "tok"}
	attempts := 0
	c := newTestClient(ts, func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`)), Header: http.Header{}}, nil
		}
		// After the one allowed refresh, the token is still stale-scoped.
		return &http.Response{
			StatusCode: 403,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"status":"PERMISSION_DENIED","message":"Request had insufficient authentication scopes."}}`)),
			Header:     http.Header{},
		}, nil
	})

	_, err := c.Do(&Request{Method: "GET", Path: "/x"})
	if err == nil {
		t.Fatal("expected the persistent 403 to surface as an error")
	}
	if ts.invalidations != 1 {
		t.Errorf("token invalidated %d times, want exactly 1 (one-shot across 401+403)", ts.invalidations)
	}
	if attempts != 2 {
		t.Errorf("transport attempted %d times, want 2 (401 → refresh → 403, no further retry)", attempts)
	}
}

// A policy-denial 403 (not the stale-scope variant) must NOT trigger a refresh
// or retry — it fails immediately as an API error.
func TestDo_PolicyForbiddenDoesNotRetry(t *testing.T) {
	stubSleep(t)
	ts := &fakeTokenSource{token: "tok"}
	attempts := 0
	c := newTestClient(ts, func(*http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{
			StatusCode: 403,
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":403,"message":"Health API has not been used in project 123 before or it is disabled.","status":"PERMISSION_DENIED"}}`)),
			Header: http.Header{},
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
	if ts.invalidations != 0 {
		t.Errorf("token invalidated %d times, want 0", ts.invalidations)
	}
	if attempts != 1 {
		t.Errorf("transport attempted %d times, want 1 (policy 403s never retry)", attempts)
	}
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
