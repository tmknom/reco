package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

//trace:checks INV-001 FR-011
func TestCreateRepositoryVariable(t *testing.T) {
	cases := []struct {
		name  string
		value string
		// wantBody は、値のエスケープの形まで固定するため、本文の JSON の文字列で持つ。
		wantBody string
	}{
		{name: "sends the name and the value", value: "1.27", wantBody: `{"name":"GO_VERSION","value":"1.27"}`},
		{name: "value is escaped as JSON", value: "a\x01\"b\n", wantBody: `{"name":"GO_VERSION","value":"a\u0001\"b\n"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			var gotMethod, gotPath, gotRawQuery, gotContentType string
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				gotMethod, gotPath, gotRawQuery, gotContentType = r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Content-Type")
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
			})

			outcome, err := c.CreateRepositoryVariable(t.Context(), mustReference(t, "owner/repo"), mustVariableName(t, "GO_VERSION"), tc.value)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("CreateRepositoryVariable() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPost || gotPath != "/repos/owner/repo/actions/variables" || gotRawQuery != "" {
				t.Errorf("request = %s %s?%s, want POST /repos/owner/repo/actions/variables without a query", gotMethod, gotPath, gotRawQuery)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if string(gotBody) != tc.wantBody {
				t.Errorf("request body = %s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

// TestUpdateRepositoryVariableValue は、値の変更が名前を本文に含めず、変数の名前を変える経路を作らないことを確かめる。
//
//trace:checks INV-001 FR-011
func TestUpdateRepositoryVariableValue(t *testing.T) {
	var received atomic.Int32
	var gotMethod, gotPath, gotRawQuery, gotContentType string
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		gotMethod, gotPath, gotRawQuery, gotContentType = r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	outcome, err := c.UpdateRepositoryVariableValue(t.Context(), mustReference(t, "owner/repo"), mustVariableName(t, "GO_VERSION"), "a\x01\"b\n")
	if err != nil || outcome != UpdateSucceeded {
		t.Fatalf("UpdateRepositoryVariableValue() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
	}

	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
	if gotMethod != http.MethodPatch || gotPath != "/repos/owner/repo/actions/variables/GO_VERSION" || gotRawQuery != "" {
		t.Errorf("request = %s %s?%s, want PATCH /repos/owner/repo/actions/variables/GO_VERSION without a query", gotMethod, gotPath, gotRawQuery)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if want := `{"value":"a\u0001\"b\n"}`; string(gotBody) != want {
		t.Errorf("request body = %s, want %s", gotBody, want)
	}
}

// TestRepositoryVariableUpdateOutcome は、変数の作成と値の変更の応答から成否を判定し、再送せず、error に値を含めないことを確かめる。
//
//trace:checks FR-013 INV-005
func TestRepositoryVariableUpdateOutcome(t *testing.T) {
	const value = "s3cr3t-value-1.27"
	create := func(c *Client, t *testing.T) (UpdateOutcome, error) {
		return c.CreateRepositoryVariable(t.Context(), mustReference(t, "example/project-a"), mustVariableName(t, "GO_VERSION"), value)
	}
	update := func(c *Client, t *testing.T) (UpdateOutcome, error) {
		return c.UpdateRepositoryVariableValue(t.Context(), mustReference(t, "example/project-a"), mustVariableName(t, "GO_VERSION"), value)
	}
	cases := []struct {
		name        string
		send        func(*Client, *testing.T) (UpdateOutcome, error)
		status      int
		wantOutcome UpdateOutcome
		wantErr     error
		wantPrefix  string
	}{
		{name: "creation 409 is rejected", send: create, status: http.StatusConflict, wantOutcome: UpdateRejected, wantErr: ErrConflict, wantPrefix: `create repository variable "GO_VERSION" example/project-a: `},
		{name: "creation 200 is outcome unknown", send: create, status: http.StatusOK, wantOutcome: UpdateUnknown, wantPrefix: `create repository variable "GO_VERSION" example/project-a: `},
		{name: "value update 404 is rejected", send: update, status: http.StatusNotFound, wantOutcome: UpdateRejected, wantErr: ErrNotFound, wantPrefix: `update repository variable "GO_VERSION" example/project-a: `},
		{name: "value update 200 is outcome unknown", send: update, status: http.StatusOK, wantOutcome: UpdateUnknown, wantPrefix: `update repository variable "GO_VERSION" example/project-a: `},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				received.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"` + value + `"}`))
			})

			outcome, err := tc.send(c, t)
			if outcome != tc.wantOutcome || err == nil {
				t.Fatalf("outcome = %v, %v, want %v with an error", outcome, err, tc.wantOutcome)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Errorf("error = %q, want it to start with %q", err, tc.wantPrefix)
			}
			if strings.Contains(err.Error(), value) {
				t.Errorf("error = %q, want it not to contain the value", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}

//trace:checks INV-001 FR-011
func TestCreateEnvironmentVariable(t *testing.T) {
	t.Run("posts the name and the value to the environment's variables", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody []byte
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.EscapedPath()
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		})

		outcome, err := c.CreateEnvironmentVariable(t.Context(), mustReference(t, "owner/repo"), "production", mustVariableName(t, "REGION"), "eu")
		if err != nil || outcome != UpdateSucceeded {
			t.Fatalf("CreateEnvironmentVariable() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
		}
		if gotMethod != http.MethodPost || gotPath != "/repos/owner/repo/environments/production/variables" {
			t.Errorf("request = %s %s, want POST /repos/owner/repo/environments/production/variables", gotMethod, gotPath)
		}
		if want := `{"name":"REGION","value":"eu"}`; string(gotBody) != want {
			t.Errorf("request body = %s, want %s", gotBody, want)
		}
	})

	t.Run("unaddressable environment name panics without sending a request", func(t *testing.T) {
		var received atomic.Int32
		c := newTestClient(t, func(http.ResponseWriter, *http.Request) { received.Add(1) })
		assertPanics(t, func() {
			_, _ = c.CreateEnvironmentVariable(t.Context(), mustReference(t, "owner/repo"), "..", mustVariableName(t, "REGION"), "eu")
		})
		if n := received.Load(); n != 0 {
			t.Errorf("server received %d requests, want 0", n)
		}
	})
}

