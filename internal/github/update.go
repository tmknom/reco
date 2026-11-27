package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// updateRequestFunc は、reference のリポジトリの設定を value にする更新要求を一度送る。
// メソッド式 ((*Client).updateAllowAutoMerge など) をそのまま渡せるよう、受け手の Client を先頭に置く。
type updateRequestFunc func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error)

// Update は、reference のリポジトリの key の設定を value にする更新要求を、UpdateEndpointOf(key) へ一度だけ送る。再送はしない。
// error が nil になるのは UpdateSucceeded のときだけで、呼び出し側は error から結果を導かない。
// マージ方式は一つずつ送る経路を持たず (UpdateMergeMethods)、渡されたら不変条件の破れとして要求を送る前に panic する。
func (c *Client) Update(ctx context.Context, reference repository.Reference, key setting.Key, value setting.Value) (UpdateOutcome, error) {
	request := updateRequestOf(key)
	if request.send == nil {
		panic("github: " + string(key) + " has no update request of its own")
	}
	return request.send(c, ctx, reference, value)
}

// UpdateMergeMethods は、reference のリポジトリのマージ方式を allowed の値にする更新要求を、一つの PATCH で一度だけ送る。再送はしない。
// allowed にないマージ方式は送らず、省略したフィールドの保持に依拠する。
// 空の allowed と、マージ方式でない設定を含む allowed は、不変条件の破れとして要求を送る前に panic する。
func (c *Client) UpdateMergeMethods(ctx context.Context, reference repository.Reference, allowed map[setting.Key]bool) (UpdateOutcome, error) {
	if len(allowed) == 0 {
		panic("github: no merge method to update")
	}
	fields := make(map[string]bool, len(allowed))
	for key, allow := range allowed {
		fields[mergeMethodField(key)] = allow
	}
	return c.updateRepository(ctx, reference, mustMarshalJSON(fields))
}

// updateAllowAutoMerge は reference のリポジトリの allow_auto_merge を allow に更新する。
func (c *Client) updateAllowAutoMerge(ctx context.Context, reference repository.Reference, allow bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"allow_auto_merge":%t}`, allow))
}

// updateDeleteBranchOnMerge は reference のリポジトリの delete_branch_on_merge を enabled に更新する。
func (c *Client) updateDeleteBranchOnMerge(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"delete_branch_on_merge":%t}`, enabled))
}

