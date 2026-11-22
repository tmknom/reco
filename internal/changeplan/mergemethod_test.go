package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-006 INV-001
func TestMergeMethodsLeftDisabled(t *testing.T) {
	plan := func(key setting.Key, observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: key}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	observation := func(squash, mergeCommit, rebase bool) github.ObservationResult {
		return github.ObservationResult{General: github.General{AllowSquashMerge: squash, AllowMergeCommit: mergeCommit, AllowRebaseMerge: rebase}}
	}
	disableSquash := plan(setting.AllowSquashMerge, true, false)
	disableMergeCommit := plan(setting.AllowMergeCommit, true, false)
	cases := []struct {
		name string
		in   SettingPlan
		// settings は、in と同じリポジトリの計画に並ぶ管理設定の計画。in を含む。
		settings []SettingPlan
		observed github.ObservationResult
		want     []setting.Key
	}{
		{
			name:     "disabling two with the undeclared one enabled leaves a merge method enabled",
			in:       disableSquash,
			settings: []SettingPlan{disableSquash, disableMergeCommit},
			observed: observation(true, true, true),
			want:     nil,
		},
		{
			name:     "disabling two with the undeclared one disabled returns the undeclared one",
			in:       disableMergeCommit,
			settings: []SettingPlan{disableSquash, disableMergeCommit},
			observed: observation(true, true, false),
			want:     []setting.Key{setting.AllowRebaseMerge},
		},
		{
			name:     "disabling one with the two undeclared disabled returns both in the configuration order",
			in:       plan(setting.AllowRebaseMerge, true, false),
			settings: []SettingPlan{plan(setting.AllowRebaseMerge, true, false)},
			observed: observation(false, false, true),
			want:     []setting.Key{setting.AllowSquashMerge, setting.AllowMergeCommit},
		},
		{
			name:     "disabling side of a swap counts the declared true and is not left disabled",
			in:       disableMergeCommit,
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, true), disableMergeCommit},
			observed: observation(false, true, false),
			want:     nil,
		},
		{
			name:     "enabling side of a swap is not judged",
			in:       plan(setting.AllowSquashMerge, false, true),
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, true), disableMergeCommit},
			observed: observation(false, true, false),
			want:     nil,
		},
		{
			name:     "matching merge method declared false is not judged",
			in:       plan(setting.AllowSquashMerge, false, false),
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, false), disableMergeCommit},
			observed: observation(false, true, false),
			want:     nil,
		},
		{
			name:     "declared matching merge method is not counted as left disabled",
			in:       disableMergeCommit,
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, false), disableMergeCommit},
			observed: observation(false, true, false),
			want:     []setting.Key{setting.AllowRebaseMerge},
		},
		{
			name:     "allow_update_branch change is not judged",
			in:       plan(setting.AllowUpdateBranch, true, false),
			settings: []SettingPlan{plan(setting.AllowUpdateBranch, true, false)},
			observed: observation(false, false, false),
			want:     nil,
		},
		{
			name:     "allow_update_branch declared true does not count as a merge method left enabled",
			in:       disableMergeCommit,
			settings: []SettingPlan{disableSquash, disableMergeCommit, plan(setting.AllowUpdateBranch, false, true)},
			observed: observation(true, true, false),
			want:     []setting.Key{setting.AllowRebaseMerge},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mergeMethodsLeftDisabled(tc.in, tc.settings, tc.observed); !slices.Equal(got, tc.want) {
				t.Errorf("mergeMethodsLeftDisabled() = %v, want %v", got, tc.want)
			}
		})
	}

	// 3 つとも false の宣言は入力検証が拒否するため、届けば不変条件の破れである
	t.Run("every merge method declared false panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("mergeMethodsLeftDisabled() did not panic for every merge method declared false")
			}
		}()
		settings := []SettingPlan{disableSquash, disableMergeCommit, plan(setting.AllowRebaseMerge, false, false)}
		mergeMethodsLeftDisabled(disableSquash, settings, observation(true, true, false))
	})
}
