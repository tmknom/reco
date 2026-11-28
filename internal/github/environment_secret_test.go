package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
)

//trace:checks FR-004 QR-002
func TestDecodeEnvironmentSecret(t *testing.T) {
	successes := []struct {
		name string
		data string
		want EnvironmentSecret
	}{
		{
			name: "name and updated_at become the secret",
			data: `{"name": "DEPLOY_KEY", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-02T03:04:05Z"}`,
			want: EnvironmentSecret{Name: "DEPLOY_KEY", UpdatedAt: "2026-01-02T03:04:05Z"},
		},
		{
			// updated_at は比べるだけで解釈しないため、時刻の形でなくても GitHub が返した文字列のまま持つ
			name: "created_at is not required and unknown fields are ignored",
			data: `{"name": "TOKEN", "updated_at": "later", "unknown": {"nested": true}}`,
			want: EnvironmentSecret{Name: "TOKEN", UpdatedAt: "later"},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeEnvironmentSecret(json.RawMessage(tc.data))
			if err != nil {
				t.Fatalf("decodeEnvironmentSecret() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("decodeEnvironmentSecret() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		data        string
		errContains string
	}{
		{name: "missing name is unexpected response", data: `{"updated_at": "2026-01-02T03:04:05Z"}`, errContains: "missing fields: name"},
		{name: "null name is unexpected response", data: `{"name": null, "updated_at": "2026-01-02T03:04:05Z"}`, errContains: "missing fields: name"},
		{name: "empty name is unexpected response", data: `{"name": "", "updated_at": "2026-01-02T03:04:05Z"}`, errContains: "unsupported value: name"},
		{name: "missing updated_at is unexpected response", data: `{"name": "TOKEN", "created_at": "2026-01-01T00:00:00Z"}`, errContains: "missing fields: updated_at"},
		{name: "empty updated_at is unexpected response", data: `{"name": "TOKEN", "updated_at": ""}`, errContains: "unsupported value: updated_at"},
		{name: "null element names both", data: `null`, errContains: "missing fields: name, updated_at"},
		{name: "numeric name is unexpected response", data: `{"name": 1, "updated_at": "2026-01-02T03:04:05Z"}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEnvironmentSecret(json.RawMessage(tc.data))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeEnvironmentSecret() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeEnvironmentSecret() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveEnvironmentSecrets(t *testing.T) {
	t.Run("the pages are joined in the response order from the escaped path of the environment", func(t *testing.T) {
		const secretsPath = environmentsPath + "/Prod%2FEu/secrets"
		var requests []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
			if r.URL.EscapedPath() != secretsPath {
				t.Errorf("got request %s, want %s", r.URL.EscapedPath(), secretsPath)
			}
			switch r.URL.Query().Get("page") {
			case "1":
				_, _ = w.Write([]byte(environmentSecretsBody(3, "TOKEN", "DEPLOY_KEY")))
			case "2":
				_, _ = w.Write([]byte(environmentSecretsBody(3, "API_KEY")))
			default:
				_, _ = w.Write([]byte(environmentSecretsBody(3)))
			}
		})

		got, err := c.observeEnvironmentSecrets(t.Context(), mustReference(t, "owner/repo"), "Prod/Eu")
		if err != nil {
			t.Fatalf("observeEnvironmentSecrets() error = %v", err)
		}
		want := EnvironmentSecrets{Existing: []EnvironmentSecret{
			{Name: "TOKEN", UpdatedAt: secretUpdatedAt}, {Name: "DEPLOY_KEY", UpdatedAt: secretUpdatedAt}, {Name: "API_KEY", UpdatedAt: secretUpdatedAt},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("observeEnvironmentSecrets() = %+v, want %+v", got, want)
		}
		wantRequests := []string{secretsPath + firstPage, secretsPath + "?per_page=100&page=2"}
		if !reflect.DeepEqual(requests, wantRequests) {
			t.Errorf("requests = %q, want %q", requests, wantRequests)
		}
	})

	t.Run("no secrets is an empty list", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			environmentsPath + "/production/secrets": githubtest.OK(environmentSecretsBody(0)),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		got, err := c.observeEnvironmentSecrets(t.Context(), mustReference(t, "owner/repo"), "production")
		if err != nil {
			t.Fatalf("observeEnvironmentSecrets() error = %v", err)
		}
		if want := (EnvironmentSecrets{Existing: []EnvironmentSecret{}}); !reflect.DeepEqual(got, want) {
			t.Errorf("observeEnvironmentSecrets() = %+v, want %+v", got, want)
		}
	})

	failures := []struct {
		name     string
		response githubtest.Response
		wantErr  error
		contains string
	}{
		{
			// 404 を Secret がないと読むと、削除で失われる Secret を示さないまま承認させるため
			name:     "404 is not found, not an empty list",
			response: githubtest.Response{Status: http.StatusNotFound, Body: `{"message": "Not Found"}`},
			wantErr:  ErrNotFound,
		},
		{
			name:     "names that differ only in ASCII case are unexpected response naming the duplicate",
			response: githubtest.OK(environmentSecretsBody(2, "TOKEN", "token")),
			wantErr:  ErrUnexpectedResponse,
			contains: `duplicate secret name "token"`,
		},
		{
			name:     "an element out of the contract is unexpected response",
			response: githubtest.OK(`{"total_count": 1, "secrets": [{"name": "TOKEN"}]}`),
			wantErr:  ErrUnexpectedResponse,
			contains: "missing fields: updated_at",
		},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{environmentsPath + "/production/secrets": tc.response}}
			c := newTestClient(t, routes.ServeHTTP)

			_, err := c.observeEnvironmentSecrets(t.Context(), mustReference(t, "owner/repo"), "production")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("observeEnvironmentSecrets() error = %v, want %v", err, tc.wantErr)
			}
			if want := `secrets of environment "production": `; !strings.HasPrefix(err.Error(), want) {
				t.Errorf("observeEnvironmentSecrets() error = %q, want prefix %q", err, want)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("observeEnvironmentSecrets() error = %q, want it to contain %q", err, tc.contains)
			}
		})
	}

	t.Run("an environment named as a dot segment is unexpected response without a request", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{}}
		c := newTestClient(t, routes.ServeHTTP)

		_, err := c.observeEnvironmentSecrets(t.Context(), mustReference(t, "owner/repo"), ".")
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("observeEnvironmentSecrets() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if got := routes.Requests(); len(got) != 0 {
			t.Errorf("requests = %q, want none", got)
		}
	})
}
