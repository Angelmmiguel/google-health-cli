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
	"testing"
	"time"
)

// pinTZ fixes the process-local timezone for the duration of a test so that
// local-day date anchoring (anchorLocalToUTC) is deterministic regardless of
// the host's timezone. offsetSeconds is the fixed UTC offset (0 == UTC).
// It also points GHEALTH_CONFIG_DIR at a fresh temp dir (so the developer's
// real config can't leak a timezone into the test) and resets the cached
// active location. The config dir is returned so tests can write a config.
func pinTZ(t *testing.T, offsetSeconds int) string {
	t.Helper()
	orig := time.Local
	time.Local = time.FixedZone("PINNED", offsetSeconds)
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	resetActiveLocation()
	t.Cleanup(func() {
		time.Local = orig
		resetActiveLocation()
	})
	return dir
}
