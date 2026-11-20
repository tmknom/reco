package show

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeEnvironments は Environments の節を書く。
// 一つもないときも節を省かず、存在しないと確かめたことと、表示しないことを区別できるようにする。
func writeEnvironments(b *strings.Builder, e github.Environments) {
	if len(e.Existing) == 0 {
		b.WriteString("Environments: none\n")
		return
	}
	b.WriteString("Environments:\n")
	for _, environment := range e.Existing {
		// 名前は任意の文字を含み得るため、制御文字をエスケープして一行に収める
		fmt.Fprintf(b, "  - name: %s\n    deployment_branch_policy:\n      mode: %s\n", strconv.Quote(environment.Name), environment.DeploymentBranchPolicy.Mode)
		if environment.DeploymentBranchPolicy.Mode == setting.DeploymentBranchPolicySelected {
			fmt.Fprintf(b, "      patterns: %s\n", environment.DeploymentBranchPolicy.PatternsString())
		}
		writeEnvironmentProtection(b, environment.Protection)
	}
}

// writeEnvironmentProtection は Environment の保護設定を書く。保護ルールがなければ none と示す。
func writeEnvironmentProtection(b *strings.Builder, p setting.EnvironmentProtection) {
	if p.WaitTimer == nil {
		b.WriteString("    wait_timer: none\n")
	} else {
		fmt.Fprintf(b, "    wait_timer: %d\n", *p.WaitTimer)
	}
	if p.RequiredReviewers == nil {
		b.WriteString("    required_reviewers: none\n")
	} else {
		reviewers := make([]string, len(p.RequiredReviewers.Reviewers))
		for i, reviewer := range p.RequiredReviewers.Reviewers {
			reviewers[i] = fmt.Sprintf("%s %d", reviewer.Type, reviewer.ID)
		}
		fmt.Fprintf(b, "    required_reviewers:\n      reviewers: [%s]\n      prevent_self_review: %t\n",
			strings.Join(reviewers, ", "), p.RequiredReviewers.PreventSelfReview)
	}
	fmt.Fprintf(b, "    can_admins_bypass: %t\n", p.CanAdminsBypass)
}
