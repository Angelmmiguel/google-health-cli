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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// GetFormat resolution order: flag > GHEALTH_FORMAT env > configured profile
// format > "json". The profile step is what 'ghealth config set format'
// writes, so it must actually be honored.
func TestGetFormat_UsesConfiguredProfileFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHEALTH_CONFIG_DIR", dir)
	t.Setenv("GHEALTH_FORMAT", "")
	os.Unsetenv("GHEALTH_FORMAT")

	if err := os.WriteFile(filepath.Join(dir, ConfigFileName),
		[]byte("[default]\nformat = \"table\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := GetFormat(""); got != "table" {
		t.Errorf("GetFormat(\"\") = %q, want %q from config.toml", got, "table")
	}
	if got := GetFormat("csv"); got != "csv" {
		t.Errorf("flag must override config: got %q, want csv", got)
	}
	t.Setenv("GHEALTH_FORMAT", "json")
	if got := GetFormat(""); got != "json" {
		t.Errorf("env must override config: got %q, want json", got)
	}
}

func TestGetFormat_DefaultsToJSON(t *testing.T) {
	t.Setenv("GHEALTH_CONFIG_DIR", t.TempDir())
	os.Unsetenv("GHEALTH_FORMAT")
	if got := GetFormat(""); got != "json" {
		t.Errorf("GetFormat(\"\") = %q, want json", got)
	}
}
