package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

//trace:checks INV-003
func TestIsApproval(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{name: "y is approval", line: "y", want: true},
		{name: "yes is approval", line: "yes", want: true},
		{name: "empty line is not approval", line: "", want: false},
		{name: "upper case Y is not approval", line: "Y", want: false},
		{name: "leading space is not approval", line: " y", want: false},
		{name: "yes followed by other words is not approval", line: "yes please", want: false},
		{name: "trailing CR is not approval", line: "y\r", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isApproval(tc.line); got != tc.want {
				t.Errorf("isApproval(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// unreadableReader は、読まれたらテストを失敗させる標準入力。
type unreadableReader struct{ t *testing.T }

func (r unreadableReader) Read([]byte) (int, error) {
	r.t.Error("standard input was read")
	return 0, io.EOF
}

//trace:checks FR-009
//trace:checks INV-003
func TestRequestApproval_NotTerminal(t *testing.T) {
	var prompt bytes.Buffer
	err := requestApproval(context.Background(), unreadableReader{t}, false, &prompt)
	if err == nil {
		t.Error("got nil, want an error")
	}
	if prompt.Len() != 0 {
		t.Errorf("got prompt %q, want nothing written", prompt.String())
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

//trace:checks INV-003
func TestRequestApproval_PromptWriteFailure(t *testing.T) {
	writeFailure := errors.New("write failure")
	err := requestApproval(context.Background(), unreadableReader{t}, true, failingWriter{err: writeFailure})
	if !errors.Is(err, writeFailure) {
		t.Errorf("got error %v, want %v", err, writeFailure)
	}
}

// failingReader は、data を返したあとに err を返す標準入力。
type failingReader struct {
	data string
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.data == "" {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

//trace:checks FR-009
//trace:checks INV-003
func TestRequestApproval_Terminal(t *testing.T) {
	readFailure := errors.New("read failure")
	cases := []struct {
		name         string
		in           io.Reader
		wantApproved bool
		// wantErrIs は、拒否のときに error がたどれるべき原因。nil なら原因を問わない。
		wantErrIs  error
		wantPrompt string
	}{
		{name: "y line approves and writes only the prompt", in: strings.NewReader("y\n"), wantApproved: true, wantPrompt: approvalPrompt},
		{name: "n line is declined", in: strings.NewReader("n\n"), wantPrompt: approvalPrompt},
		{name: "y without a newline is declined as EOF and ends the prompt line", in: strings.NewReader("y"), wantErrIs: io.EOF, wantPrompt: approvalPrompt + "\n"},
		{name: "read failure is declined and ends the prompt line", in: &failingReader{data: "y", err: readFailure}, wantErrIs: readFailure, wantPrompt: approvalPrompt + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var prompt bytes.Buffer
			err := requestApproval(context.Background(), tc.in, true, &prompt)
			if tc.wantApproved && err != nil {
				t.Errorf("got error %v, want nil", err)
			}
			if !tc.wantApproved && err == nil {
				t.Error("got nil, want an error")
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Errorf("got error %v, want %v", err, tc.wantErrIs)
			}
			if prompt.String() != tc.wantPrompt {
				t.Errorf("got prompt %q, want %q", prompt.String(), tc.wantPrompt)
			}
		})
	}
}

// endlessReader は、改行を含まない入力を際限なく返し、読ませた量を数える標準入力。
type endlessReader struct{ read int }

func (r *endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'y'
	}
	r.read += len(p)
	return len(p), nil
}

//trace:checks INV-003
func TestRequestApproval_LineWithoutNewlineIsBounded(t *testing.T) {
	in := &endlessReader{}
	err := requestApproval(context.Background(), in, true, io.Discard)
	if err == nil {
		t.Error("got nil, want an error")
	}
	if in.read > 1<<20 {
		t.Errorf("read %d bytes, want at most 1 MiB", in.read)
	}
}

// blockingReader は、読まれたら cancel を呼び、テストの終了まで止まる標準入力。
type blockingReader struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (r blockingReader) Read([]byte) (int, error) {
	r.cancel()
	<-r.done
	return 0, io.EOF
}

//trace:checks INV-003
func TestRequestApproval_CanceledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	var prompt bytes.Buffer
	err := requestApproval(ctx, blockingReader{cancel: cancel, done: done}, true, &prompt)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
	if prompt.String() != approvalPrompt+"\n" {
		t.Errorf("got prompt %q, want the prompt line ended", prompt.String())
	}
}
