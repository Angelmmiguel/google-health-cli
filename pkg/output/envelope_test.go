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

// EnsureEnvelope guarantees one stable JSON shape for every tabular command:
// {"dataPoints": [...], "_hints"?: [...], "nextPageToken"?: "..."}. Before this,
// the row container varied (dataPoints | data | rollupDataPoints | bare array |
// absent) depending on the command and whether a hint/token fired, so an agent
// doing json.load(out)["dataPoints"] worked for list and crashed on rollups.
func decode(t *testing.T, data json.RawMessage) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("result is not a JSON object: %v\n%s", err, data)
	}
	return m
}

func TestEnsureEnvelope_BareArray(t *testing.T) {
	out := EnsureEnvelope(json.RawMessage(`[{"date":"2026-06-16","countSum":"1996"}]`))
	m := decode(t, out)
	rows, ok := m["dataPoints"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("bare array must become {dataPoints:[...]}, got %s", out)
	}
}

func TestEnsureEnvelope_RollupDataPointsAndToken(t *testing.T) {
	out := EnsureEnvelope(json.RawMessage(`{"rollupDataPoints":[{"start":"t"}],"nextPageToken":"ABC"}`))
	m := decode(t, out)
	if _, ok := m["rollupDataPoints"]; ok {
		t.Errorf("rollupDataPoints must be renamed to dataPoints: %s", out)
	}
	if _, ok := m["dataPoints"].([]interface{}); !ok {
		t.Errorf("expected dataPoints array: %s", out)
	}
	if m["nextPageToken"] != "ABC" {
		t.Errorf("nextPageToken must survive: %s", out)
	}
}

func TestEnsureEnvelope_DataKeyAndHints(t *testing.T) {
	out := EnsureEnvelope(json.RawMessage(`{"data":[{"countSum":"5"}],"_hints":["h"]}`))
	m := decode(t, out)
	if _, ok := m["data"]; ok {
		t.Errorf("legacy 'data' key must be renamed to dataPoints: %s", out)
	}
	if _, ok := m["dataPoints"].([]interface{}); !ok {
		t.Errorf("expected dataPoints array: %s", out)
	}
	if h, ok := m["_hints"].([]interface{}); !ok || len(h) != 1 {
		t.Errorf("_hints must survive: %s", out)
	}
}

func TestEnsureEnvelope_AlreadyDataPoints(t *testing.T) {
	in := `{"dataPoints":[{"beatsPerMinute":"83"}],"nextPageToken":"X"}`
	m := decode(t, EnsureEnvelope(json.RawMessage(in)))
	if _, ok := m["dataPoints"].([]interface{}); !ok {
		t.Errorf("dataPoints must be preserved")
	}
	if m["nextPageToken"] != "X" {
		t.Errorf("nextPageToken must be preserved")
	}
}

func TestEnsureEnvelope_EmptyRollupGetsRowsKey(t *testing.T) {
	// Empty rollup: the API omits rollupDataPoints, so the result was {"_hints":...}
	// with NO rows key — json.load(out)["dataPoints"] would KeyError.
	out := EnsureEnvelope(json.RawMessage(`{"_hints":["No data found"]}`))
	m := decode(t, out)
	rows, ok := m["dataPoints"].([]interface{})
	if !ok || len(rows) != 0 {
		t.Fatalf("empty result must still carry dataPoints:[], got %s", out)
	}
}

func TestEnsureEnvelope_EmptyObject(t *testing.T) {
	m := decode(t, EnsureEnvelope(json.RawMessage(`{}`)))
	if rows, ok := m["dataPoints"].([]interface{}); !ok || len(rows) != 0 {
		t.Errorf("empty object must become {dataPoints:[]}")
	}
}

func TestEnsureEnvelope_NullBecomesEmptyArray(t *testing.T) {
	// A literal `null` must still yield an indexable dataPoints array, not
	// {"dataPoints": null}.
	m := decode(t, EnsureEnvelope(json.RawMessage(`null`)))
	if rows, ok := m["dataPoints"].([]interface{}); !ok || len(rows) != 0 {
		t.Errorf("null must become {dataPoints:[]}, got %v", m["dataPoints"])
	}
}

// Full-precision values in the row array must pass through byte-for-byte.
func TestEnsureEnvelope_PreservesRowBytes(t *testing.T) {
	out := EnsureEnvelope(json.RawMessage(`[{"bloodGlucoseMilligramsPerDeciliter":80.00000000000001}]`))
	if !contains(string(out), "80.00000000000001") {
		t.Errorf("row precision must be preserved verbatim: %s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
