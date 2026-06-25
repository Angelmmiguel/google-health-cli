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
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Simplified rollup output is a top-level JSON array; extractRows must turn
// it into table/CSV rows rather than letting PrintTable fall back to JSON.
func TestExtractRows_TopLevelArray(t *testing.T) {
	data := json.RawMessage(`[
		{"date": "2026-06-10", "countSum": "9221"},
		{"date": "2026-06-11", "countSum": "8329"}
	]`)
	rows := extractRows(data)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows from a bare array, got %d", len(rows))
	}
	if rows[0]["date"] != "2026-06-10" {
		t.Errorf("unexpected first row: %v", rows[0])
	}
}

// Raw rollup responses keep their rollupDataPoints key; table/CSV rendering
// must find rows under it.
func TestExtractRows_RollupDataPointsKey(t *testing.T) {
	data := json.RawMessage(`{"rollupDataPoints": [{"startTime": "2026-06-10T00:00:00Z", "steps": {"countSum": "9221"}}]}`)
	rows := extractRows(data)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row under rollupDataPoints, got %d", len(rows))
	}
}

// CSV is a data-export format: float values must round-trip at full
// precision, not be rounded to 2 decimals like the aligned table view.
func TestPrintRowsAsCSV_FullFloatPrecision(t *testing.T) {
	rows := []map[string]interface{}{
		{"temperatureCelsius": 36.789, "label": "x"},
	}
	var buf bytes.Buffer
	if err := printRowsAsCSV(&buf, rows); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "36.789") {
		t.Errorf("CSV lost float precision: %q", buf.String())
	}
	if strings.Contains(buf.String(), "36.79,") || strings.HasSuffix(strings.TrimSpace(buf.String()), "36.79") {
		t.Errorf("CSV rounded float to 2 decimals: %q", buf.String())
	}
}
