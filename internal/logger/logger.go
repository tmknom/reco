// Package logger は、CLI 全体で使う診断ログの logger を組み立てる。
package logger

import (
	"io"
	"log/slog"
)

// New は、w に slog の JSON で書く logger を返す。
// level は flag の解析後に設定するため、構築後の変更が効く *slog.LevelVar で受ける。
func New(w io.Writer, level *slog.LevelVar) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
