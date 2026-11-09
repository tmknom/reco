package cmd

import (
	"fmt"
	"log/slog"
	"strings"
)

const defaultLogLevel = "info"

// logLevelNames は、CLI 仕様が --log-level に定める値と slog のレベルの対応。
// help とエラー文言に出す値の列挙順を固定するため、map ではなくスライスにする。
var logLevelNames = []struct {
	name  string
	level slog.Level
}{
	{name: "debug", level: slog.LevelDebug},
	{name: "info", level: slog.LevelInfo},
	{name: "warn", level: slog.LevelWarn},
	{name: "error", level: slog.LevelError},
}

// logLevelChoices は、受理する値を help とエラー文言に並べる形で返す。
func logLevelChoices() string {
	names := make([]string, 0, len(logLevelNames))
	for _, n := range logLevelNames {
		names = append(names, n.name)
	}
	return strings.Join(names, ", ")
}

// logLevel は --log-level の値で、pflag.Value を満たす。
// slog.Level.UnmarshalText は "INFO" や "INFO+1" も受理し、CLI 仕様の完全一致に反するため使わない。
// PersistentPreRunE ではなく flag の解析中に検証するのは、--help と同時に指定された不正な値もエラーにするため。
type logLevel struct {
	name  string
	level *slog.LevelVar
}

func newLogLevel(level *slog.LevelVar) *logLevel {
	l := &logLevel{level: level}
	if err := l.Set(defaultLogLevel); err != nil {
		panic(err)
	}
	return l
}

func (l *logLevel) String() string { return l.name }

// Type は help に --log-level LEVEL と表示させる。
func (l *logLevel) Type() string { return "LEVEL" }

func (l *logLevel) Set(s string) error {
	for _, n := range logLevelNames {
		if n.name == s {
			l.name = n.name
			l.level.Set(n.level)
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", logLevelChoices())
}
