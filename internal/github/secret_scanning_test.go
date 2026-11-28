package github

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestDecodeSecretScanning(t *testing.T) {
	successes := []struct {
		name                string
		private             bool
		securityAndAnalysis json.RawMessage
		want                SecretScanning
	}{
		{
			name:                "public enabled secret scanning and disabled push protection are observed, ignoring the items reco does not read",
			securityAndAnalysis: json.RawMessage(repositorySecurityAndAnalysis),
			want:                SecretScanning{Enabled: Observed(true), PushProtection: Observed(false)},
		},
		{
			name:                "public disabled secret scanning and enabled push protection are observed",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "disabled"}, "secret_scanning_push_protection": {"status": "enabled"}}`),
			want:                SecretScanning{Enabled: Observed(false), PushProtection: Observed(true)},
		},
		{
			name:    "private without security_and_analysis is unavailable for the private repository",
			private: true,
			want:    SecretScanning{Enabled: Unavailable[bool](UnavailablePrivateRepository), PushProtection: Unavailable[bool](UnavailablePrivateRepository)},
		},
		{
			name:                "private with values is unavailable without reading them",
			private:             true,
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "enabled"}, "secret_scanning_push_protection": {"status": "enabled"}}`),
			want:                SecretScanning{Enabled: Unavailable[bool](UnavailablePrivateRepository), PushProtection: Unavailable[bool](UnavailablePrivateRepository)},
		},
		{
			// 二つは別々に判定し、片方が知らない値でも、もう片方は観測値を持つ
			name:                "public unknown secret scanning status makes only secret scanning unavailable as an unknown value",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "on"}, "secret_scanning_push_protection": {"status": "enabled"}}`),
			want:                SecretScanning{Enabled: Unavailable[bool](UnavailableUnknownValue), PushProtection: Observed(true)},
		},
		{
			name:                "public unknown push protection status makes only push protection unavailable as an unknown value",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "disabled"}, "secret_scanning_push_protection": {"status": "Enabled"}}`),
			want:                SecretScanning{Enabled: Observed(false), PushProtection: Unavailable[bool](UnavailableUnknownValue)},
		},
		{
			// 公開範囲の判定が先に打ち切るため、知らない値も読まず、理由はリポジトリが非公開であること
			name:                "private with unknown statuses is unavailable for the private repository",
			private:             true,
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "on"}, "secret_scanning_push_protection": {"status": "on"}}`),
			want:                SecretScanning{Enabled: Unavailable[bool](UnavailablePrivateRepository), PushProtection: Unavailable[bool](UnavailablePrivateRepository)},
		},
		{
			name:                "private with a non-object security_and_analysis is unavailable without interpreting it",
			private:             true,
			securityAndAnalysis: json.RawMessage(`"text"`),
			want:                SecretScanning{Enabled: Unavailable[bool](UnavailablePrivateRepository), PushProtection: Unavailable[bool](UnavailablePrivateRepository)},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSecretScanning(tc.private, tc.securityAndAnalysis)
			if err != nil {
				t.Fatalf("decodeSecretScanning() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeSecretScanning() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name                string
		securityAndAnalysis json.RawMessage
		errContains         string
	}{
		{name: "public without security_and_analysis is unexpected response", errContains: "missing fields: security_and_analysis"},
		{name: "public null security_and_analysis is unexpected response", securityAndAnalysis: json.RawMessage(`null`), errContains: "missing fields: security_and_analysis"},
		{
			name:                "public empty security_and_analysis reports both statuses",
			securityAndAnalysis: json.RawMessage(`{}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning.status, security_and_analysis.secret_scanning_push_protection.status",
		},
		{
			name:                "public missing secret_scanning item is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning_push_protection": {"status": "enabled"}}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning.status",
		},
		{
			name:                "public null secret scanning status is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": null}, "secret_scanning_push_protection": {"status": "enabled"}}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning.status",
		},
		{
			name:                "public missing push protection item is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "enabled"}}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning_push_protection.status",
		},
		{
			name:                "public null push protection status is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "enabled"}, "secret_scanning_push_protection": {"status": null}}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning_push_protection.status",
		},
		{
			// 知らない値でも、もう一方の項目が取得契約に適合しなければ観測を続けない
			name:                "public unknown secret scanning status with a missing push protection item is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": "on"}}`),
			errContains:         "missing fields: security_and_analysis.secret_scanning_push_protection.status",
		},
		{
			name:                "public non-string secret scanning status is unexpected response",
			securityAndAnalysis: json.RawMessage(`{"secret_scanning": {"status": 1}, "secret_scanning_push_protection": {"status": "enabled"}}`),
		},
		{name: "public non-object security_and_analysis is unexpected response", securityAndAnalysis: json.RawMessage(`[]`)},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeSecretScanning(false, tc.securityAndAnalysis)
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeSecretScanning() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeSecretScanning() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositorySecretScanning(t *testing.T) {
	t.Run("public repository is observed from the repository response alone", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(repositoryBody),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaSecretScanning)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &SecretScanning{Enabled: Observed(true), PushProtection: Observed(false)}
		if !reflect.DeepEqual(observed.SecretScanning, want) {
			t.Errorf("ObserveRepository().SecretScanning = %+v, want %+v", observed.SecretScanning, want)
		}
		wantLines := []string{"GET " + repositoryPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	t.Run("private repository is unavailable for the private repository", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(privateRepositoryBody),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaSecretScanning)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &SecretScanning{Enabled: Unavailable[bool](UnavailablePrivateRepository), PushProtection: Unavailable[bool](UnavailablePrivateRepository)}
		if !reflect.DeepEqual(observed.SecretScanning, want) {
			t.Errorf("ObserveRepository().SecretScanning = %+v, want %+v", observed.SecretScanning, want)
		}
	})

	// 公開リポジトリで、マージ関連のフィールドは返るが security_and_analysis だけを欠く応答。権限の足りない場合を想定した形で、実際の応答は確かめていない
	const publicWithoutSecurityAndAnalysis = `{"id": 1296269, "allow_auto_merge": true, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": true, "allow_update_branch": false, "squash_merge_commit_title": "COMMIT_OR_PR_TITLE", "squash_merge_commit_message": "COMMIT_MESSAGES", "merge_commit_title": "MERGE_MESSAGE", "merge_commit_message": "PR_TITLE", "has_issues": true, "has_projects": true, "has_wiki": true, "web_commit_signoff_required": false, "private": false, "permissions": {"admin": true}}`

	t.Run("public repository without security_and_analysis fails before the other read requests", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(publicWithoutSecurityAndAnalysis),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot, setting.AreaSecretScanning)
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("ObserveRepository() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if want := "fetch repository owner/repo"; !strings.Contains(err.Error(), want) {
			t.Errorf("ObserveRepository() error = %q, want it to contain %q", err, want)
		}
		if !reflect.DeepEqual(observed, ObservationResult{}) {
			t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
		}
		wantLines := []string{"GET " + repositoryPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	t.Run("observation without the secret scanning area ignores missing security_and_analysis", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(publicWithoutSecurityAndAnalysis),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"))
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if observed.SecretScanning != nil {
			t.Errorf("ObserveRepository().SecretScanning = %+v, want nil", observed.SecretScanning)
		}
	})
}
