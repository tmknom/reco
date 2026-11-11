package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// approvalPrompt は、変更計画を提示したあとに書く承認プロンプト。
const approvalPrompt = "Apply these changes? [y/N]: "

// requestApproval は、利用者に対話承認を求め、承認されたときだけ nil を返す。
func requestApproval(ctx context.Context, in io.Reader, interactive bool, prompt io.Writer) error {
	if !interactive {
		return errors.New("approval not available: standard input is not a terminal")
	}
	if _, err := io.WriteString(prompt, approvalPrompt); err != nil {
		return fmt.Errorf("write approval prompt: %w", err)
	}
	line, err := readApprovalLine(ctx, in)
	if err != nil {
		// 改行を入力せずに終わったプロンプトの行を閉じ、続く出力をプロンプトと同じ行に並べない。
		// 行を閉じられなくても承認しない結果は変わらないため、書き込みの失敗は捨てる
		_, _ = io.WriteString(prompt, "\n")
		return err
	}
	if !isApproval(line) {
		return errors.New("approval declined")
	}
	return nil
}

type readResult struct {
	line string
	err  error
}

// readApprovalLine は、in の先頭 1 行を行末の LF を除いて返す。改行で終わらない入力は EOF として扱う。
// 読み取る量は bufio.Reader の既定のバッファの大きさまでで、それを超える行は bufio.ErrBufferFull で拒否する。
// io.Reader の Read は ctx で止められないため、読み取りを goroutine で行い、ctx の中断を待たずに返す。
// 中断で返したあとも goroutine は Read が返るまで残るが、その後すぐにプロセスが終わるため終了を待たない。
func readApprovalLine(ctx context.Context, in io.Reader) (string, error) {
	result := make(chan readResult, 1)
	go func() {
		line, err := bufio.NewReader(in).ReadSlice('\n')
		result <- readResult{line: string(line), err: err}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("wait for approval: %w", ctx.Err())
	case r := <-result:
		if r.err != nil {
			return "", fmt.Errorf("read approval: %w", r.err)
		}
		return strings.TrimSuffix(r.line, "\n"), nil
	}
}

// isApproval は、行末の改行を除いた入力の行が承認を表すかを判定する。
// 誤って承認するよりも承認を取りこぼすほうを選び、表記の揺れを同一視しない。
func isApproval(line string) bool {
	return line == "y" || line == "yes"
}