// updateAllowUpdateBranch は reference のリポジトリの allow_update_branch を allow に更新する。
func (c *Client) updateAllowUpdateBranch(ctx context.Context, reference repository.Reference, allow bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"allow_update_branch":%t}`, allow))
}

// updateSquashMergeCommitDefault は reference のリポジトリの squash merge のマージメッセージを d に更新する。
// タイトルと本文の両方だけを送り、マージ方式のフィールドは送らない。
func (c *Client) updateSquashMergeCommitDefault(ctx context.Context, reference repository.Reference, d setting.SquashMergeCommitDefault) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, mustMarshalJSON(map[string]string{
		"squash_merge_commit_title":   string(d.Title),
		"squash_merge_commit_message": string(d.Message),
	}))
}

// updateMergeCommitDefault は reference のリポジトリの merge commit のマージメッセージを d に更新する。
func (c *Client) updateMergeCommitDefault(ctx context.Context, reference repository.Reference, d setting.MergeCommitDefault) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, mustMarshalJSON(map[string]string{
		"merge_commit_title":   string(d.Title),
		"merge_commit_message": string(d.Message),
	}))
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

// updateForkPullRequestApprovalPolicy は reference のリポジトリの、Fork Pull Request ワークフローの承認ポリシーを policy に更新する。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateForkPullRequestApprovalPolicy(ctx context.Context, reference repository.Reference, policy setting.ForkPullRequestApprovalPolicy) (UpdateOutcome, error) {
	// policy は英小文字と _ だけの定義済みの値で、%q の表記が JSON の文字列と一致する
	body := fmt.Appendf(nil, `{"approval_policy":%q}`, policy)
	status, _, err := c.send(ctx, http.MethodPut, c.forkPullRequestApprovalURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update fork pull request approval", reference, status, err)
}

// updateRetentionDays は reference のリポジトリの保持日数を days に更新する。上限は設定ではないため送らない。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateRetentionDays(ctx context.Context, reference repository.Reference, days int) (UpdateOutcome, error) {
	body := fmt.Appendf(nil, `{"days":%d}`, days)
	status, _, err := c.send(ctx, http.MethodPut, c.retentionURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update retention", reference, status, err)
}

// updateOIDCSubjectCustomization は reference のリポジトリの subject のカスタマイズを customization に更新する。
func (c *Client) updateOIDCSubjectCustomization(ctx context.Context, reference repository.Reference, customization setting.OIDCSubjectCustomization) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.oidcSubjectURL(reference), encodeOIDCSubjectCustomization(customization), http.StatusCreated)
	return describeUpdateOutcome("update oidc subject", reference, status, err)
}

// encodeOIDCSubjectCustomization は、subject のカスタマイズの更新要求の本文を組み立てる。
// 真偽値は false も送るため、omitempty を使わずに必要な項目だけを入れる。
// key を送るのは既定の形を使わないときだけで、nil のときも null にせず空の配列で送る。
// use_immutable_subject は、宣言したときだけ送り、省略したときは GitHub が保持する値に任せる。
func encodeOIDCSubjectCustomization(customization setting.OIDCSubjectCustomization) []byte {
	body := map[string]any{"use_default": customization.UseDefault}
	if !customization.UseDefault {
		body["include_claim_keys"] = append([]string{}, customization.IncludeClaimKeys...)
	}
	if customization.UseImmutableSubject != nil {
		body["use_immutable_subject"] = *customization.UseImmutableSubject
	}
	return mustMarshalJSON(body)
}

// updatePrivateForkPullRequestWorkflowConditions は reference のリポジトリの、非公開リポジトリの Fork Pull Request ワークフローの実行条件を conditions に更新する。
// 4項目をすべて一つの要求で送り、省略したフィールドの保持に依拠しない（Actions の設計の更新方式）。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updatePrivateForkPullRequestWorkflowConditions(ctx context.Context, reference repository.Reference, conditions setting.PrivateForkPullRequestWorkflowConditions) (UpdateOutcome, error) {
	body := mustMarshalJSON(struct {
		RunWorkflowsFromForkPullRequests           bool `json:"run_workflows_from_fork_pull_requests"`
		SendWriteTokensToWorkflows                 bool `json:"send_write_tokens_to_workflows"`
		SendSecretsAndVariables                    bool `json:"send_secrets_and_variables"`
		RequireApprovalForForkPullRequestWorkflows bool `json:"require_approval_for_fork_pr_workflows"`
	}{conditions.RunWorkflowsFromForkPullRequests, conditions.SendWriteTokensToWorkflows, conditions.SendSecretsAndVariables, conditions.RequireApprovalForForkPullRequestWorkflows})
	status, _, err := c.send(ctx, http.MethodPut, c.privateForkPullRequestWorkflowsURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update private fork pull request workflows", reference, status, err)
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

// updateRequest は、一つの設定を変える更新要求の、送り先の更新エンドポイントと送り方の組。
// 送る更新要求と比べる設定の範囲を別々の場所で足し忘れないよう、同じ switch から返す。
// 一つずつ送る経路を持たない設定 (マージ方式) では send が nil である。
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
	case setting.AllowUpdateBranch:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateAllowUpdateBranch)}
	case setting.SquashMergeCommit:
		return updateRequest{RepositoryEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateSquashMergeCommitDefault(ctx, reference, value.SquashMergeCommitDefault())
		}}
	case setting.MergeCommit:
		return updateRequest{RepositoryEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateMergeCommitDefault(ctx, reference, value.MergeCommitDefault())
		}}
	case setting.AllowSquashMerge, setting.AllowMergeCommit, setting.AllowRebaseMerge:
		return updateRequest{endpoint: RepositoryEndpoint}
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
	case setting.ForkPullRequestApproval:
		return updateRequest{ForkPullRequestApprovalEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateForkPullRequestApprovalPolicy(ctx, reference, value.ForkPullRequestApprovalPolicy())
		}}
	case setting.PrivateForkPullRequestWorkflows:
		return updateRequest{PrivateForkPullRequestWorkflowsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updatePrivateForkPullRequestWorkflowConditions(ctx, reference, value.PrivateForkPullRequestWorkflowConditions())
		}}
	case setting.RetentionDays:
		return updateRequest{RetentionEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateRetentionDays(ctx, reference, value.RetentionDays())
		}}
	case setting.OIDCSubject:
		return updateRequest{OIDCSubjectEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateOIDCSubjectCustomization(ctx, reference, value.OIDCSubjectCustomization())
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
	case setting.RepositoryVariable, setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy, setting.EnvironmentRequiredReviewers, setting.EnvironmentPreventSelfReview, setting.EnvironmentWaitTimer, setting.EnvironmentVariable, setting.RulesetResource:
		// 関連リソースの更新要求は宛先に名前か ID が要るため、設定の識別だけで引くこの関数では扱えない。
	}
	panic("github: no update request for " + string(key))
}

// boolUpdate は、真偽値を受ける更新メソッドを、設定の値を受ける更新要求にする。
func boolUpdate(update func(*Client, context.Context, repository.Reference, bool) (UpdateOutcome, error)) updateRequestFunc {
	return func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
		return update(c, ctx, reference, value.Bool())
	}
}

// mergeMethodField は、マージ方式 key の、更新要求の本文のフィールド名を返す。
// マージ方式でない設定は、不変条件の破れとして panic する。
func mergeMethodField(key setting.Key) string {
	switch key {
	case setting.AllowSquashMerge:
		return "allow_squash_merge"
	case setting.AllowMergeCommit:
		return "allow_merge_commit"
	case setting.AllowRebaseMerge:
		return "allow_rebase_merge"
	default:
		panic("github: " + string(key) + " is not a merge method")
	}
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
