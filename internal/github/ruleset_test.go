package github

import (
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

func TestRuleset(t *testing.T) {
	value := setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive, Rules: []setting.RulesetRule{setting.DeletionRule()}}
	parts := []UninterpretedPart{{Kind: UninterpretedRuleType, Name: "creation"}}

	t.Run("interpreted ruleset has its value", func(t *testing.T) {
		r := InterpretedRuleset(1, "protect", value)
		got, ok := r.Value()
		if !ok || !reflect.DeepEqual(got, value) {
			t.Errorf("Value() = (%+v, %t), want (%+v, true)", got, ok, value)
		}
		if r.ID != 1 || r.Name != "protect" {
			t.Errorf("ID, Name = %d, %q, want 1, %q", r.ID, r.Name, "protect")
		}
	})
	t.Run("uninterpreted ruleset has no value and has its parts", func(t *testing.T) {
		r := UninterpretedRuleset(2, "tags", parts)
		if got, ok := r.Value(); ok {
			t.Errorf("Value() = (%+v, true), want false", got)
		}
		if got := r.UninterpretedParts(); !slices.Equal(got, parts) {
			t.Errorf("UninterpretedParts() = %+v, want %+v", got, parts)
		}
	})
	t.Run("uninterpreted parts of an interpreted ruleset panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UninterpretedParts() did not panic")
			}
		}()
		InterpretedRuleset(1, "protect", value).UninterpretedParts()
	})
	t.Run("uninterpreted parts of the zero value panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UninterpretedParts() did not panic")
			}
		}()
		Ruleset{}.UninterpretedParts()
	})
	t.Run("uninterpreted ruleset without parts panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UninterpretedRuleset() did not panic")
			}
		}()
		UninterpretedRuleset(2, "tags", nil)
	})
	// 読んでいない Ruleset の値を、解釈しない部分を含む値と取り違えて偽で返さないことを確かめる
	t.Run("value of an unread ruleset panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Value() did not panic")
			}
		}()
		_, _ = unreadRuleset(3, "protect").Value()
	})
	t.Run("value of the zero value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Value() did not panic")
			}
		}()
		_, _ = Ruleset{}.Value()
	})
}

//trace:checks FR-004 FR-005
func TestRulesets_Lookup(t *testing.T) {
	declared := mustRulesetName(t, "protect")
	const undetermined = "Protéct"
	interpreted := InterpretedRuleset(2, "Protect", interpretedRulesetValue())
	uninterpreted := UninterpretedRuleset(2, "PROTECT", []UninterpretedPart{{Kind: UninterpretedRuleType, Name: "creation"}})
	observed := func(rulesets ...Ruleset) Rulesets {
		return Rulesets{Existing: Observed(rulesets)}
	}

	cases := []struct {
		name     string
		rulesets Rulesets
		want     RulesetObservation
	}{
		{
			name:     "a list unavailable for a private repository is unavailable for the same reason",
			rulesets: Rulesets{Existing: Unavailable[[]Ruleset](UnavailablePrivateRepository)},
			want:     RulesetObservation{state: resourceUnavailable, reason: UnavailablePrivateRepository},
		},
		{
			name:     "the same name with different case exists as the observed ruleset",
			rulesets: observed(InterpretedRuleset(1, "other", interpretedRulesetValue()), interpreted),
			want:     RulesetObservation{state: resourceExisting, ruleset: interpreted},
		},
		{
			name:     "the same name with uninterpreted parts exists as the observed ruleset",
			rulesets: observed(uninterpreted),
			want:     RulesetObservation{state: resourceExisting, ruleset: uninterpreted},
		},
		{
			name:     "an undetermined name before the same name does not hide the same name",
			rulesets: observed(unreadRuleset(1, undetermined), interpreted),
			want:     RulesetObservation{state: resourceExisting, ruleset: interpreted},
		},
		{
			name:     "an undetermined name after the same name does not hide the same name",
			rulesets: observed(interpreted, unreadRuleset(3, undetermined)),
			want:     RulesetObservation{state: resourceExisting, ruleset: interpreted},
		},
		{
			name:     "only an undetermined name is unavailable as the name undetermined",
			rulesets: observed(unreadRuleset(1, undetermined)),
			want:     RulesetObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined},
		},
		{
			name:     "a different name after an undetermined name is still unavailable",
			rulesets: observed(unreadRuleset(1, undetermined), unreadRuleset(3, "release")),
			want:     RulesetObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined},
		},
		{
			name:     "only different names is absent",
			rulesets: observed(unreadRuleset(1, "release"), unreadRuleset(3, "protect-tags")),
			want:     RulesetObservation{state: resourceAbsent},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rulesets.Lookup(declared); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lookup(%q) = %+v, want %+v", declared, got, tc.want)
			}
		})
	}
}

