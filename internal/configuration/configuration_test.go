package configuration

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// minimal は受理される最小の宣言。宣言全体を入力にするテストの基準にする。
const minimal = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

// decodeSnippet は、検証の各段階に渡す YAML 文書を src から作る。
func decodeSnippet(t *testing.T, src string) document {
	t.Helper()
	doc, err := decodeDocument([]byte(src))
	if err != nil {
		t.Fatalf("decodeDocument(%q) error = %v", src, err)
	}
	return doc
}

//trace:checks FR-001
func TestDesiredState_Value(t *testing.T) {
	cases := []struct {
		name    string
		desired DesiredState
		key     setting.Key
		want    setting.Value
		wantOK  bool
	}{
		{name: "declared delete_branch_on_merge returns its desired value", desired: DesiredState{General: General{DeleteBranchOnMerge: new(false)}}, key: setting.DeleteBranchOnMerge, want: setting.BoolValue(false), wantOK: true},
		{name: "declared allow_auto_merge returns its desired value", desired: DesiredState{General: General{AllowAutoMerge: new(true)}}, key: setting.AllowAutoMerge, want: setting.BoolValue(true), wantOK: true},
		{name: "declared default_workflow_permissions returns its desired value", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)}}, key: setting.DefaultWorkflowPermissions, want: setting.PermissionLevelValue(setting.PermissionRead), wantOK: true},
		{name: "declared can_approve_pull_request_reviews returns its desired value", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(false)}}, key: setting.CanApprovePullRequestReviews, want: setting.BoolValue(false), wantOK: true},
		{name: "declared allowed_actions returns its desired value", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsLocalOnly)}}, key: setting.AllowedActions, want: setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), wantOK: true},
		{name: "declared selected_actions returns its desired value with empty patterns kept non-nil", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsSelected), SelectedActions: &setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{}}}}, key: setting.SelectedActions, want: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{}}), wantOK: true},
		{name: "omitted allowed_actions is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.AllowedActions, wantOK: false},
		{name: "omitted selected_actions is outside the management scope", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}}, key: setting.SelectedActions, wantOK: false},
		{name: "omitted delete_branch_on_merge is outside the management scope", desired: DesiredState{General: General{AllowAutoMerge: new(true)}}, key: setting.DeleteBranchOnMerge, wantOK: false},
		{name: "omitted allow_auto_merge is outside the management scope", desired: DesiredState{General: General{DeleteBranchOnMerge: new(true)}}, key: setting.AllowAutoMerge, wantOK: false},
		{name: "omitted default_workflow_permissions is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.DefaultWorkflowPermissions, wantOK: false},
		{name: "omitted can_approve_pull_request_reviews is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)}}, key: setting.CanApprovePullRequestReviews, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.desired.Value(tc.key)
			if !reflect.DeepEqual(got, tc.want) || ok != tc.wantOK {
				t.Errorf("Value(%q) = %v, %v, want %v, %v", tc.key, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestDesiredState_ValueCoversEveryDesiredValue は、期待状態が持つ期待値の数と、Value が返す期待値の数が食い違わないことを確かめる。
// 期待状態に設定を足して setting.Keys や Value に足し忘れると、宣言を受理した設定が照合されないまま
// コンパイルもほかのテストも通るため、期待状態の全フィールドを埋めて数える。
func TestDesiredState_ValueCoversEveryDesiredValue(t *testing.T) {
	var desired DesiredState
	declared := fillDesiredValues(reflect.ValueOf(&desired).Elem())

	n := 0
	for _, key := range setting.Keys() {
		if _, ok := desired.Value(key); ok {
			n++
		}
	}
	if n != declared {
		t.Errorf("Value() returned %d desired values for %d declared settings", n, declared)
	}
}

// fillDesiredValues は、v の中の期待値 (ポインタのフィールド) をすべてゼロ値への参照で埋め、その数を返す。
// 列挙値のゼロ値は定義外の値だが、ここでは期待値の有無だけを数える。
func fillDesiredValues(v reflect.Value) int {
	n := 0
	for _, field := range v.Fields() {
		switch field.Kind() {
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
			n++
		case reflect.Struct:
			n += fillDesiredValues(field)
		default:
			panic("unexpected field kind in desired state: " + field.Kind().String())
		}
	}
	return n
}

func TestDesiredState_ValueOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Value() did not panic for an unknown setting")
		}
	}()
	_, _ = DesiredState{}.Value("general.unknown")
}
