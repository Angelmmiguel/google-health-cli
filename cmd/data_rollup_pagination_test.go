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
	"fmt"
	"strings"
	"testing"
)

func rollupPage(n int, next string) json.RawMessage {
	pts := make([]string, n)
	for i := range pts {
		pts[i] = fmt.Sprintf(`{"startTime":"t%d","steps":{"countSum":"%d"}}`, i, i)
	}
	body := fmt.Sprintf(`{"rollupDataPoints":[%s]`, strings.Join(pts, ","))
	if next != "" {
		body += fmt.Sprintf(`,"nextPageToken":%q`, next)
	}
	return json.RawMessage(body + "}")
}

// A single-page response (no nextPageToken) passes through untouched.
func TestCollectRollupPages_SinglePagePassthrough(t *testing.T) {
	first := rollupPage(2, "")
	calls := 0
	out, err := collectRollupPages(first, listPageCap, func(token string) (json.RawMessage, error) {
		calls++
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("fetched %d extra pages, want 0", calls)
	}
	if string(out) != string(first) {
		t.Errorf("single page should pass through unchanged")
	}
}

// Multiple pages are merged into one rollupDataPoints array with no token.
func TestCollectRollupPages_MergesAllPages(t *testing.T) {
	pages := map[string]json.RawMessage{
		"p1": rollupPage(2, "p2"),
		"p2": rollupPage(1, ""),
	}
	out, err := collectRollupPages(rollupPage(2, "p1"), listPageCap, func(token string) (json.RawMessage, error) {
		return pages[token], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var merged struct {
		RollupDataPoints []json.RawMessage `json:"rollupDataPoints"`
		NextPageToken    string            `json:"nextPageToken"`
	}
	if err := json.Unmarshal(out, &merged); err != nil {
		t.Fatal(err)
	}
	if len(merged.RollupDataPoints) != 5 {
		t.Errorf("got %d points, want 5 (2+2+1 merged)", len(merged.RollupDataPoints))
	}
	if merged.NextPageToken != "" {
		t.Errorf("got nextPageToken %q, want empty (exhausted)", merged.NextPageToken)
	}
}

// Hitting the page cap with the server still offering data must preserve the
// continuation token so the result is never misreported as complete.
func TestCollectRollupPages_CapSurfacesToken(t *testing.T) {
	out, err := collectRollupPages(rollupPage(1, "more"), 3, func(token string) (json.RawMessage, error) {
		return rollupPage(1, "more"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var merged struct {
		RollupDataPoints []json.RawMessage `json:"rollupDataPoints"`
		NextPageToken    string            `json:"nextPageToken"`
	}
	if err := json.Unmarshal(out, &merged); err != nil {
		t.Fatal(err)
	}
	if len(merged.RollupDataPoints) != 3 {
		t.Errorf("got %d points, want 3 (one per capped page)", len(merged.RollupDataPoints))
	}
	if merged.NextPageToken != "more" {
		t.Errorf("got nextPageToken %q, want \"more\"", merged.NextPageToken)
	}
}

// withPageToken re-marshals the original request body with pageToken set,
// preserving the range and window fields.
func TestWithPageToken(t *testing.T) {
	pinTZ(t, 0)
	body, err := buildRollupBody("2026-03-01", "2026-03-07", "86400s")
	if err != nil {
		t.Fatal(err)
	}
	b, err := withPageToken(body, "tok123")
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, b)
	if m["pageToken"] != "tok123" {
		t.Errorf("pageToken = %v, want tok123", m["pageToken"])
	}
	if m["windowSize"] != "86400s" {
		t.Errorf("windowSize lost on re-marshal: %v", m["windowSize"])
	}
	if _, ok := m["range"].(map[string]interface{}); !ok {
		t.Errorf("range lost on re-marshal")
	}
}
