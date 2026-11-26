package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

// TestObserveRepositoryFailure は、失敗応答が種類を保って呼び出し元に届き、
// エラー文言にトークン、応答本文、転送先を含めないことを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003 FR-004 QR-001
func TestObserveRepositoryFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "403 with retry-after is rate limited",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, bodyMarker, http.StatusForbidden)
			},
			want: ErrRateLimited,
		},
		{
			name: "301 is redirect and is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			},
			want: ErrRedirect,
		},
		{
			name: "200 without allow_auto_merge is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id": 1, "full_name": "owner/repo", "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": true, "allow_update_branch": false, "private": false}`))
			},
			want: ErrUnexpectedResponse,
		},
		{
			// 上限を超えても JSON として正しい本文にし、上限を見ずに読めば成功してしまう形にする
			name: "200 with oversized body is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(repositoryBody + strings.Repeat(" ", maxResponseBytes)))
			},
			want: ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.handler)
			_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			msg := err.Error()
			if !strings.Contains(msg, "owner/repo") {
				t.Errorf("error = %q, want it to name owner/repo", msg)
			}
			for _, secret := range []string{testToken, bodyMarker, location} {
				if strings.Contains(msg, secret) {
					t.Errorf("error = %q, must not contain %q", msg, secret)
				}
			}
		})
	}
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}

	t.Run("canceled context is communication failure", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(repositoryBody))
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := c.ObserveRepository(ctx, mustReference(t, "owner/repo"))
		if !errors.Is(err, ErrCommunicationFailure) || !errors.Is(err, context.Canceled) {
			t.Fatalf("ObserveRepository() error = %v, want %v wrapping %v", err, ErrCommunicationFailure, context.Canceled)
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("error = %q, must not contain the token", err)
		}
	})
}

// TestObserveRepositoryAreas は、選んだ設定の領域に応じて送る要求と、観測結果への詰め方を確かめる。
// Selected actions を送らない3件は selectedActionsUnavailability の単体テストと条件が重なるが、
// 取得条件に当たるときに要求そのものを送らないことは、要求の列でしか確かめられない。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryAreas(t *testing.T) {
	type request struct {
		method        string
		path          string
		contentLength int64
		header        http.Header
	}
	selectedCriteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"z/*", "a/*"}}
	cases := []struct {
		name string
		// repositoryBody は空のとき公開のリポジトリの repositoryBody を使う
		repositoryBody string
		areas          []setting.Area
		actionsBody    string
		wantPaths      []string
		wantAP         *ActionsPermissions
		wantWP         *WorkflowPermissions
		wantAA         *ActionsAccess
	}{
		{
			name:        "selected policy sends the selected actions GET after the actions permissions",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("selected"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath, selectedActionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsSelected),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Observed(selectedCriteria),
			},
		},
		{
			name:        "all policy does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("all"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsAll),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
		},
		{
			name:        "local_only policy does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("local_only"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsLocalOnly),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
		},
		{
			name:        "disabled actions does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsDisabledBody,
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled),
			},
		},
		{
			name:        "both areas are read in the fixed order regardless of the given order",
			areas:       []setting.Area{setting.AreaWorkflowPermissions, setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("selected"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath, selectedActionsPath, workflowPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsSelected),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Observed(selectedCriteria),
			},
			wantWP: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
		},
		{
			name:      "workflow permissions area sends the second GET after the repository",
			areas:     []setting.Area{setting.AreaWorkflowPermissions},
			wantPaths: []string{repositoryPath, workflowPermissionsPath},
			wantWP:    &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
		},
		{
			name:      "no area observes the repository only",
			wantPaths: []string{repositoryPath},
		},
		{
			name:           "private repository reads all areas in the fixed order with the actions access last",
			repositoryBody: privateRepositoryBody,
			areas:          []setting.Area{setting.AreaActionsAccess, setting.AreaWorkflowPermissions, setting.AreaActionsPermissions},
			actionsBody:    actionsPermissionsBody("all"),
			wantPaths:      []string{repositoryPath, actionsPermissionsPath, workflowPermissionsPath, actionsAccessPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsAll),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
			wantWP: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
			wantAA: &ActionsAccess{AccessLevel: Observed(setting.AccessUser)},
		},
		{
			name:      "public repository does not send the actions access GET and is unavailable",
			areas:     []setting.Area{setting.AreaActionsAccess},
			wantPaths: []string{repositoryPath},
			wantAA:    &ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](UnavailablePublicRepository)},
		},
		{
			name:           "private repository without the actions access area does not send its GET",
			repositoryBody: privateRepositoryBody,
			wantPaths:      []string{repositoryPath},
		},
		{
			name:           "private repository with disabled actions still observes the access level",
			repositoryBody: privateRepositoryBody,
			areas:          []setting.Area{setting.AreaActionsPermissions, setting.AreaActionsAccess},
			actionsBody:    actionsDisabledBody,
			wantPaths:      []string{repositoryPath, actionsPermissionsPath, actionsAccessPath},
			wantAP: &ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled),
			},
			wantAA: &ActionsAccess{AccessLevel: Observed(setting.AccessUser)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []request
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = append(got, request{method: r.Method, path: r.URL.Path, contentLength: r.ContentLength, header: r.Header.Clone()})
				switch r.URL.Path {
				case repositoryPath:
					body := repositoryBody
					if tc.repositoryBody != "" {
						body = tc.repositoryBody
					}
					_, _ = w.Write([]byte(body))
				case actionsPermissionsPath:
					_, _ = w.Write([]byte(tc.actionsBody))
				case selectedActionsPath:
					_, _ = w.Write([]byte(selectedActionsBody))
				case workflowPermissionsPath:
					_, _ = w.Write([]byte(workflowPermissionsBody))
				case actionsAccessPath:
					_, _ = w.Write([]byte(actionsAccessBody))
				default:
					t.Errorf("unexpected request path %q", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepository() error = %v", err)
			}
			wantGeneral := General{AllowAutoMerge: true, DeleteBranchOnMerge: false, AllowSquashMerge: true, AllowMergeCommit: false, AllowRebaseMerge: true, AllowUpdateBranch: false}
			if observed.RepositoryID != 1296269 || observed.General != wantGeneral {
				t.Errorf("ObserveRepository() = %+v, want ID 1296269 and General %+v", observed, wantGeneral)
			}
			if !reflect.DeepEqual(observed.ActionsPermissions, tc.wantAP) {
				t.Errorf("ObserveRepository().ActionsPermissions = %+v, want %+v", observed.ActionsPermissions, tc.wantAP)
			}
			if !reflect.DeepEqual(observed.WorkflowPermissions, tc.wantWP) {
				t.Errorf("ObserveRepository().WorkflowPermissions = %+v, want %+v", observed.WorkflowPermissions, tc.wantWP)
			}
			if !reflect.DeepEqual(observed.ActionsAccess, tc.wantAA) {
				t.Errorf("ObserveRepository().ActionsAccess = %+v, want %+v", observed.ActionsAccess, tc.wantAA)
			}
			if observed.Dependabot != nil {
				t.Errorf("ObserveRepository().Dependabot = %+v, want nil without the dependabot area", observed.Dependabot)
			}
			if observed.ForkPullRequestApproval != nil || observed.PrivateForkPullRequestWorkflows != nil || observed.Retention != nil {
				t.Errorf("ObserveRepository() = %+v, want no fork pull request settings and no retention without their areas", observed)
			}
			if observed.OIDCSubject != nil {
				t.Errorf("ObserveRepository().OIDCSubject = %+v, want nil without the oidc subject area", observed.OIDCSubject)
			}

			if len(got) != len(tc.wantPaths) {
				t.Fatalf("got %d requests %+v, want paths %v", len(got), got, tc.wantPaths)
			}
			for i, r := range got {
				if r.method != http.MethodGet || r.path != tc.wantPaths[i] {
					t.Errorf("request %d = %s %s, want GET %s", i, r.method, r.path, tc.wantPaths[i])
				}
				if r.contentLength != 0 || r.header.Get("Content-Type") != "" {
					t.Errorf("request %d has Content-Length %d and Content-Type %q, want no body", i, r.contentLength, r.header.Get("Content-Type"))
				}
				headers := map[string]string{
					"Authorization":        "Bearer " + testToken,
					"Accept":               "application/vnd.github+json",
					"X-GitHub-Api-Version": "2026-03-10",
					"User-Agent":           userAgent,
				}
				for name, value := range headers {
					if v := r.header.Get(name); v != value {
						t.Errorf("request %d header %s = %q, want %q", i, name, v, value)
					}
				}
			}
		})
	}
}

// TestObserveRepositoryForkPullRequestAndRetentionOrder は、Fork Pull Request と保持期間の領域が、
// 渡した並びによらず、リポジトリ外のワークフローからのアクセスのあと、Repository Variables の前に、公開範囲に合う取得だけを送ることを確かめる。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryForkPullRequestAndRetentionOrder(t *testing.T) {
	s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET " + repositoryPath:                      githubtest.OK(privateRepositoryBody),
		"GET " + actionsAccessPath:                   githubtest.OK(actionsAccessBody),
		"GET " + privateForkPullRequestWorkflowsPath: githubtest.OK(githubtest.PrivateForkPullRequestWorkflowsBody{}.JSON()),
		"GET " + retentionPath:                       githubtest.OK(githubtest.RetentionBody{Days: 90, MaximumAllowedDays: 400}.JSON()),
		"GET " + repositoryVariablesPath:             githubtest.OK(githubtest.VariablesBody{}.JSON()),
	}))
	c := New(testToken, WithBaseURL(s.URL()))

	_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"),
		setting.AreaRepositoryVariables, setting.AreaRetention, setting.AreaPrivateForkPullRequestWorkflows,
		setting.AreaForkPullRequestApproval, setting.AreaActionsAccess)
	if err != nil {
		t.Fatalf("ObserveRepository() error = %v", err)
	}
	wantLines := []string{
		"GET " + repositoryPath, "GET " + actionsAccessPath, "GET " + privateForkPullRequestWorkflowsPath,
		"GET " + retentionPath, "GET " + repositoryVariablesPath,
	}
	if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
		t.Errorf("requests = %q, want %q", got, wantLines)
	}
}

func TestObservationResult_Setting(t *testing.T) {
	// 観測結果ごとに真偽値の設定を一つだけ真にし、どの設定も観測結果を通した値の並びがほかの設定とも定数とも異なるようにする。
	// 別の設定の値や定数を返す誤りを検出するため。
	observations := []ObservationResult{
		{General: General{DeleteBranchOnMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite}},
		{General: General{AllowAutoMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead}},
		{WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true}},
	}
	yes, no := Observed(setting.BoolValue(true)), Observed(setting.BoolValue(false))
	write, read := Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Observed(setting.PermissionLevelValue(setting.PermissionRead))
	cases := []struct {
		key  setting.Key
		want []SettingObservation[setting.Value]
	}{
		{key: setting.DeleteBranchOnMerge, want: []SettingObservation[setting.Value]{yes, no, no}},
		{key: setting.AllowAutoMerge, want: []SettingObservation[setting.Value]{no, yes, no}},
		{key: setting.DefaultWorkflowPermissions, want: []SettingObservation[setting.Value]{write, read, write}},
		{key: setting.CanApprovePullRequestReviews, want: []SettingObservation[setting.Value]{no, no, yes}},
	}
	for _, tc := range cases {
		t.Run(string(tc.key)+" returns its observed value", func(t *testing.T) {
			for i, observed := range observations {
				if got := observed.Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want[i]) {
					t.Errorf("observations[%d].Setting(%q) = %v, want %v", i, tc.key, got, tc.want[i])
				}
			}
		})
	}
}

func TestObservationResult_SettingOfGeneral(t *testing.T) {
	// 観測結果ごとに General の設定を一つだけ真にし、別の設定の値を返す誤りを検出する。
	observations := []ObservationResult{
		{General: General{AllowSquashMerge: true}},
		{General: General{AllowMergeCommit: true}},
		{General: General{AllowRebaseMerge: true}},
		{General: General{AllowUpdateBranch: true}},
	}
	keys := []setting.Key{setting.AllowSquashMerge, setting.AllowMergeCommit, setting.AllowRebaseMerge, setting.AllowUpdateBranch}
	for k, key := range keys {
		t.Run(string(key)+" returns its observed value", func(t *testing.T) {
			for i, observed := range observations {
				want := Observed(setting.BoolValue(i == k))
				if got := observed.Setting(setting.Unit{Key: key}); !reflect.DeepEqual(got, want) {
					t.Errorf("observations[%d].Setting(%q) = %v, want %v", i, key, got, want)
				}
			}
		})
	}
}

//trace:checks FR-004 FR-005
func TestObservationResult_SettingOfDependabot(t *testing.T) {
	cases := []struct {
		name     string
		observed Dependabot
		key      setting.Key
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "dependabot_alerts returns the observed enablement as a boolean",
			observed: Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesDisabled},
			key:      setting.DependabotAlerts,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "dependabot_security_updates keeps the paused state",
			observed: Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesPaused},
			key:      setting.DependabotSecurityUpdates,
			want:     Observed(setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesPaused)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := ObservationResult{Dependabot: &tc.observed}
			if got := observed.Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 FR-005
func TestObservationResult_SettingOfSecretScanning(t *testing.T) {
	private := Unavailable[bool](UnavailablePrivateRepository)
	cases := []struct {
		name     string
		observed SecretScanning
		key      setting.Key
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "secret_scanning returns its own observed value",
			observed: SecretScanning{Enabled: Observed(true), PushProtection: Observed(false)},
			key:      setting.SecretScanningEnabled,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "push protection returns its own observed value",
			observed: SecretScanning{Enabled: Observed(true), PushProtection: Observed(false)},
			key:      setting.RepositoryPushProtection,
			want:     Observed(setting.BoolValue(false)),
		},
		{
			name:     "secret_scanning on a private repository keeps the unavailable reason",
			observed: SecretScanning{Enabled: private, PushProtection: private},
			key:      setting.SecretScanningEnabled,
			want:     Unavailable[setting.Value](UnavailablePrivateRepository),
		},
		{
			name:     "push protection on a private repository keeps the unavailable reason",
			observed: SecretScanning{Enabled: private, PushProtection: private},
			key:      setting.RepositoryPushProtection,
			want:     Unavailable[setting.Value](UnavailablePrivateRepository),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := ObservationResult{SecretScanning: &tc.observed}
			if got := observed.Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 FR-005
func TestObservationResult_SettingOfPrivateVulnerabilityReporting(t *testing.T) {
	cases := []struct {
		name     string
		observed PrivateVulnerabilityReporting
		want     SettingObservation[setting.Value]
	}{
		{name: "observed enablement is a boolean", observed: PrivateVulnerabilityReporting{Enabled: Observed(true)}, want: Observed(setting.BoolValue(true))},
		{
			name:     "private repository keeps the unavailable reason",
			observed: PrivateVulnerabilityReporting{Enabled: Unavailable[bool](UnavailablePrivateRepository)},
			want:     Unavailable[setting.Value](UnavailablePrivateRepository),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := ObservationResult{PrivateVulnerabilityReporting: &tc.observed}
			if got := observed.Setting(setting.Unit{Key: setting.PrivateVulnerabilityReportingEnabled}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfActionsPermissions(t *testing.T) {
	// 観測値の設定グループの許可パターンは、応答の並びと重複のまま運ばれることを確かめるため、順不同で重複を含める。
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"b/*", "a/*", "b/*"}}
	cases := []struct {
		name     string
		observed ActionsPermissions
		key      setting.Key
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "enabled returns true observed for enabled actions",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.ActionsEnabled,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "enabled returns false observed, not unavailable, for disabled actions",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.ActionsEnabled,
			want:     Observed(setting.BoolValue(false)),
		},
		{
			name:     "allowed_actions returns the observed policy",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsLocalOnly), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.AllowedActions,
			want:     Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly)),
		},
		{
			name:     "allowed_actions keeps the unavailable reason",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.AllowedActions,
			want:     Unavailable[setting.Value](UnavailableActionsDisabled),
		},
		{
			name:     "sha_pinning_required returns the observed value",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SHAPinningRequired: Observed(true), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.SHAPinningRequired,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "sha_pinning_required keeps the unavailable reason",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.SHAPinningRequired,
			want:     Unavailable[setting.Value](UnavailableActionsDisabled),
		},
		{
			name:     "selected_actions returns the observed criteria as is",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsSelected), SelectedActions: Observed(criteria)},
			key:      setting.SelectedActions,
			want:     Observed(setting.SelectedActionsCriteriaValue(criteria)),
		},
		{
			name:     "selected_actions keeps the unavailable reason",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.SelectedActions,
			want:     Unavailable[setting.Value](UnavailableNotSelected),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{ActionsPermissions: &tc.observed}).Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", tc.key, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfActionsAccess(t *testing.T) {
	cases := []struct {
		name     string
		observed ActionsAccess
		want     SettingObservation[setting.Value]
	}{
		{name: "access_level returns the observed level", observed: ActionsAccess{AccessLevel: Observed(setting.AccessUser)}, want: Observed(setting.AccessLevelValue(setting.AccessUser))},
		{name: "access_level keeps the unavailable reason", observed: ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](UnavailablePublicRepository)}, want: Unavailable[setting.Value](UnavailablePublicRepository)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{ActionsAccess: &tc.observed}).Setting(setting.Unit{Key: setting.ActionsAccessLevel}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", setting.ActionsAccessLevel, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfForkPullRequestApproval(t *testing.T) {
	cases := []struct {
		name     string
		observed ForkPullRequestApproval
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "approval_policy returns the observed policy",
			observed: ForkPullRequestApproval{ApprovalPolicy: Observed(setting.ForkPullRequestApprovalFirstTimeContributors)},
			want:     Observed(setting.ForkPullRequestApprovalPolicyValue(setting.ForkPullRequestApprovalFirstTimeContributors)),
		},
		{
			name:     "approval_policy keeps the unavailable reason",
			observed: ForkPullRequestApproval{ApprovalPolicy: Unavailable[setting.ForkPullRequestApprovalPolicy](UnavailablePrivateRepository)},
			want:     Unavailable[setting.Value](UnavailablePrivateRepository),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{ForkPullRequestApproval: &tc.observed}).Setting(setting.Unit{Key: setting.ForkPullRequestApproval}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", setting.ForkPullRequestApproval, got, tc.want)
			}
		})
	}
}

// TestObservationResult_SettingOfRetentionDays は、保持日数の観測値を、上限と取り違えずに返すことを確かめる。
//
//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfRetentionDays(t *testing.T) {
	got := (ObservationResult{Retention: &Retention{Days: 30, MaximumAllowedDays: 400}}).Setting(setting.Unit{Key: setting.RetentionDays})
	if want := Observed(setting.RetentionDaysValue(30)); !reflect.DeepEqual(got, want) {
		t.Errorf("Setting(%q) = %+v, want %+v", setting.RetentionDays, got, want)
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfPrivateForkPullRequestWorkflows(t *testing.T) {
	conditions := setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}
	cases := []struct {
		name     string
		observed PrivateForkPullRequestWorkflows
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "conditions return the observed four items",
			observed: PrivateForkPullRequestWorkflows{Conditions: Observed(conditions)},
			want:     Observed(setting.PrivateForkPullRequestWorkflowConditionsValue(conditions)),
		},
		{
			name:     "conditions keep the unavailable reason",
			observed: PrivateForkPullRequestWorkflows{Conditions: Unavailable[setting.PrivateForkPullRequestWorkflowConditions](UnavailablePublicRepository)},
			want:     Unavailable[setting.Value](UnavailablePublicRepository),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{PrivateForkPullRequestWorkflows: &tc.observed}).Setting(setting.Unit{Key: setting.PrivateForkPullRequestWorkflows}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", setting.PrivateForkPullRequestWorkflows, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestObservationResult_SettingOfEnvironmentDeploymentBranchPolicy(t *testing.T) {
	// 別の Environment の方式を返す誤りと、宣言の名前を綴りのまま比べる誤りを検出するため、
	// 方式の異なる二つを並べ、ASCII の大小が違う宣言の名前で引く。
	patterns := []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}, {Type: setting.DeploymentPatternTag, Name: "v*"}}
	selected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns}
	observed := ObservationResult{Environments: &Environments{Existing: []Environment{
		{ID: 1, Name: "staging", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyAll},
		{ID: 2, Name: "Production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected, DeploymentPatterns: &DeploymentPatterns{Existing: patterns}},
	}}}
	cases := []struct {
		name string
		unit setting.Unit
		want SettingObservation[setting.Value]
	}{
		{
			name: "declared name differing in ASCII case returns that environment's policy with its patterns",
			unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "production")},
			want: Observed(setting.DeploymentBranchPolicyValue(selected)),
		},
		{
			name: "absent environment returns UnavailableEnvironmentAbsent",
			unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "qa")},
			want: Unavailable[setting.Value](UnavailableEnvironmentAbsent),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Setting(tc.unit); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%v) = %+v, want %+v", tc.unit.String(), got, tc.want)
			}
		})
	}
}

// 別の Environment の担当者を返す誤りと、宣言の名前を綴りのまま比べる誤りを検出するため、
// 担当者の異なる二つを並べ、ASCII の大小が違う宣言の名前で引く。
//
//trace:checks FR-004
func TestObservationResult_SettingOfEnvironmentRequiredReviewers(t *testing.T) {
	reviewers := []setting.Reviewer{{Type: setting.ReviewerUser, ID: 2}}
	observed := ObservationResult{Environments: &Environments{Existing: []Environment{
		{ID: 1, Name: "staging", Protection: setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{Reviewers: []setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}}}}},
		{ID: 2, Name: "Production", Protection: setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{Reviewers: reviewers}}},
	}}}
	cases := []struct {
		name string
		unit setting.Unit
		want SettingObservation[setting.Value]
	}{
		{
			name: "declared name differing in ASCII case returns that environment's reviewers",
			unit: setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: mustEnvironmentName(t, "production")},
			want: Observed(setting.ReviewerSetValue(setting.ReviewerSetOf(reviewers))),
		},
		{
			name: "absent environment returns UnavailableEnvironmentAbsent",
			unit: setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: mustEnvironmentName(t, "qa")},
			want: Unavailable[setting.Value](UnavailableEnvironmentAbsent),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Setting(tc.unit); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%v) = %+v, want %+v", tc.unit.String(), got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestObservationResult_SettingOfRepositoryVariable(t *testing.T) {
	observed := ObservationResult{RepositoryVariables: &Variables{Existing: []Variable{{Name: "GO_VERSION", Value: "1.27"}}}}
	unit := setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}
	if got, want := observed.Setting(unit), Observed(setting.VariableValue("1.27")); !reflect.DeepEqual(got, want) {
		t.Errorf("Setting(%v) = %+v, want %+v", unit.String(), got, want)
	}
}

// 同じ名前の Repository Variable を引く誤りと、Environment の名前を使わずに最初の Environment を引く誤りを検出するため、
// 同じ名前で値の異なる変数を、Repository と二つの Environment に置く。
//
//trace:checks FR-004
func TestObservationResult_SettingOfEnvironmentVariable(t *testing.T) {
	observed := ObservationResult{
		RepositoryVariables: &Variables{Existing: []Variable{{Name: "REGION", Value: "repository"}}},
		Environments: &Environments{Existing: []Environment{
			{ID: 1, Name: "staging", Variables: &Variables{Existing: []Variable{{Name: "REGION", Value: "staging"}}}},
			{ID: 2, Name: "production", Variables: &Variables{Existing: []Variable{{Name: "REGION", Value: "production"}}}},
		}},
	}
	unit := setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, "production"), Variable: mustVariableName(t, "REGION")}
	if got, want := observed.Setting(unit), Observed(setting.VariableValue("production")); !reflect.DeepEqual(got, want) {
		t.Errorf("Setting(%v) = %+v, want %+v", unit.String(), got, want)
	}
}

// Environment の名前を使わずに最初の Environment を引く誤りを検出するため、別の名前の Environment を先に置く。
//
//trace:checks FR-004
func TestObservationResult_SettingOfEnvironmentResource(t *testing.T) {
	observed := ObservationResult{Environments: &Environments{Existing: []Environment{{ID: 1, Name: "staging"}, {ID: 2, Name: "Production"}}}}
	cases := []struct {
		name string
		unit setting.Unit
		want SettingObservation[setting.Value]
	}{
		{
			name: "declared name differing in ASCII case observes the environment as present",
			unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")},
			want: Observed(setting.Present()),
		},
		{
			name: "unlisted name observes the environment as absent",
			unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "qa")},
			want: Observed(setting.Absent()),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Setting(tc.unit); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%v) = %+v, want %+v", tc.unit.String(), got, tc.want)
			}
		})
	}
}

// Ruleset の名前を使わずに最初の Ruleset を引く誤りを検出するため、別の名前の Ruleset を先に置く。
//
//trace:checks FR-004
func TestObservationResult_SettingOfRulesetResource(t *testing.T) {
	observed := ObservationResult{Rulesets: &Rulesets{Existing: Observed([]Ruleset{
		InterpretedRuleset(1, "release", setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled}),
		InterpretedRuleset(2, "Protect", interpretedRulesetValue()),
	})}}
	cases := []struct {
		name string
		unit setting.Unit
		want SettingObservation[setting.Value]
	}{
		{
			name: "declared name differing in ASCII case observes the value of the ruleset",
			unit: setting.Unit{Key: setting.RulesetResource, Ruleset: mustRulesetName(t, "protect")},
			want: Observed(setting.RulesetValue(interpretedRulesetValue())),
		},
		{
			name: "unlisted name observes the ruleset as absent",
			unit: setting.Unit{Key: setting.RulesetResource, Ruleset: mustRulesetName(t, "tags")},
			want: Observed(setting.Absent()),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Setting(tc.unit); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%v) = %+v, want %+v", tc.unit.String(), got, tc.want)
			}
		})
	}
}

func TestObservationResult_SettingOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Setting() did not panic for an unknown setting")
		}
	}()
	_ = ObservationResult{}.Setting(setting.Unit{Key: "general.unknown"})
}

// TestObservationResult_SettingOfOIDCSubject は、観測した3項目を落とさずに値にし、immutable を持たない観測を不変条件の破れとして止めることを確かめる。
//
//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfOIDCSubject(t *testing.T) {
	unit := setting.Unit{Key: setting.OIDCSubject}
	t.Run("observation keeps all three items", func(t *testing.T) {
		customization := setting.OIDCSubjectCustomization{IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: new(true)}
		got := (ObservationResult{OIDCSubject: &OIDCSubject{Customization: customization}}).Setting(unit)
		if want := Observed(setting.OIDCSubjectCustomizationValue(customization)); !reflect.DeepEqual(got, want) {
			t.Errorf("Setting(%q) = %+v, want %+v", setting.OIDCSubject, got, want)
		}
	})
	t.Run("observation without immutable panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Setting() did not panic for an observation without use_immutable_subject")
			}
		}()
		_ = (ObservationResult{OIDCSubject: &OIDCSubject{Customization: setting.OIDCSubjectCustomization{UseDefault: true}}}).Setting(unit)
	})
}
