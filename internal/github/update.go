package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// UpdateOutcome は、一つの更新操作について、GitHub の応答から判断した結果である。
// ゼロ値を UpdateUnknown にするのは、結果を設定し損ねた値を成功や拒否と取り違えないため。
type UpdateOutcome int

const (
	UpdateUnknown UpdateOutcome = iota
	UpdateSucceeded
	UpdateRejected
)

// updateRequestFunc は、reference のリポジトリの設定を value にする更新要求を一度送る。
// メソッド式 ((*Client).updateAllowAutoMerge など) をそのまま渡せるよう、受け手の Client を先頭に置く。
type updateRequestFunc func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error)

// Update は、reference のリポジトリの key の設定を value にする更新要求を、UpdateEndpointOf(key) へ一度だけ送る。再送はしない。
// error が nil になるのは UpdateSucceeded のときだけで、呼び出し側は error から結果を導かない。
func (c *Client) Update(ctx context.Context, reference repository.Reference, key setting.Key, value setting.Value) (UpdateOutcome, error) {
	return updateRequestOf(key).send(c, ctx, reference, value)
}

// updateAllowAutoMerge は reference のリポジトリの allow_auto_merge を allow に更新する。
func (c *Client) updateAllowAutoMerge(ctx context.Context, reference repository.Reference, allow bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"allow_auto_merge":%t}`, allow))
}

// updateDeleteBranchOnMerge は reference のリポジトリの delete_branch_on_merge を enabled に更新する。
func (c *Client) updateDeleteBranchOnMerge(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"delete_branch_on_merge":%t}`, enabled))
}

// updateSecretScanningEnabled は reference のリポジトリの Secret scanning の有効状態を enabled に更新する。
// security_and_analysis の中のこの項目だけを送り、もう一方の項目とトップレベルの一般設定の保持に依拠する（Security の設計の更新方式）。
func (c *Client) updateSecretScanningEnabled(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, encodeSecurityAndAnalysis("secret_scanning", enabled))
}

// updateRepositoryPushProtection は reference のリポジトリの push protection を enabled に更新する。
// 送る項目と依拠する保持は updateSecretScanningEnabled と同じ。
func (c *Client) updateRepositoryPushProtection(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, encodeSecurityAndAnalysis("secret_scanning_push_protection", enabled))
}

// updateRepository は body を本文とする更新要求を送る。
// 成功応答の本文は、更新の契約に適合するかを確かめるためだけに解釈する。
func (c *Client) updateRepository(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, data, err := c.send(ctx, http.MethodPatch, c.repositoryURL(reference), body, http.StatusOK)
	if err == nil {
		_, _, err = decodeRepository(data)
	}
	return describeUpdateOutcome("update repository", reference, status, err)
}

// updateDefaultWorkflowPermissions は reference のリポジトリの default_workflow_permissions を level に更新する。
func (c *Client) updateDefaultWorkflowPermissions(ctx context.Context, reference repository.Reference, level setting.PermissionLevel) (UpdateOutcome, error) {
	return c.updateWorkflowPermissions(ctx, reference, fmt.Appendf(nil, `{"default_workflow_permissions":%q}`, level))
}

// updateCanApprovePullRequestReviews は reference のリポジトリの can_approve_pull_request_reviews を canApprove に更新する。
func (c *Client) updateCanApprovePullRequestReviews(ctx context.Context, reference repository.Reference, canApprove bool) (UpdateOutcome, error) {
	return c.updateWorkflowPermissions(ctx, reference, fmt.Appendf(nil, `{"can_approve_pull_request_reviews":%t}`, canApprove))
}

// updateWorkflowPermissions は body を本文とする更新要求を送る。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateWorkflowPermissions(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.workflowPermissionsURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update workflow permissions", reference, status, err)
}

// updateActionsEnabled は reference のリポジトリの Actions の有効状態を enabled に更新する。
// allowed_actions と sha_pinning_required は送らず、GitHub が省略した値を保持することに依拠する（Actions の設計の更新方式）。
func (c *Client) updateActionsEnabled(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":%t}`, enabled))
}

// updateAllowedActions は reference のリポジトリの Actions 許可ポリシーを policy に更新する。
// Actions が無効のリポジトリでは、同じ要求が Actions を有効にする。
func (c *Client) updateAllowedActions(ctx context.Context, reference repository.Reference, policy setting.AllowedActionsPolicy) (UpdateOutcome, error) {
	// policy は英小文字と _ だけの定義済みの値で、%q の表記が JSON の文字列と一致する
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":true,"allowed_actions":%q}`, policy))
}

