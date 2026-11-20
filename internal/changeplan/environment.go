package changeplan

import (
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// environmentPlans は、一つの Environment の宣言 e を、宣言の名前で引いた観測結果 observed と照合し、存在、デプロイ元制限の順に計画を返す。
// 宣言は存在することの期待値を兼ねるため、存在の計画は必ず返す。
// デプロイ元制限は宣言したときだけ返し、存在しない Environment では属する先が存在しないことを理由とする比較不能になる。
func environmentPlans(e configuration.EnvironmentDeclaration, observed github.EnvironmentObservation) []SettingPlan {
	result := NotComparable
	if _, ok := observed.Existing(); ok {
		result = Match
	} else if observed.Absent() {
		result = Mismatch
	}
	plans := []SettingPlan{{Key: setting.EnvironmentResource, Environment: e.Name, ObservedEnvironment: observed, Result: result}}
	if e.DeploymentBranchPolicy != nil {
		policy := newSettingPlan(setting.EnvironmentDeploymentBranchPolicy, observed.DeploymentBranchPolicy(), setting.DeploymentBranchPolicyValue(*e.DeploymentBranchPolicy))
		policy.Environment = e.Name
		plans = append(plans, policy)
	}
	return plans
}
