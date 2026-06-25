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

package output

import (
	"encoding/json"
	"testing"
)

// A rollup response carrying a nextPageToken must surface that token in the
// simplified output — dropping it silently misreports the result as complete.
func TestSimplifyResponse_RollupKeepsNextPageToken(t *testing.T) {
	raw := json.RawMessage(`{
		"rollupDataPoints": [
			{"startTime": "2026-03-01T00:00:00Z", "endTime": "2026-03-02T00:00:00Z", "steps": {"countSum": "9037"}}
		],
		"nextPageToken": "tok-abc"
	}`)

	out := SimplifyResponse(raw, "steps", false)

	var obj struct {
		RollupDataPoints []map[string]interface{} `json:"rollupDataPoints"`
		NextPageToken    string                   `json:"nextPageToken"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		t.Fatalf("simplified output with token should be an object: %v\n%s", err, out)
	}
	if obj.NextPageToken != "tok-abc" {
		t.Errorf("nextPageToken = %q, want tok-abc", obj.NextPageToken)
	}
	if len(obj.RollupDataPoints) != 1 {
		t.Errorf("got %d points, want 1", len(obj.RollupDataPoints))
	}
}

// Without a token, the rollup output stays a bare array (backward compat).
func TestSimplifyResponse_RollupNoTokenStaysArray(t *testing.T) {
	raw := json.RawMessage(`{
		"rollupDataPoints": [
			{"startTime": "2026-03-01T00:00:00Z", "endTime": "2026-03-02T00:00:00Z", "steps": {"countSum": "9037"}}
		]
	}`)

	out := SimplifyResponse(raw, "steps", false)

	var arr []map[string]interface{}
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatalf("tokenless rollup output should remain an array: %v\n%s", err, out)
	}
	if len(arr) != 1 {
		t.Errorf("got %d points, want 1", len(arr))
	}
}

// A 1-day dailyRollUp bucket keeps the single "date" field (backward compat).
func TestSimplifyResponse_SingleDayBucketKeepsDate(t *testing.T) {
	raw := json.RawMessage(`{
		"rollupDataPoints": [
			{
				"civilStartTime": {"date": {"year": 2026, "month": 3, "day": 1}},
				"civilEndTime": {"date": {"year": 2026, "month": 3, "day": 2}},
				"steps": {"countSum": "9037"}
			}
		]
	}`)

	out := SimplifyResponse(raw, "steps", false)
	var arr []map[string]interface{}
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 1 {
		t.Fatalf("got %d points, want 1", len(arr))
	}
	if arr[0]["date"] != "2026-03-01" {
		t.Errorf("date = %v, want 2026-03-01", arr[0]["date"])
	}
	if _, ok := arr[0]["startDate"]; ok {
		t.Errorf("1-day bucket should not emit startDate")
	}
}

// A multi-day bucket (--window-days > 1) must not be mislabeled as a single
// day: emit startDate and endDate (inclusive) instead of "date".
func TestSimplifyResponse_MultiDayBucketEmitsStartEndDates(t *testing.T) {
	raw := json.RawMessage(`{
		"rollupDataPoints": [
			{
				"civilStartTime": {"date": {"year": 2026, "month": 3, "day": 1}},
				"civilEndTime": {"date": {"year": 2026, "month": 3, "day": 8}},
				"steps": {"countSum": "63000"}
			}
		]
	}`)

	out := SimplifyResponse(raw, "steps", false)
	var arr []map[string]interface{}
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 1 {
		t.Fatalf("got %d points, want 1", len(arr))
	}
	if _, ok := arr[0]["date"]; ok {
		t.Errorf("multi-day bucket must not emit a single date: %v", arr[0])
	}
	if arr[0]["startDate"] != "2026-03-01" {
		t.Errorf("startDate = %v, want 2026-03-01", arr[0]["startDate"])
	}
	// civilEndTime is the closed-open exclusive bound; the inclusive last
	// day of a Mar 1 - Mar 8 bucket is Mar 7.
	if arr[0]["endDate"] != "2026-03-07" {
		t.Errorf("endDate = %v, want 2026-03-07 (inclusive)", arr[0]["endDate"])
	}
}

// Valueless multi-day buckets are still dropped (presence semantics).
func TestSimplifyResponse_ValuelessMultiDayBucketDropped(t *testing.T) {
	raw := json.RawMessage(`{
		"rollupDataPoints": [
			{
				"civilStartTime": {"date": {"year": 2026, "month": 3, "day": 1}},
				"civilEndTime": {"date": {"year": 2026, "month": 3, "day": 8}}
			}
		]
	}`)

	out := SimplifyResponse(raw, "steps", false)
	var arr []map[string]interface{}
	if err := json.Unmarshal(out, &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 0 {
		t.Errorf("valueless bucket should be dropped, got %v", arr)
	}
}
