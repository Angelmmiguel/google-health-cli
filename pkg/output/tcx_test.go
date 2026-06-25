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
	"encoding/csv"
	"strings"
	"testing"
)

// Mirrors a real outdoor GPS export (verified against a live exportExerciseTcx
// response 2026-06-12): default Garmin v2 namespace, one Lap, Trackpoints with
// Time (local offset), Position, AltitudeMeters, cumulative DistanceMeters,
// HeartRateBpm. The Creator element here carries no Version child, so the
// converter must not depend on schema-strict TCX tooling.
const gpsTCX = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">
    <Activities>
        <Activity Sport="Biking">
            <Id>2025-09-08T08:44:43.000+01:00</Id>
            <Lap StartTime="2025-09-08T08:44:43.000+01:00">
                <TotalTimeSeconds>1178.0</TotalTimeSeconds>
                <DistanceMeters>3147.752</DistanceMeters>
                <Calories>59</Calories>
                <Intensity>Active</Intensity>
                <TriggerMethod>Manual</TriggerMethod>
                <Track>
                    <Trackpoint>
                        <Time>2025-09-08T08:45:21.000+01:00</Time>
                        <Position>
                            <LatitudeDegrees>51.532945</LatitudeDegrees>
                            <LongitudeDegrees>-0.12476333333333334</LongitudeDegrees>
                        </Position>
                        <AltitudeMeters>108.4</AltitudeMeters>
                        <DistanceMeters>0.0</DistanceMeters>
                        <HeartRateBpm>
                            <Value>112</Value>
                        </HeartRateBpm>
                    </Trackpoint>
                    <Trackpoint>
                        <Time>2025-09-08T08:45:22.000+01:00</Time>
                        <Position>
                            <LatitudeDegrees>51.53294666666667</LatitudeDegrees>
                            <LongitudeDegrees>-0.12476333333333334</LongitudeDegrees>
                        </Position>
                        <AltitudeMeters>108.4</AltitudeMeters>
                        <DistanceMeters>0.18532516889036676</DistanceMeters>
                        <HeartRateBpm>
                            <Value>112</Value>
                        </HeartRateBpm>
                    </Trackpoint>
                </Track>
            </Lap>
            <Creator xsi:type="Device_t" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
                <Name>Google Pixel Watch 4</Name>
                <UnitId>0</UnitId>
                <ProductID>0</ProductID>
            </Creator>
        </Activity>
    </Activities>
</TrainingCenterDatabase>`

// Mirrors a real lap-less export for an indoor / no-sensor activity (also
// verified live): an Activity with Id + Notes + Creator and no Lap element, so
// the converter must handle a document that contains no trackpoints.
const indoorTCX = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">
    <Activities>
        <Activity Sport="Other">
            <Id>2026-06-07T12:30:31.000+01:00</Id>
            <Notes>Strength session notes live in exercise list, not the CSV.</Notes>
            <Creator xsi:type="Device_t" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
                <UnitId>0</UnitId>
                <ProductID>0</ProductID>
            </Creator>
        </Activity>
    </Activities>
</TrainingCenterDatabase>`

const extensionsTCX = `<?xml version="1.0" encoding="UTF-8"?>
<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">
    <Activities>
        <Activity Sport="Running">
            <Id>2026-06-01T07:00:00.000Z</Id>
            <Lap StartTime="2026-06-01T07:00:00.000Z">
                <Track>
                    <Trackpoint>
                        <Time>2026-06-01T07:00:01.000Z</Time>
                        <Cadence>85</Cadence>
                        <Extensions>
                            <TPX xmlns="http://www.garmin.com/xmlschemas/ActivityExtension/v2">
                                <Speed>3.125</Speed>
                                <Watts>240</Watts>
                            </TPX>
                        </Extensions>
                    </Trackpoint>
                    <Trackpoint>
                        <Time>2026-06-01T07:00:02.000Z</Time>
                        <Extensions>
                            <TPX xmlns="http://www.garmin.com/xmlschemas/ActivityExtension/v2">
                                <RunCadence>86</RunCadence>
                            </TPX>
                        </Extensions>
                    </Trackpoint>
                </Track>
            </Lap>
        </Activity>
    </Activities>
</TrainingCenterDatabase>`

func parseCSV(t *testing.T, out string) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v\n%s", err, out)
	}
	return records
}

