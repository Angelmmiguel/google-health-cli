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
	"testing"
)

// decodeBody unmarshals a generated request body for assertions.
func decodeBody(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, body)
	}
	return m
}

// rollUp: --from X --to X must cover exactly the named local day — the
// closed-open range [X 00:00, X+1 00:00), not the empty range [X, X).
func TestBuildRollupBody_ToInclusiveOfNamedDay(t *testing.T) {
	pinTZ(t, 0)
	body, err := buildRollupBody("2026-03-07", "2026-03-07", "86400s")
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, body)
	rng := m["range"].(map[string]interface{})
	if got := rng["startTime"]; got != "2026-03-07T00:00:00Z" {
		t.Errorf("startTime = %v, want 2026-03-07T00:00:00Z", got)
	}
	if got := rng["endTime"]; got != "2026-03-08T00:00:00Z" {
		t.Errorf("endTime = %v, want 2026-03-08T00:00:00Z (inclusive of the named day)", got)
	}
	if got := m["windowSize"]; got != "86400s" {
		t.Errorf("windowSize = %v, want 86400s", got)
	}
}

// rollUp on a non-UTC host: the inclusive end is local midnight of the NEXT
// day, expressed in UTC.
func TestBuildRollupBody_ToInclusiveLocalDay(t *testing.T) {
	pinTZ(t, 2*3600) // +02:00
	body, err := buildRollupBody("2026-06-08", "2026-06-08", "3600s")
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, body)
	rng := m["range"].(map[string]interface{})
	if got := rng["startTime"]; got != "2026-06-07T22:00:00Z" {
		t.Errorf("startTime = %v, want 2026-06-07T22:00:00Z", got)
	}
	if got := rng["endTime"]; got != "2026-06-08T22:00:00Z" {
		t.Errorf("endTime = %v, want 2026-06-08T22:00:00Z", got)
	}
}

// An explicit RFC-3339 --to is a precise exclusive bound — never advanced.
func TestBuildRollupBody_ExplicitTimestampNotAdvanced(t *testing.T) {
	pinTZ(t, 0)
	body, err := buildRollupBody("2026-03-07T06:00:00Z", "2026-03-07T12:00:00Z", "3600s")
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, body)
	rng := m["range"].(map[string]interface{})
	if got := rng["endTime"]; got != "2026-03-07T12:00:00Z" {
		t.Errorf("endTime = %v, want 2026-03-07T12:00:00Z (precise bound passthrough)", got)
	}
}

// dailyRollUp: --from X --to X must cover exactly the named civil day —
// the closed-open civil range [X, X+1).
func TestBuildDailyRollupBody_ToInclusiveOfNamedDay(t *testing.T) {
	body, err := buildDailyRollupBody("2026-03-07", "2026-03-07", 1)
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, body)
	rng := m["range"].(map[string]interface{})
	start := rng["start"].(map[string]interface{})["date"].(map[string]interface{})
	end := rng["end"].(map[string]interface{})["date"].(map[string]interface{})
	if d := start["day"].(float64); d != 7 {
		t.Errorf("start day = %v, want 7", d)
	}
	if d := end["day"].(float64); d != 8 {
		t.Errorf("end day = %v, want 8 (exclusive bound includes the named day)", d)
	}
	if w := m["windowSizeDays"].(float64); w != 1 {
		t.Errorf("windowSizeDays = %v, want 1", w)
	}
}

// dailyRollUp month rollover at the end bound.
func TestBuildDailyRollupBody_MonthRollover(t *testing.T) {
	body, err := buildDailyRollupBody("2026-02-25", "2026-02-28", 1)
	if err != nil {
		t.Fatal(err)
	}
	m := decodeBody(t, body)
	end := m["range"].(map[string]interface{})["end"].(map[string]interface{})["date"].(map[string]interface{})
	if mo, d := end["month"].(float64), end["day"].(float64); mo != 3 || d != 1 {
		t.Errorf("end = %v-%v, want 3-1 (2026-03-01)", mo, d)
	}
}
