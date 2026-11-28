package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeEnvironments は environments を書く。
func writeEnvironments(b *strings.Builder, e github.Environments) {
	if len(e.Existing) == 0 {
		b.WriteString("    # environments: none\n")
		return
	}
	b.WriteString("    environments:\n")
	for _, environment := range e.Existing {
		// 名前は任意の文字を含み得るため、制御文字をエスケープして一行に収める
		fmt.Fprintf(b, "      - name: %s\n", quote(environment.Name))
		writeDeploymentBranchPolicy(b, environment.DeploymentBranchPolicy())
		writeRequiredReviewers(b, environment.Protection.ReviewerSet)
		writePreventSelfReviewAndWaitTimer(b, environment.Protection)
		writeEnvironmentVariables(b, *environment.Variables)
	}
}

// writeDeploymentBranchPolicy はデプロイ元制限を書く。
// 知らない値のときは、方式だけを示すと個別パターンのない宣言として読めるため、mode と patterns を示さず、キーの行をコメントにする。
func writeDeploymentBranchPolicy(b *strings.Builder, o github.SettingObservation[setting.DeploymentBranchPolicy]) {
	policy, ok := o.Value()
	if !ok {
		requireUnknownValue("deployment_branch_policy", o.UnavailableReason())
		fmt.Fprintf(b, "        # deployment_branch_policy: %s\n", unknownValueText)
		return
	}
	fmt.Fprintf(b, "        deployment_branch_policy:\n          mode: %s\n", policy.Mode)
	if policy.Mode == setting.DeploymentBranchPolicySelected {
		writeDeploymentPatterns(b, policy.Patterns)
	}
}

// writeRequiredReviewers は担当者の集合を書く。担当者がいなければ [] で書く。
// 担当者のいない保護ルールは、設定宣言として読むと [] と一致しないため、コメントで示す。
// 知らない値のときは、その行だけをコメントにする。
func writeRequiredReviewers(b *strings.Builder, o github.SettingObservation[setting.ReviewerSet]) {
	reviewers, ok := o.Value()
	if !ok {
		requireUnknownValue("required_reviewers", o.UnavailableReason())
		fmt.Fprintf(b, "        # required_reviewers: %s\n", unknownValueText)
		return
	}
	if reviewers.IsRuleWithoutReviewers() {
		b.WriteString("        # required_reviewers: no reviewers (a protection rule without reviewers remains)\n")
		return
	}
	list := reviewers.Reviewers()
	if len(list) == 0 {
		b.WriteString("        required_reviewers: []\n")
		return
	}
	b.WriteString("        required_reviewers:\n")
	for _, reviewer := range list {
		fmt.Fprintf(b, "          - type: %s\n            id: %d", reviewer.Type, reviewer.ID)
		if login, ok := reviewers.Login(reviewer); ok {
			fmt.Fprintf(b, " # %s", login)
		}
		b.WriteString("\n")
	}
}

// writePreventSelfReviewAndWaitTimer は自己承認の禁止とデプロイの待機時間を書く。
// 単位は設定宣言に書けず、値の位置に書くと整数として読めないため、行末のコメントで示す。
func writePreventSelfReviewAndWaitTimer(b *strings.Builder, protection github.EnvironmentProtection) {
	fmt.Fprintf(b, "        prevent_self_review: %t\n        wait_timer: %d # minutes\n", protection.PreventSelfReview, protection.WaitTimer)
}

// writeDeploymentPatterns は個別パターンを、空なら [] で、そうでなければ type と name の要素で書く。
func writeDeploymentPatterns(b *strings.Builder, patterns []setting.DeploymentPattern) {
	if len(patterns) == 0 {
		b.WriteString("          patterns: []\n")
		return
	}
	b.WriteString("          patterns:\n")
	for _, pattern := range patterns {
		fmt.Fprintf(b, "            - type: %s\n              name: %s\n", pattern.Type, quote(pattern.Name))
	}
}
