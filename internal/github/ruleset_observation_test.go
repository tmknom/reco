package github

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 FR-005
func TestRulesetObservation_SettingObservation(t *testing.T) {
	parts := []UninterpretedPart{{Kind: UninterpretedRuleType, Name: "non_fast_forward"}}
	cases := []struct {
		name        string
		observation RulesetObservation
		want        SettingObservation[setting.Value]
	}{
		{
			name:        "an existing interpreted ruleset is observed as its value",
			observation: RulesetObservation{state: resourceExisting, ruleset: InterpretedRuleset(1, "protect", interpretedRulesetValue())},
			want:        Observed(setting.RulesetValue(interpretedRulesetValue())),
		},
		{
			name:        "an existing ruleset with uninterpreted parts is unavailable as uninterpreted parts",
			observation: RulesetObservation{state: resourceExisting, ruleset: UninterpretedRuleset(1, "protect", parts)},
			want:        Unavailable[setting.Value](UnavailableUninterpretedParts),
		},
		{
			name:        "an absent ruleset is observed as absent",
			observation: RulesetObservation{state: resourceAbsent},
			want:        Observed(setting.Absent()),
		},
		{
			name:        "a private repository is unavailable as private",
			observation: RulesetObservation{state: resourceUnavailable, reason: UnavailablePrivateRepository},
			want:        Unavailable[setting.Value](UnavailablePrivateRepository),
		},
		{
			name:        "an undetermined name is unavailable as the name undetermined",
			observation: RulesetObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined},
			want:        Unavailable[setting.Value](UnavailableNameUndetermined),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.observation.SettingObservation(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SettingObservation() = %+v, want %+v", got, tc.want)
			}
		})
	}

	panics := []struct {
		name        string
		observation RulesetObservation
	}{
		{name: "the zero value panics", observation: RulesetObservation{}},
		{name: "an existing ruleset whose value was not read panics", observation: RulesetObservation{state: resourceExisting, ruleset: unreadRuleset(1, "protect")}},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("SettingObservation() did not panic")
				}
			}()
			tc.observation.SettingObservation()
		})
	}
}

//trace:checks FR-004
func TestRulesetObservation_UninterpretedParts(t *testing.T) {
	parts := []UninterpretedPart{{Kind: UninterpretedTarget, Name: "tag"}, {Kind: UninterpretedRuleType, Name: "non_fast_forward"}}
	cases := []struct {
		name        string
		observation RulesetObservation
		want        []UninterpretedPart
	}{
		{name: "an existing ruleset with uninterpreted parts has its parts", observation: RulesetObservation{state: resourceExisting, ruleset: UninterpretedRuleset(1, "protect", parts)}, want: parts},
		{name: "an existing interpreted ruleset has none", observation: RulesetObservation{state: resourceExisting, ruleset: InterpretedRuleset(1, "protect", interpretedRulesetValue())}},
		{name: "an absent ruleset has none", observation: RulesetObservation{state: resourceAbsent}},
		{name: "an unavailable ruleset has none", observation: RulesetObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.observation.UninterpretedParts(); !slices.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Errorf("UninterpretedParts() = %+v, want %+v", got, tc.want)
			}
		})
	}

	t.Run("an existing ruleset whose value was not read panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UninterpretedParts() did not panic")
			}
		}()
		RulesetObservation{state: resourceExisting, ruleset: unreadRuleset(1, "protect")}.UninterpretedParts()
	})
}
