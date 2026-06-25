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

package cmd

import (
	"encoding/json"
	"errors"
	"testing"
)

// fakePages builds a fetchPage func that serves the given pages of point-counts.
// Page i returns pages[i] points and a token "p{i+1}" unless it is the last
// page, which returns "". It ignores `want` (real fetchPage sizes the request to
// want; collectPages' accumulation logic is independent of page sizing).
func fakePages(pages []int) (func(string, int) ([]json.RawMessage, string, error), *int) {
	calls := 0
	tokens := map[string]int{"": 0}
	for i := range pages {
		tokens[tokenFor(i+1)] = i + 1
	}
	fn := func(token string, want int) ([]json.RawMessage, string, error) {
		calls++
		idx := tokens[token]
		if idx >= len(pages) {
			return nil, "", nil
		}
		pts := make([]json.RawMessage, pages[idx])
		for j := range pts {
			pts[j] = json.RawMessage("{}")
		}
		next := ""
		if idx+1 < len(pages) {
			next = tokenFor(idx + 1)
		}
		return pts, next, nil
	}
	return fn, &calls
}

func tokenFor(i int) string {
	return "p" + string(rune('0'+i))
}

func TestCollectPages_ExhaustsBeforeLimit(t *testing.T) {
	// Three pages, server runs out before the limit is reached.
	fn, calls := fakePages([]int{2, 2, 1})
	pts, token, err := collectPages(100, listPageCap, "", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pts) != 5 {
		t.Errorf("got %d points, want 5", len(pts))
	}
	if token != "" {
		t.Errorf("got remainingToken %q, want empty (data exhausted)", token)
	}
	if *calls != 3 {
		t.Errorf("fetched %d pages, want 3", *calls)
	}
}

func TestCollectPages_StopsAtLimitAndSurfacesToken(t *testing.T) {
	// Plenty of data; we stop once the limit is met, but the continuation
	// token MUST be surfaced so the caller can tell the result was cut by
	// --limit rather than exhausted (G9: silent truncation looked complete).
	fn, calls := fakePages([]int{2, 2, 2, 2, 2})
	pts, token, err := collectPages(5, listPageCap, "", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pts) < 5 {
		t.Errorf("got %d points, want >= 5", len(pts))
	}
	if token != "p3" {
		t.Errorf("got remainingToken %q, want \"p3\" (limit truncation must be surfaced)", token)
	}
	if *calls != 3 { // 2+2+2 = 6 >= 5 after the third page
		t.Errorf("fetched %d pages, want 3", *calls)
	}
}

func TestCollectPages_ExactLimitNoMoreData(t *testing.T) {
	// The limit is met exactly as the server runs out: a complete result,
	// no token, no truncation signal.
	fn, _ := fakePages([]int{3, 2})
	pts, token, err := collectPages(5, listPageCap, "", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pts) != 5 {
		t.Errorf("got %d points, want 5", len(pts))
	}
	if token != "" {
		t.Errorf("got remainingToken %q, want empty (data exhausted at limit)", token)
	}
}

// collectPages must size each request to the rows still wanted, so the final
// page lands on the --limit boundary and its token resumes losslessly.
func TestCollectPages_PassesShrinkingWant(t *testing.T) {
	var wants []int
	fn := func(token string, want int) ([]json.RawMessage, string, error) {
		wants = append(wants, want)
		// Honor the request: return exactly `want` rows (within a 2-row cap, to
		// model a per-type page cap) and always offer more.
		n := want
		if n > 2 {
			n = 2
		}
		pts := make([]json.RawMessage, n)
		for j := range pts {
			pts[j] = json.RawMessage("{}")
		}
		return pts, "more", nil
	}
	pts, token, err := collectPages(5, listPageCap, "", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pts) != 5 {
		t.Errorf("got %d points, want exactly 5 (no overshoot past --limit)", len(pts))
	}
	if token != "more" {
		t.Errorf("got token %q, want \"more\"", token)
	}
	// want must shrink toward the limit: 5, 3, 1 (2-row cap each page).
	expected := []int{5, 3, 1}
	if len(wants) != len(expected) {
		t.Fatalf("requested %v, want %v", wants, expected)
	}
	for i := range expected {
		if wants[i] != expected[i] {
			t.Errorf("page %d requested want=%d, expected %d (got %v)", i, wants[i], expected[i], wants)
		}
	}
}

// A caller-supplied --page-token must seed the first fetch (resume).
func TestCollectPages_StartTokenSeedsFirstFetch(t *testing.T) {
	var firstToken string
	seen := false
	fn := func(token string, want int) ([]json.RawMessage, string, error) {
		if !seen {
			firstToken = token
			seen = true
		}
		return nil, "", nil
	}
	if _, _, err := collectPages(10, listPageCap, "RESUME", fn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if firstToken != "RESUME" {
		t.Errorf("first fetch used token %q, want \"RESUME\"", firstToken)
	}
}

func TestLimitTruncated(t *testing.T) {
	cases := []struct {
		fetched, limit int
		token          string
		want           bool
		desc           string
	}{
		{6, 5, "p3", true, "mid-page slice with server token"},
		{6, 5, "", true, "mid-page slice, server exhausted (extra points dropped)"},
		{5, 5, "p3", true, "exact limit with server token"},
		{5, 5, "", false, "exact limit, exhausted — complete"},
		{3, 5, "", false, "under limit, exhausted — complete"},
		{3, 5, "cap", false, "under limit with token = page-cap stop, not --limit"},
	}
	for _, c := range cases {
		if got := limitTruncated(c.fetched, c.limit, c.token); got != c.want {
			t.Errorf("%s: limitTruncated(%d, %d, %q) = %v, want %v",
				c.desc, c.fetched, c.limit, c.token, got, c.want)
		}
	}
}

func TestCollectPages_TruncatedByCapSurfacesToken(t *testing.T) {
	// Server always returns one point and a token, never ending. With a small
	// cap and a large limit we must stop AND surface the continuation token so
	// the caller knows the result is incomplete.
	cap := 4
	fn := func(token string, want int) ([]json.RawMessage, string, error) {
		return []json.RawMessage{json.RawMessage("{}")}, "more", nil
	}
	pts, token, err := collectPages(1000, cap, "", fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pts) != cap {
		t.Errorf("got %d points, want %d (one per capped page)", len(pts), cap)
	}
	if token != "more" {
		t.Errorf("got remainingToken %q, want \"more\" (truncation must be surfaced)", token)
	}
}

func TestCollectPages_ErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	fn := func(token string, want int) ([]json.RawMessage, string, error) {
		return nil, "", sentinel
	}
	_, _, err := collectPages(10, listPageCap, "", fn)
	if !errors.Is(err, sentinel) {
		t.Errorf("got err %v, want sentinel propagated", err)
	}
}
