package github

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"testing/iotest"
)

func TestNew(t *testing.T) {
	c := New("token")
	if c.httpClient.Timeout <= 0 {
		t.Errorf("http client timeout = %v, want a positive limit", c.httpClient.Timeout)
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

func TestReadResponseBody(t *testing.T) {
	cases := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "body at the limit is read", size: maxResponseBytes},
		{name: "body over the limit is unexpected response", size: maxResponseBytes + 1, wantErr: ErrUnexpectedResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.Repeat([]byte("a"), tc.size)
			got, err := readResponseBody(bytes.NewReader(body))
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
		_, err := readResponseBody(iotest.ErrReader(readErr))
		if !errors.Is(err, ErrCommunicationFailure) || !errors.Is(err, readErr) {
			t.Errorf("readResponseBody() error = %v, want %v wrapping %v", err, ErrCommunicationFailure, readErr)
		}
	})
}
