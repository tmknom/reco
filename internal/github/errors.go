package github

import (
	"errors"
	"fmt"
	"net/http"
)

// GitHub への要求が失敗した種類。
var (
	// ErrUnauthorized は、認証情報が受け入れられなかったことを表す。
	ErrUnauthorized = errors.New("authentication failed")
	// ErrForbidden は、操作が拒否されたことを表す。レート制限と判定できる拒否は含めない。
	ErrForbidden = errors.New("permission denied")
	// ErrRateLimited は、GitHub の利用枠の超過により拒否されたことを表す。
	ErrRateLimited = errors.New("rate limit exceeded")
	// ErrNotFound は、GitHub が対象を見つけられないと応答したことを表す。
	// GitHub は権限のない非公開リポジトリにも 404 を返すため、文言で不存在を断定しない。
	ErrNotFound = errors.New("not found or not accessible")
	// ErrUnprocessable は、GitHub が要求の内容を処理できないと応答したことを表す。
	// GitHub は値の検証の失敗にも過剰な要求にも 422 を返すため、文言で原因を断定しない。
	ErrUnprocessable = errors.New("request not processable")
	// ErrRedirect は、リダイレクト応答を受けて追従しなかったことを表す。
	ErrRedirect = errors.New("redirect not followed")
	// ErrUnexpectedResponse は、取得や更新の契約に適合しない応答を受けたことを表す。
	ErrUnexpectedResponse = errors.New("unexpected response")
	// ErrCommunicationFailure は、接続の失敗、時間切れ、中断などで応答を受け取りきれなかったことを表す。
	ErrCommunicationFailure = errors.New("communication failed")
)

// classifyFailure は 200 以外の応答を失敗の種類に変換する。
// Location などのヘッダーと応答本文は、秘密情報を含み得るため文言に入れない。
func classifyFailure(status int, header http.Header) error {
	var kind error
	switch {
	case 300 <= status && status < 400:
		return fmt.Errorf("%w (HTTP %d): the repository may have been renamed or transferred; specify its current name", ErrRedirect, status)
	case status == http.StatusUnauthorized:
		kind = ErrUnauthorized
	case status == http.StatusForbidden && isRateLimited(header):
		kind = ErrRateLimited
	case status == http.StatusForbidden:
		kind = ErrForbidden
	case status == http.StatusNotFound:
		kind = ErrNotFound
	case status == http.StatusUnprocessableEntity:
		kind = ErrUnprocessable
	case status == http.StatusTooManyRequests:
		kind = ErrRateLimited
	default:
		kind = ErrUnexpectedResponse
	}
	return fmt.Errorf("%w (HTTP %d)", kind, status)
}

// isRateLimited は、ステータスだけでは権限不足と区別できない 403 がレート制限によるものかを、GitHub が返す応答ヘッダーで判定する。
func isRateLimited(header http.Header) bool {
	return header.Get("Retry-After") != "" || header.Get("X-RateLimit-Remaining") == "0"
}
