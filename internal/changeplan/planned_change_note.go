package changeplan

import "github.com/tmknom/reco/internal/setting"

// PlannedChangeNote は、変更予定の注記の種類。宣言した変更が GitHub 上で意味する事実を表し、文言は表示が決める。
// ゼロ値は定義済みの注記と取り違えないよう、定数は 1 から始める。
type PlannedChangeNote int

const (
	// NoteForkWorkflowsGetWriteTokens は、Fork からの Pull Request で起動したワークフローに、書き込み権限を持つ GITHUB_TOKEN が渡ること。
	NoteForkWorkflowsGetWriteTokens PlannedChangeNote = iota + 1
	// NoteForkWorkflowsGetSecretsAndVariables は、Fork からの Pull Request で起動したワークフローが、Secrets と Variables を使えること。
	NoteForkWorkflowsGetSecretsAndVariables
	// NoteOIDCSubjectChanges は、OIDC トークンの subject クレームの組み立て方が変わること。
	NoteOIDCSubjectChanges
)

// Notes は、この変更予定に添える変更予定の注記を、YAML 設定仕様の表の並びで返す。なければ nil を返す。
// subject のカスタマイズは、どの項目の違いでも subject の組み立て方が変わるため、不一致なら一つの注記を添える。
// 実行条件の注記では、観測値は実行の許可と合わせて比べるが、期待値は、送信を true とする宣言が実行の許可を伴うことを入力検証が保証するため、送信の値だけで比べる。
// 計画の観測値と期待値だけで決まるため、フィールドに持たず導く。
func (s SettingPlan) Notes() []PlannedChangeNote {
	if s.Unit.Key == setting.OIDCSubject && s.Result == Mismatch {
		return []PlannedChangeNote{NoteOIDCSubjectChanges}
	}
	if s.Unit.Key != setting.PrivateForkPullRequestWorkflows || s.Result != Mismatch {
		return nil
	}
	observed, _ := s.Observed.Value()
	before, after := observed.PrivateForkPullRequestWorkflowConditions(), s.Desired.PrivateForkPullRequestWorkflowConditions()
	var notes []PlannedChangeNote
	if !(before.RunWorkflowsFromForkPullRequests && before.SendWriteTokensToWorkflows) && after.SendWriteTokensToWorkflows {
		notes = append(notes, NoteForkWorkflowsGetWriteTokens)
	}
	if !(before.RunWorkflowsFromForkPullRequests && before.SendSecretsAndVariables) && after.SendSecretsAndVariables {
		notes = append(notes, NoteForkWorkflowsGetSecretsAndVariables)
	}
	return notes
}
