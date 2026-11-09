package github

import (
	"bytes"
	"errors"
	"testing"
	"testing/iotest"
)

func TestNew(t *testing.T) {
	c := New("token")
	if c.httpClient.Timeout <= 0 {
		t.Errorf("http client timeout = %v, want a positive limit", c.httpClient.Timeout)
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