// 解釈しなかったものの種類ごとに、種類と、引用符で囲んでエスケープした名前の一行を確かめる。
//
//trace:checks FR-004 QR-002
func TestUninterpretedPart_String(t *testing.T) {
	cases := []struct {
		part UninterpretedPart
		want string
	}{
		{part: UninterpretedPart{Kind: UninterpretedTarget, Name: "tag"}, want: `target: "tag"`},
		{part: UninterpretedPart{Kind: UninterpretedEnforcement, Name: "enforced\n"}, want: `enforcement: "enforced\n"`},
		{part: UninterpretedPart{Kind: UninterpretedConditionsKey, Name: "ref_name.extra"}, want: `conditions key: "ref_name.extra"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassActorType, Name: "Team"}, want: `bypass actor type: "Team"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassMode, Name: "never"}, want: `bypass mode: "never"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassActorKey, Name: "note"}, want: `bypass actor key: "note"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleType, Name: "tag_name_pattern"}, want: `rule type: "tag_name_pattern"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleKey, Rule: setting.RulesetRuleDeletion, Name: "extra"}, want: `deletion key: "extra"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: setting.RulesetRulePullRequest, Name: "required_reviewers"}, want: `pull_request parameter: "required_reviewers"`},
		// GitHub が返した任意の名前を、行の区切りや表示の向きを変えずに一行に収める
		{part: UninterpretedPart{Kind: UninterpretedRuleType, Name: "x\n\"y\"\u202e"}, want: `rule type: "x\n\"y\"\u202e"`},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.part.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("an undefined kind panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("String() did not panic")
			}
		}()
		_ = UninterpretedPart{Kind: UninterpretedKind(99), Name: "x"}.String()
	})
}

