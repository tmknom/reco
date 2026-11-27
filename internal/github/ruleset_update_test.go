package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks INV-001 FR-011
func TestEncodeRulesetCreation(t *testing.T) {
	pullRequest := setting.PullRequestRule(setting.PullRequestParameters{
		RequiredApprovingReviewCount:               1,
		DismissStaleReviewsOnPush:                  true,
		RequireCodeOwnerReview:                     false,
		RequireLastPushApproval:                    true,
		RequiredReviewThreadResolution:             false,
		AllowedMergeMethods:                        []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodMerge},
		RequireExtraApprovalForUnattributedChanges: new(false),
	})
	statusChecks := setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{
		RequiredStatusChecks:             []setting.StatusCheck{{Context: "test"}, {Context: "lint", IntegrationID: 15368}},
		StrictRequiredStatusChecksPolicy: true,
	})
	full := setting.Ruleset{
		Target:      setting.RulesetTargetBranch,
		Enforcement: setting.RulesetEnforcementActive,
		Conditions:  setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH", "refs/heads/release"}, Exclude: []string{"refs/heads/tmp"}}},
		BypassActors: []setting.BypassActor{
			{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways},
			{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways},
		},
		Rules: []setting.RulesetRule{pullRequest, setting.DeletionRule(), setting.NonFastForwardRule(), statusChecks},
	}
	cases := []struct {
		name    string
		ruleset string
		value   setting.Ruleset
		// want は、キーの並びとエスケープの形まで固定するため、本文の JSON の文字列で持つ。
		want string
	}{
		{
			name:    "sends every field in the declared order, with null actor_id for DeployKey and the declared pull_request parameters",
			ruleset: "protect",
			value:   full,
			want: `{"name":"protect","target":"branch","enforcement":"active",` +
				`"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH","refs/heads/release"],"exclude":["refs/heads/tmp"]}},` +
				`"bypass_actors":[{"actor_type":"RepositoryRole","actor_id":5,"bypass_mode":"always"},{"actor_type":"DeployKey","actor_id":null,"bypass_mode":"always"}],` +
				`"rules":[{"type":"pull_request","parameters":{"required_approving_review_count":1,"dismiss_stale_reviews_on_push":true,"require_code_owner_review":false,"require_last_push_approval":true,"required_review_thread_resolution":false,"allowed_merge_methods":["squash","merge"],"require_extra_approval_for_unattributed_changes":false}},{"type":"deletion"},{"type":"non_fast_forward"},` +
				`{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"test"},{"context":"lint","integration_id":15368}],"strict_required_status_checks_policy":true}}]}`,
		},
		{
			name:    "nil collections are sent as empty arrays",
			ruleset: "protect",
			value: setting.Ruleset{
				Target:      setting.RulesetTargetBranch,
				Enforcement: setting.RulesetEnforcementDisabled,
				Conditions:  setting.RulesetConditions{RefName: &setting.RefNameCondition{}},
			},
			want: `{"name":"protect","target":"branch","enforcement":"disabled","conditions":{"ref_name":{"include":[],"exclude":[]}},"bypass_actors":[],"rules":[]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeRulesetCreation(mustRulesetName(t, tc.ruleset), tc.value)); got != tc.want {
				t.Errorf("encodeRulesetCreation() = %s, want %s", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001 FR-011
func TestEncodeRulesetValueUpdate(t *testing.T) {
	pullRequest := setting.PullRequestRule(setting.PullRequestParameters{
		RequiredApprovingReviewCount:               1,
		DismissStaleReviewsOnPush:                  true,
		RequireCodeOwnerReview:                     false,
		RequireLastPushApproval:                    true,
		RequiredReviewThreadResolution:             false,
		AllowedMergeMethods:                        []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodMerge},
		RequireExtraApprovalForUnattributedChanges: new(false),
	})
	statusChecks := setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{
		RequiredStatusChecks:             []setting.StatusCheck{{Context: "test"}, {Context: "lint", IntegrationID: 15368}},
		StrictRequiredStatusChecksPolicy: true,
	})
	full := setting.Ruleset{
		Target:      setting.RulesetTargetBranch,
		Enforcement: setting.RulesetEnforcementActive,
		Conditions:  setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH", "refs/heads/release"}, Exclude: []string{"refs/heads/tmp"}}},
		BypassActors: []setting.BypassActor{
			{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways},
			{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways},
		},
		Rules: []setting.RulesetRule{pullRequest, setting.DeletionRule(), setting.NonFastForwardRule(), statusChecks},
	}
	cases := []struct {
		name  string
		value setting.Ruleset
		// want は、キーの並びとエスケープの形まで固定するため、本文の JSON の文字列で持つ。
		want string
	}{
		{
			name:  "sends every value field in the declared order without name, with null actor_id for DeployKey and the declared pull_request parameters",
			value: full,
			want: `{"target":"branch","enforcement":"active",` +
				`"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH","refs/heads/release"],"exclude":["refs/heads/tmp"]}},` +
				`"bypass_actors":[{"actor_type":"RepositoryRole","actor_id":5,"bypass_mode":"always"},{"actor_type":"DeployKey","actor_id":null,"bypass_mode":"always"}],` +
				`"rules":[{"type":"pull_request","parameters":{"required_approving_review_count":1,"dismiss_stale_reviews_on_push":true,"require_code_owner_review":false,"require_last_push_approval":true,"required_review_thread_resolution":false,"allowed_merge_methods":["squash","merge"],"require_extra_approval_for_unattributed_changes":false}},{"type":"deletion"},{"type":"non_fast_forward"},` +
				`{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"test"},{"context":"lint","integration_id":15368}],"strict_required_status_checks_policy":true}}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(encodeRulesetValueUpdate(mustRulesetName(t, "protect"), tc.value)); got != tc.want {
				t.Errorf("encodeRulesetValueUpdate() = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestEncodeRulesetRule は、ルールの本文が、宣言した省略できるパラメータと integration_id だけを送ることを確かめる。
//
//trace:checks INV-001 FR-011
func TestEncodeRulesetRule(t *testing.T) {
	cases := []struct {
		name string
		rule setting.RulesetRule
		want string
	}{
		{
			name: "declared do_not_enforce_on_create false is sent",
			rule: setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{RequiredStatusChecks: []setting.StatusCheck{}, DoNotEnforceOnCreate: new(false)}),
			want: `{"type":"required_status_checks","parameters":{"required_status_checks":[],"strict_required_status_checks_policy":false,"do_not_enforce_on_create":false}}`,
		},
		{
			name: "nil checks are sent as an empty array",
			rule: setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{StrictRequiredStatusChecksPolicy: true}),
			want: `{"type":"required_status_checks","parameters":{"required_status_checks":[],"strict_required_status_checks_policy":true}}`,
		},
		{
			name: "pull_request omitting the optional parameters sends only the five required ones",
			rule: setting.PullRequestRule(setting.PullRequestParameters{RequiredApprovingReviewCount: 1}),
			want: `{"type":"pull_request","parameters":{"required_approving_review_count":1,"dismiss_stale_reviews_on_push":false,"require_code_owner_review":false,"require_last_push_approval":false,"required_review_thread_resolution":false}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(mustMarshalJSON(encodeRulesetRule(tc.rule))); got != tc.want {
				t.Errorf("encodeRulesetRule() = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestEncodeRulesetCreationPanics は、宣言から作られない値を補って送らず、不変条件の破れとして止めることを確かめる。
func TestEncodeRulesetCreationPanics(t *testing.T) {
	cases := []struct {
		name  string
		value setting.Ruleset
	}{
		{name: "conditions without ref_name panics", value: setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive}},
		{name: "undefined rule type panics", value: setting.Ruleset{
			Target:      setting.RulesetTargetBranch,
			Enforcement: setting.RulesetEnforcementActive,
			Conditions:  setting.RulesetConditions{RefName: &setting.RefNameCondition{}},
			Rules:       []setting.RulesetRule{{}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("encodeRulesetCreation() did not panic")
				}
			}()
			encodeRulesetCreation(mustRulesetName(t, "protect"), tc.value)
		})
	}
}

//trace:checks INV-001 FR-011 INV-005
func TestCreateRuleset(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{name: "201 with a body that meets the read contract succeeds", body: rulesetDetail(t, nil)},
		{name: "201 with a body that has uninterpreted parts succeeds", body: rulesetDetail(t, func(body map[string]any) {
			body["rules"] = []any{map[string]any{"type": "creation"}}
		})},
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
				_, _ = w.Write(tc.body)
			})
			name, value := mustRulesetName(t, "protect"), interpretedRulesetValue()

			outcome, err := c.CreateRuleset(t.Context(), mustReference(t, "owner/repo"), name, value)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("CreateRuleset() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPost || gotPath != "/repos/owner/repo/rulesets" || gotRawQuery != "" {
				t.Errorf("request = %s %s?%s, want POST /repos/owner/repo/rulesets without a query", gotMethod, gotPath, gotRawQuery)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if want := string(encodeRulesetCreation(name, value)); string(gotBody) != want {
				t.Errorf("request body = %s, want %s", gotBody, want)
			}
		})
	}
}

// TestCreateRulesetOutcome は、作成の応答の分類と、要求を送り直さないこと、GitHub が返した本文を文言に入れないことを確かめる。
//
//trace:checks FR-013 INV-005 QR-001
func TestCreateRulesetOutcome(t *testing.T) {
	const bodyMarker = "Name must be unique"
	cases := []struct {
		name        string
		handler     http.HandlerFunc
		wantOutcome UpdateOutcome
		wantErr     error
	}{
		{
			name: "422 is rejected without the response body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":["` + bodyMarker + `"]}`))
			},
			wantOutcome: UpdateRejected,
			wantErr:     ErrUnprocessable,
		},
		{
			name: "200 is unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(rulesetDetail(t, nil))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
		{
			name: "201 with a body missing fields is unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{}`))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				tc.handler(w, r)
			})

			outcome, err := c.CreateRuleset(t.Context(), mustReference(t, "example/project-a"), mustRulesetName(t, "protect"), interpretedRulesetValue())

			if outcome != tc.wantOutcome {
				t.Errorf("CreateRuleset() outcome = %v, want %v", outcome, tc.wantOutcome)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateRuleset() error = %v, want %v", err, tc.wantErr)
			}
			if want := `create ruleset "protect" example/project-a: `; !strings.HasPrefix(err.Error(), want) {
				t.Errorf("error = %q, want prefix %q", err, want)
			}
			if strings.Contains(err.Error(), bodyMarker) {
				t.Errorf("error = %q, must not contain the response body", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}

//trace:checks INV-001 FR-011 INV-005
func TestUpdateRulesetValue(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{name: "200 with a body that meets the read contract succeeds", body: rulesetDetail(t, nil)},
		{name: "200 with a body that has uninterpreted parts succeeds", body: rulesetDetail(t, func(body map[string]any) {
			body["rules"] = []any{map[string]any{"type": "creation"}}
		})},
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
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(tc.body)
			})
			name, value := mustRulesetName(t, "protect"), interpretedRulesetValue()

			outcome, err := c.UpdateRulesetValue(t.Context(), mustReference(t, "owner/repo"), 42, name, value)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateRulesetValue() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPut || gotPath != "/repos/owner/repo/rulesets/42" || gotRawQuery != "" {
				t.Errorf("request = %s %s?%s, want PUT /repos/owner/repo/rulesets/42 without a query", gotMethod, gotPath, gotRawQuery)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if want := string(encodeRulesetValueUpdate(name, value)); string(gotBody) != want {
				t.Errorf("request body = %s, want %s", gotBody, want)
			}
		})
	}
}

