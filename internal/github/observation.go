package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// ObservationResult は、一回の観測で確認できた、リポジトリ指定が指す実体の ID と、読み取った設定の領域の設定ごとの、設定の観測結果である。
// 関連リソースを読み取った設定の領域については、関連リソースの観測結果も含む。
type ObservationResult struct {
	RepositoryID repository.ID
	General      General
	// ActionsPermissions、WorkflowPermissions、ActionsAccess は、その設定の領域を選ばなかった観測では nil になる。
	// 読み取っていない値を false や read として読ませず、観測不能とも取り違えさせないため、値型にしない。
	ActionsPermissions  *ActionsPermissions
	WorkflowPermissions *WorkflowPermissions
	ActionsAccess       *ActionsAccess
	// Environments は、その設定の領域を選ばなかった観測では nil になる。Environment が一つもないことと取り違えさせないため、値型にしない。
	Environments *Environments
}

// ActionsPermissions は Actions の実行許可のうち、reco が観測する設定である。
// Actions の有効状態は常に観測値を持ち、ほかの設定は取得条件により観測不能になり得る。
type ActionsPermissions struct {
	Enabled            bool
	AllowedActions     SettingObservation[setting.AllowedActionsPolicy]
	SHAPinningRequired SettingObservation[bool]
	SelectedActions    SettingObservation[setting.SelectedActionsCriteria]
}

// WorkflowPermissions はワークフロー権限のうち、reco が観測する値である。
type WorkflowPermissions struct {
	DefaultWorkflowPermissions   setting.PermissionLevel
	CanApprovePullRequestReviews bool
}

// ActionsAccess はリポジトリ外のワークフローからのアクセスのうち、reco が観測する設定である。
type ActionsAccess struct {
	AccessLevel SettingObservation[setting.AccessLevel]
}

// General はリポジトリ一般設定のうち、reco が観測する値である。管理範囲を知らないため、未管理設定の値も含む。
type General struct {
	AllowAutoMerge      bool
	DeleteBranchOnMerge bool
}

// Setting は、key の設定の観測結果を返す。取得条件を持たない設定は、常に観測値になる。
// o は key の設定の領域を読み取った観測の結果に限り、読み取らなかった領域の値は補わずに nil 参照で止まる。
// 未知の設定は不変条件の破れとして panic する。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (o ObservationResult) Setting(key setting.Key) SettingObservation[setting.Value] {
	switch key {
	case setting.DeleteBranchOnMerge:
		return Observed(setting.BoolValue(o.General.DeleteBranchOnMerge))
	case setting.AllowAutoMerge:
		return Observed(setting.BoolValue(o.General.AllowAutoMerge))
	case setting.ActionsEnabled:
		return Observed(setting.BoolValue(o.ActionsPermissions.Enabled))
	case setting.AllowedActions:
		return valueObservation(o.ActionsPermissions.AllowedActions, setting.AllowedActionsPolicyValue)
	case setting.SHAPinningRequired:
		return valueObservation(o.ActionsPermissions.SHAPinningRequired, setting.BoolValue)
	case setting.SelectedActions:
		return valueObservation(o.ActionsPermissions.SelectedActions, setting.SelectedActionsCriteriaValue)
	case setting.DefaultWorkflowPermissions:
		return Observed(setting.PermissionLevelValue(o.WorkflowPermissions.DefaultWorkflowPermissions))
	case setting.CanApprovePullRequestReviews:
		return Observed(setting.BoolValue(o.WorkflowPermissions.CanApprovePullRequestReviews))
	case setting.ActionsAccessLevel:
		return valueObservation(o.ActionsAccess.AccessLevel, setting.AccessLevelValue)
	case setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy:
		// Environment の管理の単位は名前との組で識別し、観測は Environments.Lookup で引くため、この関数では引けない。
	}
	panic(fmt.Sprintf("unknown setting %q", key))
}

// ObserveRepository は reference のリポジトリを観測する。areas で、General に加えて読み取る設定の領域を選ぶ。
// General は常に読み取るため、areas に AreaGeneral を含めても結果は変わらない。
// 読み取り要求は areas の並びによらず、設計で定めた固定の順に送る。
func (c *Client) ObserveRepository(ctx context.Context, reference repository.Reference, areas ...setting.Area) (ObservationResult, error) {
	o, private, err := c.observeRepository(ctx, reference)
	if err != nil {
		return ObservationResult{}, fmt.Errorf("fetch repository %s: %w", reference, err)
	}
	if slices.Contains(areas, setting.AreaActionsPermissions) {
		ap, err := c.observeActionsPermissions(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch actions permissions %s: %w", reference, err)
		}
		ap.SelectedActions, err = c.observeSelectedActions(ctx, reference, ap.AllowedActions)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch selected actions %s: %w", reference, err)
		}
		o.ActionsPermissions = &ap
	}
	if slices.Contains(areas, setting.AreaWorkflowPermissions) {
		wp, err := c.observeWorkflowPermissions(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch workflow permissions %s: %w", reference, err)
		}
		o.WorkflowPermissions = &wp
	}
	if slices.Contains(areas, setting.AreaActionsAccess) {
		aa, err := c.observeActionsAccess(ctx, reference, private)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch actions access %s: %w", reference, err)
		}
		o.ActionsAccess = &aa
	}
	if slices.Contains(areas, setting.AreaEnvironments) {
		environments, err := c.observeEnvironments(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch environments %s: %w", reference, err)
		}
		o.Environments = &environments
	}
	return o, nil
}

