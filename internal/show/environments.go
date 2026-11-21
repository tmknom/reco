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
		fmt.Fprintf(b, "      - name: %s\n        deployment_branch_policy:\n          mode: %s\n", quote(environment.Name), environment.DeploymentBranchPolicy.Mode)
		if environment.DeploymentBranchPolicy.Mode == setting.DeploymentBranchPolicySelected {
			writeDeploymentPatterns(b, environment.DeploymentBranchPolicy.Patterns)
		}
	}
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