// updateSHAPinningRequired は reference のリポジトリの sha_pinning_required を required に更新する。
// 必須の enabled には true を送り、allowed_actions は送らない（Actions の設計の更新方式）。
// 無効のリポジトリへ送ると Actions を有効にするため、呼び出し側が更新直前に有効と確かめる。
func (c *Client) updateSHAPinningRequired(ctx context.Context, reference repository.Reference, required bool) (UpdateOutcome, error) {
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":true,"sha_pinning_required":%t}`, required))
}

// updateActionsPermissions は body を本文とする更新要求を送る。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateActionsPermissions(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.actionsPermissionsURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update actions permissions", reference, status, err)
}

// updateAccessLevel は reference のリポジトリの、リポジトリ外のワークフローからのアクセスの範囲を level に更新する。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateAccessLevel(ctx context.Context, reference repository.Reference, level setting.AccessLevel) (UpdateOutcome, error) {
	// level は英小文字だけの定義済みの値で、%q の表記が JSON の文字列と一致する
	body := fmt.Appendf(nil, `{"access_level":%q}`, level)
	status, _, err := c.send(ctx, http.MethodPut, c.actionsAccessURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update actions access", reference, status, err)
}

// updateSelectedActions は reference のリポジトリの選択方式の Actions 許可条件を criteria に更新する。
// GitHub は、要求の前提となる設定の状態が成り立たないとき 409 で拒否する（GitHub Actions 権限 API 仕様の Selected actions）。
func (c *Client) updateSelectedActions(ctx context.Context, reference repository.Reference, criteria setting.SelectedActionsCriteria) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.selectedActionsURL(reference), encodeSelectedActions(criteria), http.StatusNoContent)
	return describeUpdateOutcome("update selected actions", reference, status, err)
}

// updateDependabotAlerts は reference のリポジトリの Dependabot alerts を enabled に更新する。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateDependabotAlerts(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, enablementMethod(enabled), c.vulnerabilityAlertsURL(reference), nil, http.StatusNoContent)
	return describeUpdateOutcome("update dependabot alerts", reference, status, err)
}

// updateDependabotSecurityUpdates は reference のリポジトリの Dependabot security updates を enabled に更新する。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateDependabotSecurityUpdates(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, enablementMethod(enabled), c.automatedSecurityFixesURL(reference), nil, http.StatusNoContent)
	return describeUpdateOutcome("update dependabot security updates", reference, status, err)
}

// updatePrivateVulnerabilityReporting は reference のリポジトリの Private vulnerability reporting を enabled に更新する。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updatePrivateVulnerabilityReporting(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, enablementMethod(enabled), c.privateVulnerabilityReportingURL(reference), nil, http.StatusNoContent)
	return describeUpdateOutcome("update private vulnerability reporting", reference, status, err)
}

// enablementMethod は、有効にする要求と無効にする要求の HTTP メソッドを返す。
// この更新エンドポイントは、設定の値を本文ではなくメソッドで受ける。
func enablementMethod(enabled bool) string {
	if enabled {
		return http.MethodPut
	}
	return http.MethodDelete
}

// CreateEnvironment は、reference のリポジトリに name の Environment を作る PUT を、本文なしで一度だけ送る。再送はしない。
// 同じ名前の Environment が存在すれば、この要求は何も変えずに成功を応答する。
func (c *Client) CreateEnvironment(ctx context.Context, reference repository.Reference, name string) (UpdateOutcome, error) {
	return c.putEnvironment(ctx, "create environment", reference, name, nil)
}

// UpdateEnvironmentDeploymentBranchPolicy は、reference のリポジトリの name の Environment のデプロイ元制限の方式を mode にする PUT を一度だけ送る。再送はしない。
// name の Environment が存在しなければ、この要求は方式が mode の Environment を作る。
// 本文は deployment_branch_policy だけで、保護設定を併送しない。
// selected は方式だけを送り、個別パターンは登録しない。登録は AddDeploymentPattern で一つずつ送る。
// 未定義の方式は、本文を組めない不変条件の破れとして panic する。
func (c *Client) UpdateEnvironmentDeploymentBranchPolicy(ctx context.Context, reference repository.Reference, name string, mode setting.DeploymentBranchPolicyMode) (UpdateOutcome, error) {
	return c.putEnvironment(ctx, "set deployment branch policy of environment", reference, name, encodeDeploymentBranchPolicyMode(mode))
}

