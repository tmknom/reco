package apply

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

// TestExecuteChange_Rulesets は、Ruleset の作成を、一覧と名指した Ruleset の詳細の読み直しで確かめて一度だけ送るつなぎ目を確かめる。
// 名前で引く規則は、github の Rulesets.Lookup のテストが確かめている。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_Rulesets(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getRulesets   = "GET /repos/owner/repo/rulesets"
		getProtect    = "GET /repos/owner/repo/rulesets/42"
		postRuleset   = "POST /repos/owner/repo/rulesets"
	)
	other := githubtest.RulesetSummaryBody{ID: 7, Name: "other"}
	protect := githubtest.RulesetSummaryBody{ID: 42, Name: "protect"}
	created := protectRulesetBody(42, "protect", setting.RulesetEnforcementActive)
	// readAfter は、作成のあとの観測を times 回、同じ一覧と詳細で返す応答と、その要求の並び。
	readAfter := func(times int, detail githubtest.RulesetBody) ([]response, []string) {
		var responses []response
		var requests []string
		for range times {
			responses = append(responses, okRepository(1, false, false), okRulesets(other, protect), okRuleset(detail))
			requests = append(requests, getRepository, getRulesets, getProtect)
		}
		return responses, requests
	}
	matching, matchingRequests := readAfter(1, created)
	disabled, disabledRequests := readAfter(1+len(reobservationDelays), protectRulesetBody(42, "protect", setting.RulesetEnforcementDisabled))
	uninterpretedBody := created
	uninterpretedBody.Rules = []string{`{"type": "creation"}`}
	uninterpreted, uninterpretedRequests := readAfter(1+len(reobservationDelays), uninterpretedBody)
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantPrecond  preconditionCheck
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantReason   github.UnavailableReason
		wantResult   changeResult
	}{
		{
			// 宣言していない other は一覧にあっても詳細を読まず、宛先にもならない
			name:         "creation absent before the update is sent once and completes",
			responses:    slices.Concat([]response{okRepository(1, false, false), okRulesets(other), createdRuleset(created)}, matching),
			wantRequests: slices.Concat([]string{getRepository, getRulesets, postRuleset}, matchingRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// GitHub は大文字小文字だけが違う名前の作成も 422 で拒否するが、拒否に頼らず更新直前の観測で止める
			name: "creation with a case-insensitively equal name existing before the update sends no update request",
			responses: []response{
				okRepository(1, false, false), okRulesets(githubtest.RulesetSummaryBody{ID: 42, Name: "PROTECT"}), okRuleset(protectRulesetBody(42, "PROTECT", setting.RulesetEnforcementActive)),
			},
			wantRequests: []string{getRepository, getRulesets, getProtect},
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			name: "creation rejected with 422 fails without reading after the update",
			responses: []response{
				okRepository(1, false, false), okRulesets(), {status: http.StatusUnprocessableEntity, body: `{"message":"Validation Failed","errors":["Name must be unique"]}`},
			},
			wantRequests: []string{getRepository, getRulesets, postRuleset},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateRejected,
			wantResult:   failed,
		},
		{
			name:         "creation answered with 500 is not sent again and fails even if the ruleset is seen in the read for diagnosis",
			responses:    slices.Concat([]response{okRepository(1, false, false), okRulesets(), {status: http.StatusInternalServerError}}, matching),
			wantRequests: slices.Concat([]string{getRepository, getRulesets, postRuleset}, matchingRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateUnknown,
			wantDesired:  desiredStateMatches,
			wantResult:   failed,
		},
		{
			name:         "created ruleset whose value differs from the declaration in every read is a mismatch",
			responses:    slices.Concat([]response{okRepository(1, false, false), okRulesets(), createdRuleset(created)}, disabled),
			wantRequests: slices.Concat([]string{getRepository, getRulesets, postRuleset}, disabledRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMismatch,
			wantResult:   failed,
		},
		{
			name:         "created ruleset with uninterpreted parts in every read is unavailable by those parts",
			responses:    slices.Concat([]response{okRepository(1, false, false), okRulesets(), createdRuleset(created)}, uninterpreted),
			wantRequests: slices.Concat([]string{getRepository, getRulesets, postRuleset}, uninterpretedRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateUnavailable,
			wantReason:   github.UnavailableUninterpretedParts,
			wantResult:   failed,
		},
		{
			// 一覧の同じ名前は観測操作の失敗で、どちらかの値を期待値と比べない
			name: "list with the same name twice after the update is unconfirmed",
			responses: []response{
				okRepository(1, false, false), okRulesets(), createdRuleset(created),
				okRepository(1, false, false), okRulesets(protect, githubtest.RulesetSummaryBody{ID: 43, Name: "Protect"}),
			},
			wantRequests: []string{getRepository, getRulesets, postRuleset, getRepository, getRulesets},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateObservationFailed,
			wantResult:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), rulesetCreationChange(t, "protect"), elapsedAtOnce, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.desiredUnavailable.reason != tc.wantReason {
				t.Errorf("desiredUnavailable.reason = %v, want %v", got.desiredUnavailable.reason, tc.wantReason)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_RulesetValueUpdate は、Ruleset の値の変更を、一覧と名指した Ruleset の詳細の読み直しで確かめて、
// 承認時に予定した ID へ一度だけ PUT で送るつなぎ目を確かめる。
// 値の全体と ID の比べ方は TestMatchesPreconditionBasis_Ruleset、本文は TestSendUpdate が確かめている。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_RulesetValueUpdate(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getRulesets   = "GET /repos/owner/repo/rulesets"
		getProtect    = "GET /repos/owner/repo/rulesets/42"
		putProtect    = "PUT /repos/owner/repo/rulesets/42"
	)
	protect := githubtest.RulesetSummaryBody{ID: 42, Name: "protect"}
	approved := protectRulesetBody(42, "protect", setting.RulesetEnforcementDisabled)
	updated := protectRulesetBody(42, "protect", setting.RulesetEnforcementActive)
	before := []response{okRepository(1, false, false), okRulesets(protect), okRuleset(approved)}
	beforeRequests := []string{getRepository, getRulesets, getProtect}
	// readAfter は、値の変更のあとの観測を times 回、同じ一覧と詳細で返す応答と、その要求の並び。
	readAfter := func(times int, detail githubtest.RulesetBody) ([]response, []string) {
		var responses []response
		var requests []string
		for range times {
			responses = append(responses, okRepository(1, false, false), okRulesets(protect), okRuleset(detail))
			requests = append(requests, getRepository, getRulesets, getProtect)
		}
		return responses, requests
	}
	matching, matchingRequests := readAfter(1, updated)
	unchanged, unchangedRequests := readAfter(1+len(reobservationDelays), approved)
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantPrecond  preconditionCheck
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			name:         "ruleset as approved before the update is replaced once and completes",
			responses:    slices.Concat(before, []response{okRuleset(updated)}, matching),
			wantRequests: slices.Concat(beforeRequests, []string{putProtect}, matchingRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 承認したのは値の変更で、作成ではない。存在しなくなった Ruleset を作成に切り替えない
			name:         "ruleset deleted before the update sends neither PUT nor POST",
			responses:    []response{okRepository(1, false, false), okRulesets()},
			wantRequests: []string{getRepository, getRulesets},
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			name:         "ruleset whose value differs from the declaration in every read after the update is a mismatch",
			responses:    slices.Concat(before, []response{okRuleset(updated)}, unchanged),
			wantRequests: slices.Concat(beforeRequests, []string{putProtect}, unchangedRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMismatch,
			wantResult:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), rulesetValueUpdateChange(t, "protect"), elapsedAtOnce, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_RulesetDeletion は、存在しないことを宣言した Ruleset の削除を、承認時に予定した ID へ一度だけ DELETE で送り、
// 成功応答だけで完了にせず、更新後の観測で存在しないと確かめるつなぎ目を確かめる。
// 値の全体と ID の比べ方は TestMatchesPreconditionBasis_Ruleset、宛先は TestSendUpdate が確かめている。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_RulesetDeletion(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getRulesets   = "GET /repos/owner/repo/rulesets"
		getProtect    = "GET /repos/owner/repo/rulesets/42"
		deleteProtect = "DELETE /repos/owner/repo/rulesets/42"
		getRecreated  = "GET /repos/owner/repo/rulesets/43"
	)
	protect := githubtest.RulesetSummaryBody{ID: 42, Name: "protect"}
	other := githubtest.RulesetSummaryBody{ID: 7, Name: "other"}
	approved := protectRulesetBody(42, "protect", setting.RulesetEnforcementDisabled)
	before := []response{okRepository(1, false, false), okRulesets(other, protect), okRuleset(approved)}
	beforeRequests := []string{getRepository, getRulesets, getProtect}
	// remaining は、削除のあとの観測を、毎回 protect がまだ存在する応答で返す応答と、その要求の並び。
	var remaining []response
	var remainingRequests []string
	for range 1 + len(reobservationDelays) {
		remaining = append(remaining, okRepository(1, false, false), okRulesets(other, protect), okRuleset(approved))
		remainingRequests = append(remainingRequests, getRepository, getRulesets, getProtect)
	}
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantPrecond  preconditionCheck
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			// 宣言していない other は、一覧に現れても詳細を読まず、DELETE も送らない
			name:         "ruleset as approved is deleted once and completes when the read after the update finds it absent",
			responses:    slices.Concat(before, []response{noContent(), okRepository(1, false, false), okRulesets(other)}),
			wantRequests: slices.Concat(beforeRequests, []string{deleteProtect, getRepository, getRulesets}),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:         "ruleset whose value changed before the update is not deleted",
			responses:    []response{okRepository(1, false, false), okRulesets(protect), okRuleset(protectRulesetBody(42, "protect", setting.RulesetEnforcementActive))},
			wantRequests: beforeRequests,
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			name:         "ruleset recreated with the same value before the update is not deleted",
			responses:    []response{okRepository(1, false, false), okRulesets(githubtest.RulesetSummaryBody{ID: 43, Name: "protect"}), okRuleset(protectRulesetBody(43, "protect", setting.RulesetEnforcementDisabled))},
			wantRequests: []string{getRepository, getRulesets, getRecreated},
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			// すでに存在しないことは期待状態と同じでも、承認した削除を行っていないため完了にしない
			name:         "ruleset already absent before the update is not attempted rather than completed",
			responses:    []response{okRepository(1, false, false), okRulesets(other)},
			wantRequests: []string{getRepository, getRulesets},
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			name:         "404 to DELETE is a rejected update and is not read as absence",
			responses:    slices.Concat(before, []response{{status: http.StatusNotFound}}),
			wantRequests: slices.Concat(beforeRequests, []string{deleteProtect}),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateRejected,
			wantResult:   failed,
		},
		{
			name:         "ruleset still present in every read after the update is a mismatch",
			responses:    slices.Concat(before, []response{noContent()}, remaining),
			wantRequests: slices.Concat(beforeRequests, []string{deleteProtect}, remainingRequests),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMismatch,
			wantResult:   failed,
		},
		{
			name:         "read failure after the update is not read as absence",
			responses:    slices.Concat(before, []response{noContent(), okRepository(1, false, false), {status: http.StatusNotFound}}),
			wantRequests: slices.Concat(beforeRequests, []string{deleteProtect, getRepository, getRulesets}),
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateObservationFailed,
			wantResult:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), rulesetDeletionChange(t, "protect"), elapsedAtOnce, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChanges_Rulesets は、二つの Ruleset の作成を変更の並びで実行し、あいだに更新要求の間隔を一度だけ待ち、
// 二つ目の観測が、一つ目が作った Ruleset の詳細を読まないことを確かめる。
//
//trace:checks INV-005 FR-010 FR-013
func TestExecuteChanges_Rulesets(t *testing.T) {
	alpha := githubtest.RulesetSummaryBody{ID: 1, Name: "alpha"}
	beta := githubtest.RulesetSummaryBody{ID: 2, Name: "beta"}
	alphaBody := protectRulesetBody(1, "alpha", setting.RulesetEnforcementActive)
	betaBody := protectRulesetBody(2, "beta", setting.RulesetEnforcementActive)
	baseURL, server := newSequenceServer(t,
		okRepository(1, false, false), okRulesets(), createdRuleset(alphaBody),
		okRepository(1, false, false), okRulesets(alpha), okRuleset(alphaBody),
		okRepository(1, false, false), okRulesets(alpha), createdRuleset(betaBody),
		okRepository(1, false, false), okRulesets(alpha, beta), okRuleset(betaBody),
	)
	delays := &delayRecorder{}
	changes := []changeplan.LogicalChange{rulesetCreationChange(t, "alpha"), rulesetCreationChange(t, "beta")}

	got := executeChanges(context.Background(), newClient(baseURL), newRepositoryPlan(t), changes, delays.after, &updateInterval{})

	wantRequests := []string{
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "POST /repos/owner/repo/rulesets",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "GET /repos/owner/repo/rulesets/1",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "POST /repos/owner/repo/rulesets",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "GET /repos/owner/repo/rulesets/2",
	}
	if !slices.Equal(server.RequestLines(), wantRequests) {
		t.Errorf("requests = %v, want %v", server.RequestLines(), wantRequests)
	}
	if names := []any{server.Bodies()[2]["name"], server.Bodies()[8]["name"]}; !slices.Equal(names, []any{"alpha", "beta"}) {
		t.Errorf("created names = %v, want [alpha beta]", names)
	}
	if want := []time.Duration{updateIntervalDelay}; !slices.Equal(delays.delays, want) {
		t.Errorf("delays = %v, want %v", delays.delays, want)
	}
	for i, record := range got {
		if record.result() != completed {
			t.Errorf("records[%d].result() = %v, want %v", i, record.result(), completed)
		}
	}
}