// observeRepository はリポジトリを読み取り、観測結果と公開範囲を返す。
// 公開範囲は取得条件の判定にだけ使い、観測結果には載せない。
func (c *Client) observeRepository(ctx context.Context, reference repository.Reference) (ObservationResult, bool, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.repositoryURL(reference), nil, http.StatusOK)
	if err != nil {
		return ObservationResult{}, false, err
	}
	return decodeRepository(body)
}

func (c *Client) observeWorkflowPermissions(ctx context.Context, reference repository.Reference) (WorkflowPermissions, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.workflowPermissionsURL(reference), nil, http.StatusOK)
	if err != nil {
		return WorkflowPermissions{}, err
	}
	return decodeWorkflowPermissions(body)
}

// observeActionsPermissions は Actions permissions を読み取る。選択方式の Actions 許可条件は別の要求のため決めない。
func (c *Client) observeActionsPermissions(ctx context.Context, reference repository.Reference) (ActionsPermissions, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.actionsPermissionsURL(reference), nil, http.StatusOK)
	if err != nil {
		return ActionsPermissions{}, err
	}
	return decodeActionsPermissions(body)
}

// observeSelectedActions は、観測した Actions 許可ポリシーが取得条件に当たれば要求を送らずに観測不能とし、当たらなければ Selected actions を読み取る。
// selected と観測したあとの失敗は、二つの要求のあいだにポリシーが変わった可能性があっても観測不能にせず、error として返す。
func (c *Client) observeSelectedActions(ctx context.Context, reference repository.Reference, allowed SettingObservation[setting.AllowedActionsPolicy]) (SettingObservation[setting.SelectedActionsCriteria], error) {
	if reason, unavailable := selectedActionsUnavailability(allowed); unavailable {
		return Unavailable[setting.SelectedActionsCriteria](reason), nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.selectedActionsURL(reference), nil, http.StatusOK)
	if err != nil {
		return SettingObservation[setting.SelectedActionsCriteria]{}, err
	}
	criteria, err := decodeSelectedActions(body)
	if err != nil {
		return SettingObservation[setting.SelectedActionsCriteria]{}, err
	}
	return Observed(criteria), nil
}

// observeActionsAccess は、公開範囲が取得条件に当たれば要求を送らずに観測不能とし、当たらなければ access_level を読み取る。
// 非公開と観測したあとの失敗は、二つの要求のあいだに公開範囲が変わった可能性があっても、422 を含めて観測不能にせず error として返す。
func (c *Client) observeActionsAccess(ctx context.Context, reference repository.Reference, private bool) (ActionsAccess, error) {
	if reason, unavailable := accessLevelUnavailability(private); unavailable {
		return ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](reason)}, nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.actionsAccessURL(reference), nil, http.StatusOK)
	if err != nil {
		return ActionsAccess{}, err
	}
	level, err := decodeActionsAccess(body)
	if err != nil {
		return ActionsAccess{}, err
	}
	return ActionsAccess{AccessLevel: Observed(level)}, nil
}

// valueObservation は、設定ごとの型の観測結果を、設定の値の観測結果に写す。観測不能は同じ理由のまま写す。
func valueObservation[T any](o SettingObservation[T], toValue func(T) setting.Value) SettingObservation[setting.Value] {
	if v, ok := o.Value(); ok {
		return Observed(toValue(v))
	}
	return Unavailable[setting.Value](o.UnavailableReason())
}

// workflowPermissionsPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type workflowPermissionsPayload struct {
	DefaultWorkflowPermissions   *string `json:"default_workflow_permissions"`
	CanApprovePullRequestReviews *bool   `json:"can_approve_pull_request_reviews"`
}