//trace:checks FR-004 QR-002
func TestRulesetsUnavailability(t *testing.T) {
	cases := []struct {
		name            string
		private         bool
		wantReason      UnavailableReason
		wantUnavailable bool
	}{
		{name: "private repository is unavailable as private", private: true, wantReason: UnavailablePrivateRepository, wantUnavailable: true},
		{name: "public repository is available", private: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, unavailable := rulesetsUnavailability(tc.private)
			if reason != tc.wantReason || unavailable != tc.wantUnavailable {
				t.Errorf("rulesetsUnavailability(%t) = (%v, %t), want (%v, %t)", tc.private, reason, unavailable, tc.wantReason, tc.wantUnavailable)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestCheckRulesetDetail(t *testing.T) {
	summary := rulesetSummary{id: 7, name: "Protect"}
	value := setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive}
	cases := []struct {
		name    string
		detail  Ruleset
		wantErr bool
	}{
		{name: "the same id and name is the listed ruleset", detail: InterpretedRuleset(7, "Protect", value)},
		{name: "an uninterpreted detail with the same id and name is the listed ruleset", detail: UninterpretedRuleset(7, "Protect", []UninterpretedPart{{Kind: UninterpretedTarget, Name: "tag"}})},
		{name: "another id is unexpected response", detail: InterpretedRuleset(8, "Protect", value), wantErr: true},
		{name: "another name is unexpected response", detail: InterpretedRuleset(7, "Release", value), wantErr: true},
		{name: "a name that differs only in case is unexpected response", detail: InterpretedRuleset(7, "protect", value), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkRulesetDetail(summary, tc.detail)
			if tc.wantErr {
				if !errors.Is(err, ErrUnexpectedResponse) {
					t.Errorf("checkRulesetDetail() error = %v, want %v", err, ErrUnexpectedResponse)
				}
				return
			}
			if err != nil {
				t.Errorf("checkRulesetDetail() error = %v, want nil", err)
			}
		})
	}
}

const (
	rulesetsPath = "/repos/owner/repo/rulesets"
	rulesetsList = rulesetsPath + "?includes_parents=false&per_page=100"
)

// TestObserveRepositoryRulesets は、入口から Rulesets の要求を送る条件と順序、結果とエラーの受け渡しを確かめる。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryRulesets(t *testing.T) {
	deletionOnly := githubtest.RulesetBody{Target: "branch", Enforcement: "active", Rules: []string{githubtest.DeletionRuleBody}}
	detail := func(id int64, name string) githubtest.Response {
		b := deletionOnly
		b.ID, b.Name = id, name
		return githubtest.OK(b.JSON())
	}
	list := func(summaries ...githubtest.RulesetSummaryBody) githubtest.Response {
		return githubtest.OK(githubtest.RulesetsBody(summaries).JSON())
	}

	t.Run("an observation without the rulesets area sends no ruleset request and has no Rulesets", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:                    githubtest.OK(repositoryBody),
			privateVulnerabilityReportingPath: githubtest.OK(githubtest.PrivateVulnerabilityReportingBody{}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaPrivateVulnerabilityReporting)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if observed.Rulesets != nil {
			t.Errorf("ObserveRepository().Rulesets = %+v, want nil", observed.Rulesets)
		}
		want := []string{repositoryPath + "?", privateVulnerabilityReportingPath + "?"}
		if got := routes.Requests(); !slices.Equal(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("a private repository sends no ruleset request and is unavailable as private", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath: githubtest.OK(privateRepositoryBody),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRulesets)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &Rulesets{Existing: Unavailable[[]Ruleset](UnavailablePrivateRepository)}
		if !reflect.DeepEqual(observed.Rulesets, want) {
			t.Errorf("ObserveRepository().Rulesets = %+v, want %+v", observed.Rulesets, want)
		}
		if got, wantRequests := routes.Requests(), []string{repositoryPath + "?"}; !slices.Equal(got, wantRequests) {
			t.Errorf("requests = %q, want %q", got, wantRequests)
		}
	})

	t.Run("a public repository lists rulesets after private vulnerability reporting and reads the details in the list order", func(t *testing.T) {
		tagged := deletionOnly
		tagged.ID, tagged.Name, tagged.Target = 1, "a", "tag"
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:                    githubtest.OK(repositoryBody),
			privateVulnerabilityReportingPath: githubtest.OK(githubtest.PrivateVulnerabilityReportingBody{}.JSON()),
			rulesetsPath:                      list(githubtest.RulesetSummaryBody{ID: 2, Name: "b"}, githubtest.RulesetSummaryBody{ID: 1, Name: "a"}),
			rulesetsPath + "/2":               detail(2, "b"),
			rulesetsPath + "/1":               githubtest.OK(tagged.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRulesets, setting.AreaPrivateVulnerabilityReporting)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		value := setting.Ruleset{
			Target:       setting.RulesetTargetBranch,
			Enforcement:  setting.RulesetEnforcementActive,
			Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{}, Exclude: []string{}}},
			BypassActors: []setting.BypassActor{},
			Rules:        []setting.RulesetRule{setting.DeletionRule()},
		}
		want := &Rulesets{Existing: Observed([]Ruleset{
			InterpretedRuleset(2, "b", value),
			UninterpretedRuleset(1, "a", []UninterpretedPart{{Kind: UninterpretedTarget, Name: "tag"}}),
		})}
		if !reflect.DeepEqual(observed.Rulesets, want) {
			t.Errorf("ObserveRepository().Rulesets = %+v, want %+v", observed.Rulesets, want)
		}
		wantRequests := []string{repositoryPath + "?", privateVulnerabilityReportingPath + "?", rulesetsList, rulesetsPath + "/2?", rulesetsPath + "/1?"}
		if got := routes.Requests(); !slices.Equal(got, wantRequests) {
			t.Errorf("requests = %q, want %q", got, wantRequests)
		}
	})

	t.Run("an empty list sends no detail request and is observed as no rulesets", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath: githubtest.OK(repositoryBody),
			rulesetsPath:   list(),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRulesets)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &Rulesets{Existing: Observed([]Ruleset{})}
		if !reflect.DeepEqual(observed.Rulesets, want) {
			t.Errorf("ObserveRepository().Rulesets = %+v, want %+v", observed.Rulesets, want)
		}
		if got, wantRequests := routes.Requests(), []string{repositoryPath + "?", rulesetsList}; !slices.Equal(got, wantRequests) {
			t.Errorf("requests = %q, want %q", got, wantRequests)
		}
	})

	failures := []struct {
		name         string
		routes       map[string]githubtest.Response
		want         error
		wantRequests []string
	}{
		{
			name:         "names that differ only in ASCII case send no detail request",
			routes:       map[string]githubtest.Response{rulesetsPath: list(githubtest.RulesetSummaryBody{ID: 1, Name: "Protect"}, githubtest.RulesetSummaryBody{ID: 2, Name: "protect"})},
			want:         ErrUnexpectedResponse,
			wantRequests: []string{rulesetsList},
		},
		{
			name:         "forbidden list on a public repository is forbidden",
			routes:       map[string]githubtest.Response{rulesetsPath: {Status: http.StatusForbidden}},
			want:         ErrForbidden,
			wantRequests: []string{rulesetsList},
		},
		{
			name: "not found first detail is not found and the second detail is not sent",
			routes: map[string]githubtest.Response{
				rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "a"}, githubtest.RulesetSummaryBody{ID: 2, Name: "b"}),
				rulesetsPath + "/1": {Status: http.StatusNotFound},
			},
			want:         ErrNotFound,
			wantRequests: []string{rulesetsList, rulesetsPath + "/1?"},
		},
		{
			name: "a detail with another id is unexpected response",
			routes: map[string]githubtest.Response{
				rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "a"}),
				rulesetsPath + "/1": detail(9, "a"),
			},
			want:         ErrUnexpectedResponse,
			wantRequests: []string{rulesetsList, rulesetsPath + "/1?"},
		},
		{
			name: "a detail that breaks the contract is unexpected response",
			routes: map[string]githubtest.Response{
				rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "a"}),
				rulesetsPath + "/1": githubtest.OK(`{}`),
			},
			want:         ErrUnexpectedResponse,
			wantRequests: []string{rulesetsList, rulesetsPath + "/1?"},
		},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			routes := &escapedPathRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRulesets)
			const wantFetch = "fetch rulesets owner/repo"
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, wantFetch)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
			wantRequests := append([]string{repositoryPath + "?"}, tc.wantRequests...)
			if got := routes.Requests(); !slices.Equal(got, wantRequests) {
				t.Errorf("requests = %q, want %q", got, wantRequests)
			}
		})
	}

	t.Run("a failed detail names the ruleset", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:      githubtest.OK(repositoryBody),
			rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "release/*"}),
			rulesetsPath + "/1": {Status: http.StatusNotFound},
		}}
		c := newTestClient(t, routes.ServeHTTP)

		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRulesets)
		if want := `ruleset "release/*"`; err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ObserveRepository() error = %v, want it to contain %s", err, want)
		}
	})
}
