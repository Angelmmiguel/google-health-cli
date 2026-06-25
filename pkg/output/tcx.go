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
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
)

// TCX → CSV flattening for agent/dataframe consumption.
//
// exportExerciseTcx returns TrainingCenterDatabase v2 documents in the default
// Garmin namespace. The shape varies by activity: workouts without a recorded
// time series (e.g. indoor or no-sensor sessions) carry no Lap/Trackpoint
// elements, and optional trackpoint fields (Cadence, the TPX speed/power
// extension) appear only when the source device recorded them. Since pandas
// has no TCX reader, this converter reads whatever trackpoint data is present
// — plus the optional Cadence and TPX Speed/Watts/RunCadence fields when they
// occur — and emits one CSV row per Trackpoint with a FIXED column set so
// downstream dataframes have a stable shape regardless of which sensors were
// present.
//
// Activities with no track produce a header-only CSV: session summaries and
// workout notes are available via `data exercise list`, not this stream.

// tcxCSVHeader is the fixed column set, in order. Absent values are empty
// cells, never zeros — a 0 heart rate and a missing heart rate are different
// facts (same presence rule as rollup output).
var tcxCSVHeader = []string{
	"time", "activity", "lap", "sport",
	"latitude_deg", "longitude_deg", "altitude_m", "distance_m",
	"heart_rate_bpm", "cadence_rpm", "speed_mps", "watts",
}

type tcxDocument struct {
	XMLName    xml.Name      `xml:"TrainingCenterDatabase"`
	Activities []tcxActivity `xml:"Activities>Activity"`
}

type tcxActivity struct {
	Sport string   `xml:"Sport,attr"`
	ID    string   `xml:"Id"`
	Laps  []tcxLap `xml:"Lap"`
}

type tcxLap struct {
	StartTime   string          `xml:"StartTime,attr"`
	Trackpoints []tcxTrackpoint `xml:"Track>Trackpoint"`
}

type tcxTrackpoint struct {
	Time      string       `xml:"Time"`
	Position  *tcxPosition `xml:"Position"`
	Altitude  *float64     `xml:"AltitudeMeters"`
	Distance  *float64     `xml:"DistanceMeters"`
	HeartRate *int         `xml:"HeartRateBpm>Value"`
	Cadence   *int         `xml:"Cadence"`
	TPX       *tcxTPX      `xml:"Extensions>TPX"`
}

type tcxPosition struct {
	Lat float64 `xml:"LatitudeDegrees"`
	Lon float64 `xml:"LongitudeDegrees"`
}

// tcxTPX is the Garmin ActivityExtension v2 trackpoint extension — the standard
// home for speed, power, and running cadence. It appears only when the source
// device records those metrics, so we read it when present.
type tcxTPX struct {
	Speed      *float64 `xml:"Speed"`
	Watts      *int     `xml:"Watts"`
	RunCadence *int     `xml:"RunCadence"`
}

// WriteTCXAsCSV parses a TCX document and writes one CSV row per Trackpoint.
// Returns the number of data rows written (0 for lap-less indoor activities,
// which still get the header so the output is always a valid CSV).
func WriteTCXAsCSV(tcxData []byte, w io.Writer) (int, error) {
	var doc tcxDocument
	if err := xml.Unmarshal(tcxData, &doc); err != nil {
		return 0, fmt.Errorf("parse TCX: %w", err)
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(tcxCSVHeader); err != nil {
		return 0, err
	}

	rows := 0
	for ai, activity := range doc.Activities {
		for li, lap := range activity.Laps {
			for _, tp := range lap.Trackpoints {
				record := []string{
					tp.Time,
					strconv.Itoa(ai + 1),
					strconv.Itoa(li + 1),
					activity.Sport,
					tcxFloatCell(positionLat(tp.Position)),
					tcxFloatCell(positionLon(tp.Position)),
					tcxFloatCell(tp.Altitude),
					tcxFloatCell(tp.Distance),
					tcxIntCell(tp.HeartRate),
					tcxIntCell(cadenceOf(tp)),
					tcxFloatCell(tpxSpeed(tp.TPX)),
					tcxIntCell(tpxWatts(tp.TPX)),
				}
				if err := cw.Write(record); err != nil {
					return rows, err
				}
				rows++
			}
		}
	}
	cw.Flush()
	return rows, cw.Error()
}

// cadenceOf prefers the trackpoint's Cadence element (bike) and falls back to
// the TPX RunCadence extension — they are the same physical quantity (rpm /
// strides-per-minute) reported by different sport profiles.
func cadenceOf(tp tcxTrackpoint) *int {
	if tp.Cadence != nil {
		return tp.Cadence
	}
	if tp.TPX != nil {
		return tp.TPX.RunCadence
	}
	return nil
}

func positionLat(p *tcxPosition) *float64 {
	if p == nil {
		return nil
	}
	return &p.Lat
}

func positionLon(p *tcxPosition) *float64 {
	if p == nil {
		return nil
	}
	return &p.Lon
}

func tpxSpeed(t *tcxTPX) *float64 {
	if t == nil {
		return nil
	}
	return t.Speed
}

func tpxWatts(t *tcxTPX) *int {
	if t == nil {
		return nil
	}
	return t.Watts
}

// tcxFloatCell renders a float with full round-trip precision (the same rule
// as tabular output: %.2f-style rounding silently corrupts sensor data).
func tcxFloatCell(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}

func tcxIntCell(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}
