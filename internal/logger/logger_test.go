package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func records(t *testing.T, out string) []map[string]any {
	t.Helper()
	var got []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("got non-JSON line %q: %v", line, err)
		}
		got = append(got, m)
	}
	return got
}

func TestNew(t *testing.T) {
	t.Run("records at the level are written as JSON with slog default keys", func(t *testing.T) {
		var out bytes.Buffer
		level := new(slog.LevelVar)
		level.Set(slog.LevelWarn)

		New(&out, level).Warn("written")

		got := records(t, out.String())
		if len(got) != 1 {
			t.Fatalf("got %d records %v, want 1", len(got), got)
		}
		if got[0]["level"] != "WARN" || got[0]["msg"] != "written" || got[0]["time"] == nil {
			t.Errorf("got record %v, want level WARN, msg written and time", got[0])
		}
	})

	t.Run("changing the level after construction takes effect", func(t *testing.T) {
		var out bytes.Buffer
		level := new(slog.LevelVar)
		level.Set(slog.LevelWarn)
		log := New(&out, level)

		level.Set(slog.LevelDebug)
		log.Debug("debug")

		got := records(t, out.String())
		if len(got) != 1 || got[0]["msg"] != "debug" {
			t.Errorf("got records %v, want one debug record", got)
		}
	})
}