//trace:checks INV-001 FR-011
func TestUpdateEnvironmentVariableValue(t *testing.T) {
	t.Run("patches only the value to the variable of the environment", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody []byte
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.EscapedPath()
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		})

		outcome, err := c.UpdateEnvironmentVariableValue(t.Context(), mustReference(t, "owner/repo"), "production", mustVariableName(t, "REGION"), "eu")
		if err != nil || outcome != UpdateSucceeded {
			t.Fatalf("UpdateEnvironmentVariableValue() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
		}
		if gotMethod != http.MethodPatch || gotPath != "/repos/owner/repo/environments/production/variables/REGION" {
			t.Errorf("request = %s %s, want PATCH /repos/owner/repo/environments/production/variables/REGION", gotMethod, gotPath)
		}
		if want := `{"value":"eu"}`; string(gotBody) != want {
			t.Errorf("request body = %s, want %s", gotBody, want)
		}
	})

	t.Run("unaddressable environment name panics without sending a request", func(t *testing.T) {
		var received atomic.Int32
		c := newTestClient(t, func(http.ResponseWriter, *http.Request) { received.Add(1) })
		assertPanics(t, func() {
			_, _ = c.UpdateEnvironmentVariableValue(t.Context(), mustReference(t, "owner/repo"), "..", mustVariableName(t, "REGION"), "eu")
		})
		if n := received.Load(); n != 0 {
			t.Errorf("server received %d requests, want 0", n)
		}
	})
}

// TestEnvironmentVariableUpdateOutcome は、Environment の変数の作成と値の変更のエラーが、操作名と Environment と変数の名前を示し、値を含めないことを確かめる。
// 操作名はメソッドごとに組み立てるため確かめ、応答の分類と再送しないことは TestRepositoryVariableUpdateOutcome が確かめる。
//
//trace:checks FR-013 INV-005
func TestEnvironmentVariableUpdateOutcome(t *testing.T) {
	const value = "s3cr3t-value-eu"
	create := func(c *Client, t *testing.T) (UpdateOutcome, error) {
		return c.CreateEnvironmentVariable(t.Context(), mustReference(t, "example/project-a"), "production", mustVariableName(t, "REGION"), value)
	}
	update := func(c *Client, t *testing.T) (UpdateOutcome, error) {
		return c.UpdateEnvironmentVariableValue(t.Context(), mustReference(t, "example/project-a"), "production", mustVariableName(t, "REGION"), value)
	}
	cases := []struct {
		name       string
		send       func(*Client, *testing.T) (UpdateOutcome, error)
		status     int
		wantPrefix string
	}{
		{name: "creation error names the operation, the environment, and the variable", send: create, status: http.StatusConflict, wantPrefix: `create environment variable "REGION" in environment "production" example/project-a: `},
		{name: "value update error names the operation, the environment, and the variable", send: update, status: http.StatusNotFound, wantPrefix: `update environment variable "REGION" in environment "production" example/project-a: `},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"` + value + `"}`))
			})

			_, err := tc.send(c, t)
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Fatalf("error = %v, want it to start with %q", err, tc.wantPrefix)
			}
			if strings.Contains(err.Error(), value) {
				t.Errorf("error = %q, want it not to contain the value", err)
			}
		})
	}
}
