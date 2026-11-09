package cmd

import (
	"log/slog"
	"testing"
)

// 名前とレベルの対応を Execute から観測するには各レベルのログを出す処理が要るため、ここで直接確かめる。
func TestLogLevel_Set(t *testing.T) {
	cases := []struct {
		name string
		want slog.Level
	}{
		{name: "debug", want: slog.LevelDebug},
		{name: "info", want: slog.LevelInfo},
		{name: "warn", want: slog.LevelWarn},
		{name: "error", want: slog.LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name+" sets "+tc.want.String(), func(t *testing.T) {
			level := new(slog.LevelVar)
			l := newLogLevel(level)
			// 既定と同じ値でも反映を確かめられるよう、先に別の値にしておく
			level.Set(slog.Level(100))

			if err := l.Set(tc.name); err != nil {
				t.Fatalf("got error %v, want nil", err)
			}
			if got := level.Level(); got != tc.want {
				t.Errorf("got level %v, want %v", got, tc.want)
			}
			if got := l.String(); got != tc.name {
				t.Errorf("got String() %q, want %q", got, tc.name)
			}
		})
	}
}

func TestNewLogLevel_DefaultIsInfo(t *testing.T) {
	level := new(slog.LevelVar)
	level.Set(slog.Level(100))

	l := newLogLevel(level)

	if got := level.Level(); got != slog.LevelInfo {
		t.Errorf("got level %v, want INFO", got)
	}
	if got := l.String(); got != "info" {
		t.Errorf("got String() %q, want info", got)
	}
}
