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

func TestEnsureUTC(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Zoneless datetime → append Z.
		{"2026-03-07T00:00:00", "2026-03-07T00:00:00Z"},
		// Already UTC → unchanged.
		{"2026-03-07T00:00:00Z", "2026-03-07T00:00:00Z"},
		// Positive offset → unchanged.
		{"2026-03-01T00:00:00+02:00", "2026-03-01T00:00:00+02:00"},
		// Negative offset → unchanged (regression: previously became "...-05:00Z").
		{"2026-03-01T00:00:00-05:00", "2026-03-01T00:00:00-05:00"},
		// Fractional seconds with negative offset → unchanged.
		{"2026-03-01T00:00:00.5-08:00", "2026-03-01T00:00:00.5-08:00"},
	}
	for _, c := range cases {
		if got := ensureUTC(c.in); got != c.want {
			t.Errorf("ensureUTC(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFilterTo_SamplePreservesNegativeOffset(t *testing.T) {
	dt := &DataType{FilterName: "heart_rate", TimeField: TimeFieldSample}

	got := dt.FilterTo("2026-03-01T00:00:00-05:00")
	want := `heart_rate.sample_time.physical_time < "2026-03-01T00:00:00-05:00"`
	if got != want {
		t.Errorf("FilterTo negative offset = %q, want %q", got, want)
	}

	// Zoneless input should still get a Z (UTC) appended.
	gotZ := dt.FilterTo("2026-03-07T00:00:00")
	wantZ := `heart_rate.sample_time.physical_time < "2026-03-07T00:00:00Z"`
	if gotZ != wantZ {
		t.Errorf("FilterTo zoneless = %q, want %q", gotZ, wantZ)
	}
}

func TestFilterFrom_SamplePreservesNegativeOffset(t *testing.T) {
	dt := &DataType{FilterName: "weight", TimeField: TimeFieldSample}
	got := dt.FilterFrom("2026-03-01T00:00:00-08:00")
	want := `weight.sample_time.physical_time >= "2026-03-01T00:00:00-08:00"`
	if got != want {
		t.Errorf("FilterFrom negative offset = %q, want %q", got, want)
	}
}
