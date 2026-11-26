package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
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

// TestMergeMethodsChange は、マージ方式の変更予定を、一つの更新要求で送る一つの論理的な設定変更にまとめる規則を確かめる (General の設計の論理的な設定変更と更新操作)。
//
//trace:checks INV-001 INV-004 FR-010 FR-012
func TestMergeMethodsChange(t *testing.T) {
	plan := func(key setting.Key, observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: key}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	desired := func(key setting.Key, value bool) configuration.ManagedSetting {
		return configuration.ManagedSetting{Unit: setting.Unit{Key: key}, Desired: setting.BoolValue(value)}
	}
	basis := func(key setting.Key, observed bool) PreconditionBasis {
		return PreconditionBasis{Unit: setting.Unit{Key: key}, AnticipatedValue: AnticipateValue(setting.BoolValue(observed))}
	}
	// change は、期待状態 state を一つの更新操作で送り、その計画時の観測値 anticipated を基準にする論理的な設定変更。
	change := func(state []configuration.ManagedSetting, anticipated ...PreconditionBasis) LogicalChange {
		return LogicalChange{
			DesiredState: state,
			Operations:   []UpdateOperation{{Kind: MergeMethodsUpdate, Unit: state[0].Unit, Settings: state, PreconditionBases: anticipated}},
		}
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     LogicalChange
	}{
		{
			name:     "swapping the enabled merge method is one update relevant to both observed values",
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, true), plan(setting.AllowMergeCommit, true, false)},
			want: change([]configuration.ManagedSetting{desired(setting.AllowSquashMerge, true), desired(setting.AllowMergeCommit, false)},
				basis(setting.AllowSquashMerge, false), basis(setting.AllowMergeCommit, true)),
		},
		{
			name:     "one merge method change is the same kind of update",
			settings: []SettingPlan{plan(setting.AllowRebaseMerge, true, false)},
			want:     change([]configuration.ManagedSetting{desired(setting.AllowRebaseMerge, false)}, basis(setting.AllowRebaseMerge, true)),
		},
		{
			// 一致したマージ方式を送ると、宣言どおりの値でも承認していない更新要求を送ることになり、変更前提にも計画にない値が入るため
			name:     "matching merge method is neither sent nor relevant",
			settings: []SettingPlan{plan(setting.AllowSquashMerge, false, true), plan(setting.AllowMergeCommit, true, true)},
			want:     change([]configuration.ManagedSetting{desired(setting.AllowSquashMerge, true)}, basis(setting.AllowSquashMerge, false)),
		},
		{
			name:     "settings other than merge methods are not bundled",
			settings: []SettingPlan{plan(setting.AllowAutoMerge, false, true), plan(setting.AllowMergeCommit, false, true), plan(setting.AllowUpdateBranch, false, true)},
			want:     change([]configuration.ManagedSetting{desired(setting.AllowMergeCommit, true)}, basis(setting.AllowMergeCommit, false)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mergeMethodsChange(tc.settings); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mergeMethodsChange() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}

	t.Run("no merge method change panics", func(t *testing.T) {
		// 空の期待状態を引いた runtime error ではなく、不変条件の破れとして止まることを確かめる
		defer func() {
			if _, ok := recover().(string); !ok {
				t.Error("mergeMethodsChange() did not panic as a broken invariant")
			}
		}()
		_ = mergeMethodsChange([]SettingPlan{plan(setting.AllowSquashMerge, true, true)})
	})
}
