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
	"strings"
	"testing"
)

// The generated exercise→heart-rate hint must use only API-supported
// comparators: >= and < (the API rejects <=).
func TestGenerateHints_ExerciseHRHintUsesSupportedComparators(t *testing.T) {
	data := json.RawMessage(`{
		"dataPoints": [
			{"start": "2026-03-29T14:18:32+01:00", "end": "2026-03-29T14:39:14+01:00", "exerciseType": "RUN"}
		]
	}`)

	hints := GenerateHints(data, "exercise", "list", 0, "", "", false)
	if len(hints) == 0 {
		t.Fatal("expected an exercise correlation hint")
	}
	for _, h := range hints {
		if strings.Contains(h, "<=") {
			t.Errorf("hint contains unsupported '<=': %s", h)
		}
	}
	found := false
	for _, h := range hints {
		if strings.Contains(h, `physical_time < "2026-03-29T13:39:14Z"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an upper bound using '<', got: %v", hints)
	}
}

// The sleep --detail suggestion must not fire when the caller already
// requested the detailed view.
func TestGenerateHints_SleepDetailHintSuppressedWhenDetailSet(t *testing.T) {
	data := json.RawMessage(`{
		"dataPoints": [
			{"start": "2026-06-11T00:42:00+01:00", "end": "2026-06-11T08:26:00+01:00", "stages": []}
		]
	}`)

	for _, h := range GenerateHints(data, "sleep", "list", 0, "", "", true) {
		if strings.Contains(h, "--detail") {
			t.Errorf("--detail hint fired even though detail was already set: %s", h)
		}
	}

	found := false
	for _, h := range GenerateHints(data, "sleep", "list", 0, "", "", false) {
		if strings.Contains(h, "--detail") {
			found = true
		}
	}
	if !found {
		t.Error("expected the --detail hint when detail is not set")
	}
}
