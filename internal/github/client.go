// Package github は GitHub REST API からリポジトリの設定を取得し、要求の失敗を種類に分けて返す。
package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	apiVersion     = "2026-03-10"
	userAgent      = "reco (https://github.com/tmknom/reco)"
	requestTimeout = 30 * time.Second
	// maxResponseBytes は応答本文を読む上限。リポジトリの応答は数 KB で、上限は異常な応答を読み続けないためにある。
	maxResponseBytes = 1 << 20
)

// Client は GitHub REST API へ認証付きの要求を送る。
type Client struct {
	token      string
	baseURL    *url.URL
	httpClient *http.Client
}

// Option は Client の省略できる設定を変える。
type Option func(*Client)

// WithBaseURL は送信先を u のオリジン (scheme と host) に変える。
// httptest のサーバへ向けるためのもので、平文 HTTP の送信先を指定できる唯一の経路である。u の path は使わない。
func WithBaseURL(u *url.URL) Option {
	return func(c *Client) {
		c.baseURL = &url.URL{Scheme: u.Scheme, Host: u.Host}
	}
}

// New は token で認証する Client を作る。
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:   token,
		baseURL: &url.URL{Scheme: "https", Host: "api.github.com"},
		httpClient: &http.Client{
			Timeout: requestTimeout,
			// リダイレクト先へ認証情報を送らず、要求を再送しないため、3xx を追従せずに受け取る
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) get(ctx context.Context, u *url.URL) ([]byte, error) {
	// http.NewRequestWithContext は URL の解析に失敗し得る形をとるが、ここでは組み立て済みの URL を渡すため、到達しない失敗の分岐を作らないように直接組み立てる
	req := (&http.Request{Method: http.MethodGet, URL: u, Header: http.Header{}}).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommunicationFailure, err)
	}
	// 読み取りだけの応答本文で、Close の失敗は読み取った結果に影響しない
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, classifyFailure(resp.StatusCode, resp.Header)
	}
	return readResponseBody(resp.Body)
}

// readResponseBody は応答本文を maxResponseBytes まで読む。
// 上限で切り詰めると JSON の解析エラーになって原因が読めないため、超過を区別して返す。
func readResponseBody(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommunicationFailure, err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("%w: body exceeds %d bytes", ErrUnexpectedResponse, maxResponseBytes)
	}
	return data, nil
}
