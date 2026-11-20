package github

import (
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// environmentJSON は一覧の要素を作る。fields の値で既定の項目を置き換え、値が空文字列の項目は含めない。
func environmentJSON(fields map[string]string) string {
	values := map[string]string{
		"id":                       "161088068",
		"name":                     `"production"`,
		"can_admins_bypass":        "false",
		"protection_rules":         "[]",
		"deployment_branch_policy": "null",
	}
	maps.Copy(values, fields)
	var parts []string
	for _, k := range []string{"id", "name", "can_admins_bypass", "protection_rules", "deployment_branch_policy"} {
		if values[k] != "" {
			parts = append(parts, `"`+k+`": `+values[k])
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeEnvironment(t *testing.T) {
	// actualShape は、reco が読まない項目 (node_id、url、reviewer.login など) と branch_policy の保護ルールを含む、実際の応答の形。
	const actualShape = `{
		"id": 161088068,
		"node_id": "EN_kwDOAAAAAc4JiGZE",
		"name": "Prod/Eu",
		"url": "https://api.github.com/repos/owner/repo/environments/Prod%2FEu",
		"html_url": "https://github.com/owner/repo/deployments/activity_log?environments_filter=Prod%2FEu",
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-02T00:00:00Z",
		"can_admins_bypass": true,
		"protection_rules": [
			{"id": 1, "node_id": "GA_1", "type": "wait_timer", "wait_timer": 30},
			{"id": 2, "node_id": "GA_2", "type": "required_reviewers", "prevent_self_review": true, "reviewers": [
				{"type": "User", "reviewer": {"login": "octocat", "id": 583231, "type": "User", "node_id": "U_1"}},
				{"type": "Team", "reviewer": {"id": 42, "slug": "core", "name": "Core"}}
			]},
			{"id": 3, "node_id": "GA_3", "type": "branch_policy"}
		],
		"deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}
	}`
	wait := 30

	successes := []struct {
		name string
		body string
		want Environment
	}{
		{
			name: "null policy and no rules is all without protection",
			body: environmentJSON(nil),
			want: Environment{ID: 161088068, Name: "production", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}},
		},
		{
			name: "actual shape with unread fields is read",
			body: actualShape,
			want: Environment{
				ID:                     161088068,
				Name:                   "Prod/Eu",
				DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected},
				Protection: setting.EnvironmentProtection{
					WaitTimer: &wait,
					RequiredReviewers: &setting.RequiredReviewers{
						PreventSelfReview: true,
						Reviewers:         []setting.Reviewer{{Type: setting.ReviewerUser, ID: 583231}, {Type: setting.ReviewerTeam, ID: 42}},
					},
					CanAdminsBypass: true,
				},
			},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeEnvironment(json.RawMessage(tc.body))
			if err != nil {
				t.Fatalf("decodeEnvironment() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeEnvironment() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name     string
		body     string
		contains []string
	}{
		{name: "missing id is unexpected response", body: environmentJSON(map[string]string{"id": ""}), contains: []string{"missing fields", "id"}},
		{name: "null id is unexpected response", body: environmentJSON(map[string]string{"id": "null"}), contains: []string{"missing fields", "id"}},
		{name: "missing name is unexpected response", body: environmentJSON(map[string]string{"name": ""}), contains: []string{"missing fields", "name"}},
		{name: "null name is unexpected response", body: environmentJSON(map[string]string{"name": "null"}), contains: []string{"missing fields", "name"}},
		{name: "missing can_admins_bypass is unexpected response", body: environmentJSON(map[string]string{"can_admins_bypass": ""}), contains: []string{"missing fields", "can_admins_bypass"}},
		{name: "null can_admins_bypass is unexpected response", body: environmentJSON(map[string]string{"can_admins_bypass": "null"}), contains: []string{"missing fields", "can_admins_bypass"}},
		{name: "missing protection_rules is unexpected response", body: environmentJSON(map[string]string{"protection_rules": ""}), contains: []string{"missing fields", "protection_rules"}},
		{name: "null protection_rules is unexpected response", body: environmentJSON(map[string]string{"protection_rules": "null"}), contains: []string{"missing fields", "protection_rules"}},
		{name: "missing deployment_branch_policy is unexpected response", body: environmentJSON(map[string]string{"deployment_branch_policy": ""}), contains: []string{"missing fields", "deployment_branch_policy"}},
		{name: "empty object names every missing field", body: `{}`, contains: []string{"id", "name", "can_admins_bypass", "protection_rules", "deployment_branch_policy"}},
		{name: "null element is unexpected response", body: `null`, contains: []string{"missing fields"}},
		{name: "empty name is unexpected response", body: environmentJSON(map[string]string{"name": `""`}), contains: []string{"unsupported value", "name"}},
		{name: "string id is unexpected response", body: environmentJSON(map[string]string{"id": `"1"`})},
		{name: "unsupported policy is unexpected response", body: environmentJSON(map[string]string{"deployment_branch_policy": `{"protected_branches": true, "custom_branch_policies": true}`}), contains: []string{"unsupported value", "deployment_branch_policy"}},
		{name: "invalid protection rule is unexpected response", body: environmentJSON(map[string]string{"protection_rules": `[{"type": "wait_timer"}]`}), contains: []string{"wait_timer"}},
		{name: "non-JSON element is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEnvironment(json.RawMessage(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeEnvironment() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeEnvironment() error = %q, want it to contain %q", err, s)
				}
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeDeploymentBranchPolicyMode(t *testing.T) {
	successes := []struct {
		name string
		raw  string
		want setting.DeploymentBranchPolicyMode
	}{
		{name: "null is all", raw: `null`, want: setting.DeploymentBranchPolicyAll},
		{name: "protected_branches true and custom_branch_policies false is protected_branches", raw: `{"protected_branches": true, "custom_branch_policies": false}`, want: setting.DeploymentBranchPolicyProtectedBranches},
		{name: "protected_branches false and custom_branch_policies true is selected", raw: `{"protected_branches": false, "custom_branch_policies": true}`, want: setting.DeploymentBranchPolicySelected},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeDeploymentBranchPolicyMode(json.RawMessage(tc.raw))
			if err != nil {
				t.Fatalf("decodeDeploymentBranchPolicyMode() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("decodeDeploymentBranchPolicyMode() = %q, want %q", got, tc.want)
			}
		})
	}

	failures := []struct {
		name     string
		body     string
		contains []string
	}{
		{name: "both true is unexpected response", body: `{"protected_branches": true, "custom_branch_policies": true}`, contains: []string{"unsupported value", "deployment_branch_policy"}},
		{name: "both false is unexpected response", body: `{"protected_branches": false, "custom_branch_policies": false}`, contains: []string{"unsupported value", "deployment_branch_policy"}},
		{name: "missing protected_branches is unexpected response", body: `{"custom_branch_policies": true}`, contains: []string{"missing fields", "protected_branches"}},
		{name: "null protected_branches is unexpected response", body: `{"protected_branches": null, "custom_branch_policies": true}`, contains: []string{"missing fields", "protected_branches"}},
		{name: "missing custom_branch_policies is unexpected response", body: `{"protected_branches": true}`, contains: []string{"missing fields", "custom_branch_policies"}},
		{name: "null custom_branch_policies is unexpected response", body: `{"protected_branches": true, "custom_branch_policies": null}`, contains: []string{"missing fields", "custom_branch_policies"}},
		{name: "empty object is unexpected response", body: `{}`, contains: []string{"protected_branches", "custom_branch_policies"}},
		{name: "number is unexpected response", body: `1`},
		{name: "string is unexpected response", body: `"all"`},
		{name: "array is unexpected response", body: `[]`},
		{name: "absent key is unexpected response", body: ``},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeDeploymentBranchPolicyMode(json.RawMessage(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeDeploymentBranchPolicyMode() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeDeploymentBranchPolicyMode() error = %q, want it to contain %q", err, s)
				}
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeEnvironmentProtection(t *testing.T) {
	const waitTimerRule = `{"type": "wait_timer", "wait_timer": 30}`
	const reviewersRule = `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": "User", "reviewer": {"id": 1}}, {"type": "Team", "reviewer": {"id": 2}}]}`
	wait30, wait0 := 30, 0
	toRules := func(rules ...string) []json.RawMessage {
		raws := make([]json.RawMessage, len(rules))
		for i, r := range rules {
			raws[i] = json.RawMessage(r)
		}
		return raws
	}

	successes := []struct {
		name            string
		rules           []string
		canAdminsBypass bool
		want            setting.EnvironmentProtection
	}{
		{name: "no rules is no protection", rules: nil, want: setting.EnvironmentProtection{}},
		{name: "can_admins_bypass is carried over", rules: nil, canAdminsBypass: true, want: setting.EnvironmentProtection{CanAdminsBypass: true}},
		{name: "wait_timer rule is the wait timer", rules: []string{waitTimerRule}, want: setting.EnvironmentProtection{WaitTimer: &wait30}},
		{name: "wait_timer 0 is a value", rules: []string{`{"type": "wait_timer", "wait_timer": 0}`}, want: setting.EnvironmentProtection{WaitTimer: &wait0}},
		{
			name:  "required_reviewers rule keeps the reviewers in order",
			rules: []string{reviewersRule},
			want: setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{
				PreventSelfReview: true,
				Reviewers:         []setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}, {Type: setting.ReviewerTeam, ID: 2}},
			}},
		},
		{
			name:  "empty reviewers is an empty list, not no rule",
			rules: []string{`{"type": "required_reviewers", "prevent_self_review": false, "reviewers": []}`},
			want:  setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{Reviewers: []setting.Reviewer{}}},
		},
		{
			name:  "both rules are read",
			rules: []string{reviewersRule, waitTimerRule},
			want: setting.EnvironmentProtection{
				WaitTimer: &wait30,
				RequiredReviewers: &setting.RequiredReviewers{
					PreventSelfReview: true,
					Reviewers:         []setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}, {Type: setting.ReviewerTeam, ID: 2}},
				},
			},
		},
		{name: "branch_policy rule is skipped", rules: []string{`{"type": "branch_policy"}`}, want: setting.EnvironmentProtection{}},
		{name: "rule of a type reco does not know is skipped", rules: []string{`{"type": "custom_rule", "id": 9}`, waitTimerRule}, want: setting.EnvironmentProtection{WaitTimer: &wait30}},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeEnvironmentProtection(toRules(tc.rules...), tc.canAdminsBypass)
			if err != nil {
				t.Fatalf("decodeEnvironmentProtection() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeEnvironmentProtection() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// failures の body は、保護ルールを1つ置く。reviewersRule を基準に、条件を1つだけ変える。
	// notContains は、欠けていない項目が missing fields に挙がらないことを確かめる。
	failures := []struct {
		name        string
		body        string
		contains    []string
		notContains []string
	}{
		{name: "wait_timer rule without wait_timer is unexpected response", body: `{"type": "wait_timer"}`, contains: []string{"missing fields", "protection_rules.wait_timer"}},
		{name: "wait_timer rule with null wait_timer is unexpected response", body: `{"type": "wait_timer", "wait_timer": null}`, contains: []string{"missing fields", "protection_rules.wait_timer"}},
		{name: "required_reviewers rule without reviewers names only reviewers", body: `{"type": "required_reviewers", "prevent_self_review": true}`, contains: []string{"missing fields", "protection_rules.reviewers"}, notContains: []string{"prevent_self_review"}},
		{name: "required_reviewers rule with null reviewers names only reviewers", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": null}`, contains: []string{"missing fields", "protection_rules.reviewers"}, notContains: []string{"prevent_self_review"}},
		{name: "required_reviewers rule without prevent_self_review names only prevent_self_review", body: `{"type": "required_reviewers", "reviewers": []}`, contains: []string{"missing fields", "protection_rules.prevent_self_review"}, notContains: []string{"protection_rules.reviewers"}},
		{name: "required_reviewers rule with null prevent_self_review names only prevent_self_review", body: `{"type": "required_reviewers", "prevent_self_review": null, "reviewers": []}`, contains: []string{"missing fields", "protection_rules.prevent_self_review"}, notContains: []string{"protection_rules.reviewers"}},
		{name: "required_reviewers rule without either names both", body: `{"type": "required_reviewers"}`, contains: []string{"missing fields", "protection_rules.reviewers", "protection_rules.prevent_self_review"}},
		{name: "reviewer of an unsupported type is unexpected response without the value", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": "Bot-MARKER", "reviewer": {"id": 1}}]}`, contains: []string{"unsupported value", "protection_rules.reviewers.type"}, notContains: []string{"Bot-MARKER"}},
		{name: "reviewer without type names only type", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"reviewer": {"id": 1}}]}`, contains: []string{"missing fields", "protection_rules.reviewers.type"}, notContains: []string{"reviewer.id"}},
		{name: "reviewer with null type names only type", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": null, "reviewer": {"id": 1}}]}`, contains: []string{"missing fields", "protection_rules.reviewers.type"}, notContains: []string{"reviewer.id"}},
		{name: "reviewer without reviewer names only reviewer id", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": "User"}]}`, contains: []string{"missing fields", "protection_rules.reviewers.reviewer.id"}, notContains: []string{"reviewers.type"}},
		{name: "reviewer with null reviewer names only reviewer id", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": "User", "reviewer": null}]}`, contains: []string{"missing fields", "protection_rules.reviewers.reviewer.id"}, notContains: []string{"reviewers.type"}},
		{name: "reviewer without reviewer id names only reviewer id", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{"type": "User", "reviewer": {"login": "a"}}]}`, contains: []string{"missing fields", "protection_rules.reviewers.reviewer.id"}, notContains: []string{"reviewers.type"}},
		{name: "reviewer without either names both", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [{}]}`, contains: []string{"missing fields", "protection_rules.reviewers.type", "protection_rules.reviewers.reviewer.id"}},
		{name: "null reviewer element is an unsupported value", body: `{"type": "required_reviewers", "prevent_self_review": true, "reviewers": [null]}`, contains: []string{"unsupported value", "protection_rules.reviewers"}, notContains: []string{"missing fields"}},
		{name: "rule without type is unexpected response", body: `{"wait_timer": 30}`, contains: []string{"missing fields", "protection_rules.type"}},
		{name: "rule with null type is unexpected response", body: `{"type": null, "wait_timer": 30}`, contains: []string{"missing fields", "protection_rules.type"}},
		{name: "null rule is unexpected response", body: `null`, contains: []string{"missing fields", "protection_rules.type"}},
		{name: "non-object rule is unexpected response", body: `1`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEnvironmentProtection(toRules(waitTimerRule, tc.body), false)
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeEnvironmentProtection() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeEnvironmentProtection() error = %q, want it to contain %q", err, s)
				}
			}
			for _, s := range tc.notContains {
				if strings.Contains(err.Error(), s) {
					t.Errorf("decodeEnvironmentProtection() error = %q, must not contain %q", err, s)
				}
			}
		})
	}

	duplicates := []struct {
		name     string
		rule     string
		contains []string
	}{
		{name: "two wait_timer rules are unexpected response", rule: waitTimerRule, contains: []string{"duplicate", "wait_timer"}},
		{name: "two required_reviewers rules are unexpected response", rule: reviewersRule, contains: []string{"duplicate", "required_reviewers"}},
	}
	for _, tc := range duplicates {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeEnvironmentProtection(toRules(tc.rule, tc.rule), false)
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeEnvironmentProtection() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeEnvironmentProtection() error = %q, want it to contain %q", err, s)
				}
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeDeploymentPattern(t *testing.T) {
	successes := []struct {
		name        string
		body        string
		wantPattern setting.DeploymentPattern
		wantID      DeploymentPatternID
	}{
		{name: "branch pattern is read with its id", body: `{"id": 11, "node_id": "x", "name": "release/*", "type": "branch"}`, wantPattern: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "release/*"}, wantID: 11},
		{name: "tag pattern is read with its id", body: `{"id": 12, "name": "v*", "type": "tag"}`, wantPattern: setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"}, wantID: 12},
		{name: "empty name is kept as it is", body: `{"id": 13, "name": "", "type": "branch"}`, wantPattern: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch}, wantID: 13},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			pattern, id, err := decodeDeploymentPattern(json.RawMessage(tc.body))
			if err != nil {
				t.Fatalf("decodeDeploymentPattern() error = %v", err)
			}
			if pattern != tc.wantPattern || id != tc.wantID {
				t.Errorf("decodeDeploymentPattern() = %+v, %d, want %+v, %d", pattern, id, tc.wantPattern, tc.wantID)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		contains    []string
		notContains string
	}{
		{name: "missing id is unexpected response", body: `{"name": "a", "type": "branch"}`, contains: []string{"missing fields", "id"}},
		{name: "null id is unexpected response", body: `{"id": null, "name": "a", "type": "branch"}`, contains: []string{"missing fields", "id"}},
		{name: "missing name is unexpected response", body: `{"id": 1, "type": "branch"}`, contains: []string{"missing fields", "name"}},
		{name: "null name is unexpected response", body: `{"id": 1, "name": null, "type": "branch"}`, contains: []string{"missing fields", "name"}},
		{name: "missing type is unexpected response", body: `{"id": 1, "name": "a"}`, contains: []string{"missing fields", "type"}},
		{name: "null type is unexpected response", body: `{"id": 1, "name": "a", "type": null}`, contains: []string{"missing fields", "type"}},
		{name: "empty object names every missing field", body: `{}`, contains: []string{"id", "name", "type"}},
		{name: "unsupported type is unexpected response without the value", body: `{"id": 1, "name": "a", "type": "commit-MARKER"}`, contains: []string{"unsupported value", "type"}, notContains: "commit-MARKER"},
		{name: "null element is unexpected response", body: `null`, contains: []string{"missing fields"}},
		{name: "non-JSON element is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := decodeDeploymentPattern(json.RawMessage(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeDeploymentPattern() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.contains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeDeploymentPattern() error = %q, want it to contain %q", err, s)
				}
			}
			if tc.notContains != "" && strings.Contains(err.Error(), tc.notContains) {
				t.Errorf("decodeDeploymentPattern() error = %q, must not contain %q", err, tc.notContains)
			}
		})
	}
}
