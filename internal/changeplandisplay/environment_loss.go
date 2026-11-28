package changeplandisplay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// environmentDeletedValueLines は、Environment の削除の変更予定 s が失う値の5項目を、1行ずつ返す。
// 値の表記は、同じ設定の変更予定の現在値と同じにする。
// 失われる値を持たない計画は空を返す。
func environmentDeletedValueLines(s changeplan.SettingPlan) string {
	if s.EnvironmentLoss == nil {
		return ""
	}
	v := s.EnvironmentLoss.DeletedValue
	return fmt.Sprintf("    deployment_branch_policy: %s\n    reviewers: %s\n    prevent_self_review: %s\n    wait_timer: %s\n    can_admins_bypass: %s\n",
		setting.DeploymentBranchPolicyValue(v.DeploymentBranchPolicy).String(), setting.ReviewerSetValue(v.ReviewerSet).String(),
		setting.BoolValue(v.PreventSelfReview).String(), setting.WaitTimerValue(v.WaitTimer).String(), setting.BoolValue(v.CanAdminsBypass).String())
}

// environmentVariablesText は、削除で一緒に消える変数を、二重引用符で囲んでエスケープした名前と値の並びで返す。
// 名前も値も利用者か GitHub が決める文字列のため、改行や制御文字が変更計画の行を偽装しないようにする。
func environmentVariablesText(variables []github.Variable) string {
	texts := make([]string, len(variables))
	for i, v := range variables {
		texts[i] = strconv.Quote(v.Name) + ": " + strconv.Quote(v.Value)
	}
	return strings.Join(texts, ", ")
}

// environmentSecretsText は、削除で一緒に消える Secret を、二重引用符で囲んでエスケープした名前の並びで返す。
func environmentSecretsText(secrets []github.EnvironmentSecret) string {
	texts := make([]string, len(secrets))
	for i, secret := range secrets {
		texts[i] = strconv.Quote(secret.Name)
	}
	return strings.Join(texts, ", ")
}
