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
	"net/http"
	"testing"
	"time"
)

func hdr(v string) http.Header {
	h := http.Header{}
	if v != "" {
		h.Set("Retry-After", v)
	}
	return h
}

func TestParseRetryAfter_DeltaSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0}, // absent
		{"120", 120 * time.Second},
		{" 30 ", 30 * time.Second}, // surrounding whitespace
		{"0", 0},                   // non-positive
		{"-5", 0},                  // negative
		{"abc", 0},                 // unparseable
	}
	for _, c := range cases {
		if got := parseRetryAfter(hdr(c.in)); got != c.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseRetryAfter_HTTPDateFuture(t *testing.T) {
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	got := parseRetryAfter(hdr(future))
	// Allow for clock/second-truncation slack.
	if got <= 0 || got > 31*time.Second {
		t.Errorf("parseRetryAfter(future date) = %v, want ~30s", got)
	}
}

func TestParseRetryAfter_HTTPDatePast(t *testing.T) {
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(hdr(past)); got != 0 {
		t.Errorf("parseRetryAfter(past date) = %v, want 0", got)
	}
}
