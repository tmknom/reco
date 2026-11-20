package cmd

import (
	"context"
	"io"
	"testing"
)

// unreadableReader は、読まれたらテストを失敗させる標準入力。
type unreadableReader struct{ t *testing.T }

// blockingReader は、読まれたら cancel を呼び、テストの終了まで止まる標準入力。
type blockingReader struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (r unreadableReader) Read([]byte) (int, error) {
	r.t.Error("standard input was read")
	return 0, io.EOF
}

func (r blockingReader) Read([]byte) (int, error) {
	r.cancel()
	<-r.done
	return 0, io.EOF
}