// TestUpdateRulesetValueOutcome は、値の変更の応答の分類と、要求を送り直さないこと、GitHub が返した本文を文言に入れないことを確かめる。
//
//trace:checks FR-013 INV-005 QR-001
func TestUpdateRulesetValueOutcome(t *testing.T) {
	const bodyMarker = "Name must be unique"
	cases := []struct {
		name        string
		handler     http.HandlerFunc
		wantOutcome UpdateOutcome
		wantErr     error
	}{
		{
			name: "422 is rejected without the response body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"message":"Validation Failed","errors":["` + bodyMarker + `"]}`))
			},
			wantOutcome: UpdateRejected,
			wantErr:     ErrUnprocessable,
		},
		{
			name: "201 is unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(rulesetDetail(t, nil))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
		{
			name: "200 with a body missing fields is unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				tc.handler(w, r)
			})

			outcome, err := c.UpdateRulesetValue(t.Context(), mustReference(t, "example/project-a"), 42, mustRulesetName(t, "protect"), interpretedRulesetValue())

			if outcome != tc.wantOutcome {
				t.Errorf("UpdateRulesetValue() outcome = %v, want %v", outcome, tc.wantOutcome)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UpdateRulesetValue() error = %v, want %v", err, tc.wantErr)
			}
			if want := `update ruleset "protect" example/project-a: `; !strings.HasPrefix(err.Error(), want) {
				t.Errorf("error = %q, want prefix %q", err, want)
			}
			if strings.Contains(err.Error(), bodyMarker) {
				t.Errorf("error = %q, must not contain the response body", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}