func TestWriteTCXAsCSVGPSRide(t *testing.T) {
	var buf bytes.Buffer
	rows, err := WriteTCXAsCSV([]byte(gpsTCX), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2", rows)
	}
	records := parseCSV(t, buf.String())
	if len(records) != 3 {
		t.Fatalf("expected header + 2 rows, got %d records", len(records))
	}
	wantHeader := strings.Join(tcxCSVHeader, ",")
	if got := strings.Join(records[0], ","); got != wantHeader {
		t.Errorf("header = %q, want %q", got, wantHeader)
	}
	first := records[1]
	if first[0] != "2025-09-08T08:45:21.000+01:00" {
		t.Errorf("time = %q", first[0])
	}
	if first[1] != "1" || first[2] != "1" || first[3] != "Biking" {
		t.Errorf("activity/lap/sport = %q/%q/%q", first[1], first[2], first[3])
	}
	// Full round-trip float precision — no %.2f-style corruption.
	second := records[2]
	if second[5] != "-0.12476333333333334" {
		t.Errorf("longitude lost precision: %q", second[5])
	}
	if second[7] != "0.18532516889036676" {
		t.Errorf("distance lost precision: %q", second[7])
	}
	// distance 0.0 on the first point is a true zero, not an empty cell.
	if first[7] != "0" {
		t.Errorf("zero distance must render as 0, got %q", first[7])
	}
	// This export carries no cadence/speed/watts → empty cells.
	if first[9] != "" || first[10] != "" || first[11] != "" {
		t.Errorf("absent sensor columns must be empty: %v", first)
	}
}

func TestWriteTCXAsCSVIndoorLapless(t *testing.T) {
	var buf bytes.Buffer
	rows, err := WriteTCXAsCSV([]byte(indoorTCX), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rows = %d, want 0", rows)
	}
	records := parseCSV(t, buf.String())
	if len(records) != 1 {
		t.Fatalf("lap-less activity must emit header only, got %d records", len(records))
	}
	if strings.Contains(buf.String(), "Strength session") {
		t.Error("Notes must not leak into the trackpoint CSV")
	}
}

func TestWriteTCXAsCSVExtensions(t *testing.T) {
	var buf bytes.Buffer
	rows, err := WriteTCXAsCSV([]byte(extensionsTCX), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2", rows)
	}
	records := parseCSV(t, buf.String())
	first, second := records[1], records[2]
	if first[9] != "85" {
		t.Errorf("Cadence element not read: %q", first[9])
	}
	if first[10] != "3.125" || first[11] != "240" {
		t.Errorf("TPX speed/watts not read: %q/%q", first[10], first[11])
	}
	if second[9] != "86" {
		t.Errorf("TPX RunCadence fallback not read: %q", second[9])
	}
	// Position/altitude/distance/HR absent → empty, not zero.
	if first[4] != "" || first[6] != "" || first[8] != "" {
		t.Errorf("absent fields must be empty cells: %v", first)
	}
}

func TestWriteTCXAsCSVMalformed(t *testing.T) {
	if _, err := WriteTCXAsCSV([]byte("{\"tcxData\": \"oops\"}"), &bytes.Buffer{}); err == nil {
		t.Fatal("JSON envelope (missing alt=media) must fail loudly, not emit an empty CSV")
	}
}

// A valid-but-unrelated XML document (e.g. an error page) must fail loudly via
// the TrainingCenterDatabase root pin, not parse to a misleading 0-row CSV.
func TestWriteTCXAsCSVWrongRoot(t *testing.T) {
	other := `<?xml version="1.0"?><html><body>error</body></html>`
	if _, err := WriteTCXAsCSV([]byte(other), &bytes.Buffer{}); err == nil {
		t.Fatal("non-TCX XML must error, not yield a header-only CSV")
	}
}

const multiLapTCX = `<?xml version="1.0" encoding="UTF-8"?>
<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">
    <Activities>
        <Activity Sport="Running">
            <Id>2026-06-01T07:00:00.000Z</Id>
            <Lap StartTime="2026-06-01T07:00:00.000Z">
                <Track>
                    <Trackpoint><Time>2026-06-01T07:00:01.000Z</Time><HeartRateBpm><Value>120</Value></HeartRateBpm></Trackpoint>
                </Track>
            </Lap>
            <Lap StartTime="2026-06-01T07:05:00.000Z">
                <Track>
                    <Trackpoint><Time>2026-06-01T07:05:01.000Z</Time><HeartRateBpm><Value>140</Value></HeartRateBpm></Trackpoint>
                </Track>
            </Lap>
        </Activity>
    </Activities>
</TrainingCenterDatabase>`

func TestWriteTCXAsCSVMultiLapIndexing(t *testing.T) {
	var buf bytes.Buffer
	rows, err := WriteTCXAsCSV([]byte(multiLapTCX), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2 (one trackpoint per lap)", rows)
	}
	records := parseCSV(t, buf.String())
	// records[0] is the header; lap is column index 2.
	if records[1][2] != "1" || records[2][2] != "2" {
		t.Errorf("lap indices = %q, %q; want 1, 2", records[1][2], records[2][2])
	}
	if records[1][8] != "120" || records[2][8] != "140" {
		t.Errorf("heart_rate per lap = %q, %q; want 120, 140", records[1][8], records[2][8])
	}
}