// AddDeploymentPattern は、reference のリポジトリの environment の Environment に、個別パターン pattern を加える POST を一度だけ送る。再送はしない。
// 応答の扱いはアーキテクチャ設計の HTTP 通信とリダイレクトの信頼境界と、Environments の設計の更新方式に従う。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) AddDeploymentPattern(ctx context.Context, reference repository.Reference, environment string, pattern setting.DeploymentPattern) (UpdateOutcome, error) {
	u, ok := c.deploymentBranchPoliciesURL(reference, environment)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	status, data, err := c.send(ctx, http.MethodPost, u, encodeDeploymentPattern(pattern), http.StatusOK)
	if err == nil {
		_, _, err = decodeDeploymentPattern(data)
	}
	if status == http.StatusSeeOther {
		// classifyFailure のリポジトリの名前の変更の案内は、この応答の原因と異なり、利用者を誤らせる
		err = fmt.Errorf("%w (HTTP %d): GitHub returns this when the pattern already exists", ErrRedirect, status)
	}
	return describeUpdateOutcome(fmt.Sprintf("add deployment pattern %s %q to environment %q", pattern.Type, pattern.Name, environment), reference, status, err)
}

// RemoveDeploymentPattern は、reference のリポジトリの environment の Environment から、ID が id の個別パターン pattern を取り除く DELETE を、本文なしで一度だけ送る。再送はしない。
// pattern は error で取り除く要素を示すためだけに使う。ID だけでは、どの種類と名前の要素かが分からないため。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) RemoveDeploymentPattern(ctx context.Context, reference repository.Reference, environment string, pattern setting.DeploymentPattern, id DeploymentPatternID) (UpdateOutcome, error) {
	u, ok := c.deploymentBranchPolicyURL(reference, environment, id)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	status, _, err := c.send(ctx, http.MethodDelete, u, nil, http.StatusNoContent)
	return describeUpdateOutcome(fmt.Sprintf("remove deployment pattern %s %q (id %d) from environment %q", pattern.Type, pattern.Name, id, environment), reference, status, err)
}

// putEnvironment は name の Environment へ body を本文とする PUT を送る。body が nil なら本文を送らない。
// 成功応答の本文は、Environment の取得の契約に適合するかを確かめるためだけに解釈する。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) putEnvironment(ctx context.Context, operation string, reference repository.Reference, name string, body []byte) (UpdateOutcome, error) {
	u, ok := c.environmentURL(reference, name)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", name))
	}
	status, data, err := c.send(ctx, http.MethodPut, u, body, http.StatusOK)
	if err == nil {
		_, err = decodeEnvironment(data)
	}
	return describeUpdateOutcome(fmt.Sprintf("%s %q", operation, name), reference, status, err)
}

// updateRequest は、一つの設定を変える更新要求の、送り先の更新エンドポイントと送り方の組。
// 送る更新要求と比べる設定の範囲を別々の場所で足し忘れないよう、同じ switch から返す。
type updateRequest struct {
	endpoint UpdateEndpoint
	send     updateRequestFunc
}

// updateRequestOf は、key の設定を変える更新要求を返す。
// 未定義の設定は、不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func updateRequestOf(key setting.Key) updateRequest {
	switch key {
	case setting.DeleteBranchOnMerge:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateDeleteBranchOnMerge)}
	case setting.AllowAutoMerge:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateAllowAutoMerge)}
	case setting.ActionsEnabled:
		return updateRequest{ActionsPermissionsEndpoint, boolUpdate((*Client).updateActionsEnabled)}
	case setting.AllowedActions:
		return updateRequest{ActionsPermissionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateAllowedActions(ctx, reference, value.AllowedActionsPolicy())
		}}
	case setting.SelectedActions:
		// Actions 許可ポリシーが selected のまま3項目だけを変えるときも、ポリシーの更新要求は送らない。
		// selected を送り直すと、GitHub が github_owned_allowed を true に変えるため。
		return updateRequest{SelectedActionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateSelectedActions(ctx, reference, value.SelectedActionsCriteria())
		}}
	case setting.DefaultWorkflowPermissions:
		return updateRequest{WorkflowPermissionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateDefaultWorkflowPermissions(ctx, reference, value.PermissionLevel())
		}}
	case setting.CanApprovePullRequestReviews:
		return updateRequest{WorkflowPermissionsEndpoint, boolUpdate((*Client).updateCanApprovePullRequestReviews)}
	case setting.SHAPinningRequired:
		return updateRequest{ActionsPermissionsEndpoint, boolUpdate((*Client).updateSHAPinningRequired)}
	case setting.ActionsAccessLevel:
		return updateRequest{ActionsAccessEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateAccessLevel(ctx, reference, value.AccessLevel())
		}}
	case setting.DependabotAlerts:
		return updateRequest{DependabotAlertsEndpoint, boolUpdate((*Client).updateDependabotAlerts)}
	case setting.DependabotSecurityUpdates:
		return updateRequest{DependabotSecurityUpdatesEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateDependabotSecurityUpdates(ctx, reference, value.DependabotSecurityUpdates().Enabled())
		}}
	case setting.SecretScanningEnabled:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateSecretScanningEnabled)}
	case setting.RepositoryPushProtection:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateRepositoryPushProtection)}
	case setting.PrivateVulnerabilityReportingEnabled:
		return updateRequest{PrivateVulnerabilityReportingEndpoint, boolUpdate((*Client).updatePrivateVulnerabilityReporting)}
	case setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy:
		// Environment の更新要求は宛先に名前が要るため、設定の識別だけで引くこの関数では扱えない。
	}
	panic("github: no update request for " + string(key))
}