// decodeWorkflowPermissions は Workflow permissions の応答本文を観測値に変換する。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeWorkflowPermissions(data []byte) (WorkflowPermissions, error) {
	var p workflowPermissionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return WorkflowPermissions{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.DefaultWorkflowPermissions == nil {
		missing = append(missing, "default_workflow_permissions")
	}
	if p.CanApprovePullRequestReviews == nil {
		missing = append(missing, "can_approve_pull_request_reviews")
	}
	if len(missing) > 0 {
		return WorkflowPermissions{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	level, ok := setting.ParsePermissionLevel(*p.DefaultWorkflowPermissions)
	if !ok {
		return WorkflowPermissions{}, fmt.Errorf("%w: unsupported value: default_workflow_permissions", ErrUnexpectedResponse)
	}
	return WorkflowPermissions{
		DefaultWorkflowPermissions:   level,
		CanApprovePullRequestReviews: *p.CanApprovePullRequestReviews,
	}, nil
}

// actionsPermissionsPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type actionsPermissionsPayload struct {
	Enabled            *bool   `json:"enabled"`
	AllowedActions     *string `json:"allowed_actions"`
	SHAPinningRequired *bool   `json:"sha_pinning_required"`
}

// decodeActionsPermissions は Actions permissions の応答本文を、Actions の有効状態、Actions 許可ポリシー、sha_pinning_required の観測結果に変換する。
// Actions が無効のときは、応答に値があっても設定の状態を示さないため読まない。有効のときの欠落は既定値で補わない。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeActionsPermissions(data []byte) (ActionsPermissions, error) {
	var p actionsPermissionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return ActionsPermissions{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p.Enabled == nil {
		return ActionsPermissions{}, fmt.Errorf("%w: missing fields: enabled", ErrUnexpectedResponse)
	}
	if !*p.Enabled {
		return ActionsPermissions{
			AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
			SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
		}, nil
	}
	var missing []string
	if p.AllowedActions == nil {
		missing = append(missing, "allowed_actions")
	}
	if p.SHAPinningRequired == nil {
		missing = append(missing, "sha_pinning_required")
	}
	if len(missing) > 0 {
		return ActionsPermissions{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	policy, ok := setting.ParseAllowedActionsPolicy(*p.AllowedActions)
	if !ok {
		return ActionsPermissions{}, fmt.Errorf("%w: unsupported value: allowed_actions", ErrUnexpectedResponse)
	}
	return ActionsPermissions{
		Enabled:            true,
		AllowedActions:     Observed(policy),
		SHAPinningRequired: Observed(*p.SHAPinningRequired),
	}, nil
}

// selectedActionsUnavailability は、観測した Actions 許可ポリシーから、Selected actions が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func selectedActionsUnavailability(allowed SettingObservation[setting.AllowedActionsPolicy]) (reason UnavailableReason, unavailable bool) {
	policy, ok := allowed.Value()
	if !ok {
		return allowed.UnavailableReason(), true
	}
	if policy != setting.AllowedActionsSelected {
		return UnavailableNotSelected, true
	}
	return 0, false
}

// selectedActionsPayload は欠落と null を検出するため、各フィールドと許可パターンの要素をポインタで受ける。
// encoding/json は null の要素を string へ読むと空文字列のまま残し、欠けた値を空文字列と区別できなくなるため。
type selectedActionsPayload struct {
	GitHubOwnedAllowed *bool      `json:"github_owned_allowed"`
	VerifiedAllowed    *bool      `json:"verified_allowed"`
	PatternsAllowed    *[]*string `json:"patterns_allowed"`
}

// decodeSelectedActions は Selected actions の応答本文を選択方式の Actions 許可条件に変換する。
// 3項目は一つでも欠ければ、既定値で補わず契約外の応答とする。許可パターンは応答本文の一部のため、文言に入れない。
func decodeSelectedActions(data []byte) (setting.SelectedActionsCriteria, error) {
	var p selectedActionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.GitHubOwnedAllowed == nil {
		missing = append(missing, "github_owned_allowed")
	}
	if p.VerifiedAllowed == nil {
		missing = append(missing, "verified_allowed")
	}
	if p.PatternsAllowed == nil {
		missing = append(missing, "patterns_allowed")
	}
	if len(missing) > 0 {
		return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	patterns := make([]string, len(*p.PatternsAllowed))
	for i, pattern := range *p.PatternsAllowed {
		if pattern == nil {
			return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: unsupported value: patterns_allowed", ErrUnexpectedResponse)
		}
		patterns[i] = *pattern
	}
	return setting.SelectedActionsCriteria{
		GitHubOwnedAllowed: *p.GitHubOwnedAllowed,
		VerifiedAllowed:    *p.VerifiedAllowed,
		PatternsAllowed:    patterns,
	}, nil
}

// accessLevelUnavailability は、観測した公開範囲から、access_level が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func accessLevelUnavailability(private bool) (reason UnavailableReason, unavailable bool) {
	if !private {
		return UnavailablePublicRepository, true
	}
	return 0, false
}

// actionsAccessPayload は欠落と null を検出するため、フィールドをポインタで受ける。
type actionsAccessPayload struct {
	AccessLevel *string `json:"access_level"`
}

// decodeActionsAccess はリポジトリ外のワークフローからのアクセスの応答本文を access_level の観測値に変換する。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeActionsAccess(data []byte) (setting.AccessLevel, error) {
	var p actionsAccessPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p.AccessLevel == nil {
		return "", fmt.Errorf("%w: missing fields: access_level", ErrUnexpectedResponse)
	}
	level, ok := setting.ParseAccessLevel(*p.AccessLevel)
	if !ok {
		return "", fmt.Errorf("%w: unsupported value: access_level", ErrUnexpectedResponse)
	}
	return level, nil
}
