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

package types

import "testing"

// ECG must filter on physical interval.start_time (with Z) and support only a
// lower bound; civil_start_time and any end bound are rejected by the API.
func TestECGFilter_PhysicalStartOnly(t *testing.T) {
	ecg := Get("electrocardiogram")
	if ecg == nil {
		t.Fatal("electrocardiogram not registered")
	}
	if ecg.TimeField != TimeFieldPhysicalIntervalStart {
		t.Fatalf("ECG TimeField = %q, want %q", ecg.TimeField, TimeFieldPhysicalIntervalStart)
	}

	from := ecg.FilterFrom("2026-05-01T00:00:00")
	wantFrom := `electrocardiogram.interval.start_time >= "2026-05-01T00:00:00Z"`
	if from != wantFrom {
		t.Errorf("FilterFrom = %q, want %q", from, wantFrom)
	}

	// End bound must be empty (unsupported) so buildFilter omits it.
	if to := ecg.FilterTo("2026-06-09T00:00:00"); to != "" {
		t.Errorf("FilterTo = %q, want empty (end bound unsupported)", to)
	}
}

// A normal interval type must be unaffected (still civil_start_time, both bounds).
func TestIntervalFilter_StillCivilBothBounds(t *testing.T) {
	steps := Get("steps")
	if got := steps.FilterFrom("2026-05-01T00:00:00"); got != `steps.interval.civil_start_time >= "2026-05-01T00:00:00"` {
		t.Errorf("steps FilterFrom = %q", got)
	}
	if got := steps.FilterTo("2026-06-01T00:00:00"); got != `steps.interval.civil_start_time < "2026-06-01T00:00:00"` {
		t.Errorf("steps FilterTo = %q", got)
	}
}