// boolUpdate は、真偽値を受ける更新メソッドを、設定の値を受ける更新要求にする。
func boolUpdate(update func(*Client, context.Context, repository.Reference, bool) (UpdateOutcome, error)) updateRequestFunc {
	return func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
		return update(c, ctx, reference, value.Bool())
	}
}

// encodeDeploymentBranchPolicyMode は mode を Environment の PUT の本文にする。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func encodeDeploymentBranchPolicyMode(mode setting.DeploymentBranchPolicyMode) []byte {
	switch mode {
	case setting.DeploymentBranchPolicyAll:
		return []byte(`{"deployment_branch_policy":null}`)
	case setting.DeploymentBranchPolicyProtectedBranches:
		return []byte(`{"deployment_branch_policy":{"protected_branches":true,"custom_branch_policies":false}}`)
	case setting.DeploymentBranchPolicySelected:
		return []byte(`{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}`)
	}
	panic(fmt.Sprintf("github: no request body for deployment branch policy %q", mode))
}

// encodeSelectedActions は criteria を Selected actions の更新要求の本文にする。
// 許可パターンは任意の文字列のため、Go の %q ではなく JSON の規則でエスケープする。
func encodeSelectedActions(criteria setting.SelectedActionsCriteria) []byte {
	patterns := criteria.PatternsAllowed
	// encoding/json は nil のスライスを null にするため、空の集合を [] として送るよう長さ 0 のスライスに替える
	if patterns == nil {
		patterns = []string{}
	}
	return mustMarshalJSON(struct {
		GitHubOwnedAllowed bool     `json:"github_owned_allowed"`
		VerifiedAllowed    bool     `json:"verified_allowed"`
		PatternsAllowed    []string `json:"patterns_allowed"`
	}{criteria.GitHubOwnedAllowed, criteria.VerifiedAllowed, patterns})
}

// encodeSecurityAndAnalysis は、security_and_analysis の中の item の status だけを送る PATCH の本文にする。
func encodeSecurityAndAnalysis(item string, enabled bool) []byte {
	return mustMarshalJSON(map[string]map[string]map[string]string{
		"security_and_analysis": {item: {"status": securityStatus(enabled)}},
	})
}

// encodeDeploymentPattern は pattern を個別パターンを加える POST の本文にする。
// type は省略すると GitHub が branch として作るため、種類によらず必ず送る。名前は任意の文字列のため、JSON の規則でエスケープする。
func encodeDeploymentPattern(pattern setting.DeploymentPattern) []byte {
	return mustMarshalJSON(struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}{pattern.Name, string(pattern.Type)})
}

// mustMarshalJSON は v を JSON にする。
// 本文の型は bool、string、[]string と、string を値に持つ map だけを持ち、json.Marshal がどの値でも失敗しないため、失敗を不変条件の破れとして panic する。
// encoding/json に Must の関数がないため、この包みを置く。
func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("github: marshal request body: %v", err))
	}
	return data
}

// describeUpdateOutcome は更新操作の結果を判定し、失敗と成否不明の error に operation と対象を添える。
func describeUpdateOutcome(operation string, reference repository.Reference, status int, err error) (UpdateOutcome, error) {
	outcome := updateOutcomeOf(status, err)
	if outcome == UpdateSucceeded {
		return outcome, nil
	}
	if outcome == UpdateUnknown {
		err = fmt.Errorf("update outcome unknown: %w", err)
	}
	return outcome, fmt.Errorf("%s %s: %w", operation, reference, err)
}

// updateOutcomeOf は、更新要求の応答のステータスと失敗から更新操作の結果を判定する。
// 拒否とみなすのは 4xx に限る。応答を受け取れなかったときや 2xx・3xx・5xx の失敗は、
// GitHub が更新要求を処理したかを確かめられないため成否不明とする。
func updateOutcomeOf(status int, err error) UpdateOutcome {
	switch {
	case err == nil:
		return UpdateSucceeded
	case http.StatusBadRequest <= status && status < http.StatusInternalServerError:
		return UpdateRejected
	default:
		return UpdateUnknown
	}
}
