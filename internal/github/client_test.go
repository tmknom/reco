package github

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"testing"
	"testing/iotest"
)

func TestNew(t *testing.T) {
	c := New("token")
	if c.httpClient.Timeout <= 0 {
		t.Errorf("http client timeout = %v, want a positive limit", c.httpClient.Timeout)
	}
}

// TestClientMethodPrefixes は、Client の公開メソッドが、読み取りの Observe か、lint の forbidigo 規則が internal/apply の外で禁じる更新の接頭辞で始まることを確かめる。
// 規則は更新メソッドを接頭辞で見分けるため、別の接頭辞の更新メソッドを足すと、ほかの package から呼んでも lint が検出しない。
// 接頭辞は lint の設定から読み、その接頭辞を規則に足すか、名前を合わせるまで、このテストが失敗するようにする。
//
//trace:checks INV-003
func TestClientMethodPrefixes(t *testing.T) {
	config, err := os.ReadFile("../../scripts/golangci-lint/.golangci.yml")
	if err != nil {
		t.Fatalf("read lint configuration: %v", err)
	}
	rule := regexp.MustCompile(`pattern: \^github\\\.Client\\\.\(([A-Za-z|]+)\)`).FindSubmatch(config)
	if rule == nil {
		t.Fatal("lint configuration has no forbidigo pattern for update methods of github.Client")
	}
	prefixes := regexp.MustCompile(`^(Observe|` + string(rule[1]) + `)`)
	for method := range reflect.TypeFor[*Client]().Methods() {
		if !prefixes.MatchString(method.Name) {
			t.Errorf("method %s starts with neither Observe nor a prefix of update methods forbidden outside internal/apply", method.Name)
		}
	}
}

const sendTestBody = `{"ok":true}`

// TestSendNormalStatus は、要求ごとに渡した正常応答のステータスだけを成功とすることを確かめる。
//
//trace:checks FR-013
func TestSendNormalStatus(t *testing.T) {
	cases := []struct {
		name         string
		normalStatus int
		status       int
		wantData     string
		wantErr      error
	}{
		{name: "200 for a 200 request returns the body", normalStatus: http.StatusOK, status: http.StatusOK, wantData: sendTestBody},
		{name: "204 for a 200 request is unexpected response", normalStatus: http.StatusOK, status: http.StatusNoContent, wantErr: ErrUnexpectedResponse},
		{name: "204 for a 204 request succeeds", normalStatus: http.StatusNoContent, status: http.StatusNoContent},
		{name: "200 for a 204 request is unexpected response", normalStatus: http.StatusNoContent, status: http.StatusOK, wantErr: ErrUnexpectedResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(sendTestBody))
			})
			u := c.repositoryURL(mustReference(t, "owner/repo"))

			status, data, err := c.send(t.Context(), http.MethodGet, u, nil, tc.normalStatus)
			if status != tc.status {
				t.Errorf("send() status = %d, want %d", status, tc.status)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("send() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("send() error = %v", err)
			}
			if string(data) != tc.wantData {
				t.Errorf("send() data = %q, want %q", data, tc.wantData)
			}
		})
	}
}

// TestSendBodyLimit は、send が要求ごとの上限を渡さないとき、応答本文を maxResponseBytes まで読むことを確かめる。
//
//trace:checks FR-004 QR-002
func TestSendBodyLimit(t *testing.T) {
	cases := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "body at maxResponseBytes is read", size: maxResponseBytes},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(bytes.Repeat([]byte("a"), tc.size))
			})
			_, data, err := c.send(t.Context(), http.MethodGet, c.repositoryURL(mustReference(t, "owner/repo")), nil, http.StatusOK)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("send() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("send() error = %v", err)
			}
			if len(data) != tc.size {
				t.Errorf("send() returned %d bytes, want %d", len(data), tc.size)
			}
		})
	}
}

func TestReadResponseBody(t *testing.T) {
	// maxResponseBytes と違う上限を渡し、渡した上限で判定することを確かめる
	const limit = 10
	cases := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "body at the given limit is read", size: limit},
		{name: "body over the given limit is unexpected response", size: limit + 1, wantErr: ErrUnexpectedResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.Repeat([]byte("a"), tc.size)
			got, err := readResponseBody(bytes.NewReader(body), limit)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("readResponseBody() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("readResponseBody() error = %v", err)
			}
			if !bytes.Equal(got, body) {
				t.Errorf("readResponseBody() returned %d bytes, want %d", len(got), len(body))
			}
		})
	}

	t.Run("read failure is communication failure", func(t *testing.T) {
		readErr := errors.New("connection reset")
		_, err := readResponseBody(iotest.ErrReader(readErr), maxResponseBytes)
		if !errors.Is(err, ErrCommunicationFailure) || !errors.Is(err, readErr) {
			t.Errorf("readResponseBody() error = %v, want %v wrapping %v", err, ErrCommunicationFailure, readErr)
		}
	})
}
