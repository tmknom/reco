package github

import (
	"encoding/json"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustRulesetName(t *testing.T, s string) setting.RulesetName {
	t.Helper()
	name, err := setting.ParseRulesetName(s)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", s, err)
	}
	return name
}

// interpretedRulesetValue は、全体を解釈できた Ruleset の値で、GitHub が詳細で返す形と同じく ref_name と空の集合を持つ。
func interpretedRulesetValue() setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{},
		Rules:        []setting.RulesetRule{setting.DeletionRule()},
	}
}

// rulesetDetail は、Ruleset の取得の応答のうち受理される基準の本文を change で変えて返す。
// 基準は、値に含めるすべての部分と、reco が読まないトップレベルのフィールドを持つ。
// rules は deletion、pull_request、non_fast_forward、required_status_checks の順で、bypass_actors は RepositoryRole の一つだけを持つ。
func rulesetDetail(t *testing.T, change func(body map[string]any)) []byte {
	t.Helper()
	body := map[string]any{
		"id":          42,
		"name":        "protect",
		"target":      "branch",
		"source_type": "Repository",
		"source":      "owner/repo",
		"enforcement": "active",
		"conditions": map[string]any{
			"ref_name": map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{"refs/heads/tmp"}},
		},
		"bypass_actors": []any{
			map[string]any{"actor_type": "RepositoryRole", "actor_id": 5, "bypass_mode": "always"},
		},
		"rules": []any{
			map[string]any{"type": "deletion"},
			map[string]any{"type": "pull_request", "parameters": map[string]any{
				"required_approving_review_count":                 1,
				"dismiss_stale_reviews_on_push":                   true,
				"require_code_owner_review":                       false,
				"require_last_push_approval":                      true,
				"required_review_thread_resolution":               false,
				"allowed_merge_methods":                           []any{"merge", "squash", "rebase"},
				"require_extra_approval_for_unattributed_changes": true,
				"required_reviewers":                              []any{},
			}},
			map[string]any{"type": "non_fast_forward"},
			map[string]any{"type": "required_status_checks", "parameters": map[string]any{
				"required_status_checks":               []any{map[string]any{"context": "test"}, map[string]any{"context": "lint", "integration_id": 15368}},
				"strict_required_status_checks_policy": true,
				"do_not_enforce_on_create":             false,
			}},
		},
		"node_id":                 "RRS_1",
		"_links":                  map[string]any{"self": map[string]any{"href": "https://api.github.com/repos/owner/repo/rulesets/42"}},
		"created_at":              "2026-01-01T00:00:00Z",
		"updated_at":              "2026-01-01T00:00:00Z",
		"current_user_can_bypass": "always",
		"future_field":            map[string]any{"unknown": true},
	}
	if change != nil {
		change(body)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
