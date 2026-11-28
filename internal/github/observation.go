package github

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// ObservationResult は、一回の観測で確認できた、リポジトリ指定が指す実体の ID と、読み取った設定の領域の設定ごとの、設定の観測結果である。
// 設定できる値の範囲を GitHub が設定と同じ応答で返す領域については、その範囲も含む。
// 関連リソースを読み取った設定の領域については、関連リソースの観測結果も含む。
type ObservationResult struct {
	RepositoryID repository.ID
	General      General
	// ActionsPermissions、WorkflowPermissions、ActionsAccess は、その設定の領域を選ばなかった観測では nil になる。
	// 読み取っていない値を false や read として読ませず、観測不能とも取り違えさせないため、値型にしない。
	ActionsPermissions  *ActionsPermissions
	WorkflowPermissions *WorkflowPermissions
	ActionsAccess       *ActionsAccess
	// ForkPullRequestApproval、PrivateForkPullRequestWorkflows、Retention は、その設定の領域を選ばなかった観測では nil になる。
	// 読み取っていない値を、公開範囲による観測不能や、0 日の保持期間として読ませないため、値型にしない。
	ForkPullRequestApproval         *ForkPullRequestApproval
	PrivateForkPullRequestWorkflows *PrivateForkPullRequestWorkflows
	Retention                       *Retention
	// OIDCSubject は、その設定の領域を選ばなかった観測では nil になる。読み取っていない値を、既定の形を使う状態として読ませないため、値型にしない。
	OIDCSubject *OIDCSubject
	// RepositoryVariables は、その設定の領域を選ばなかった観測では nil になる。Variable が一つもないことと取り違えさせないため、値型にしない。
	RepositoryVariables *Variables
	// Environments は、Environments と Environment Variables のどちらの設定の領域も選ばなかった観測では nil になる。Environment が一つもないことと取り違えさせないため、値型にしない。
	Environments *Environments
	// Dependabot は、その設定の領域を選ばなかった観測では nil になる。
	Dependabot *Dependabot
	// SecretScanning は、その設定の領域を選ばなかった観測では nil になる。非公開リポジトリの観測不能と取り違えさせないため、値型にしない。
	SecretScanning *SecretScanning
	// PrivateVulnerabilityReporting は、その設定の領域を選ばなかった観測では nil になる。非公開リポジトリの観測不能と取り違えさせないため、値型にしない。
	PrivateVulnerabilityReporting *PrivateVulnerabilityReporting
	// Rulesets は、その設定の領域を選ばなかった観測では nil になる。Ruleset が一つもないことと、非公開リポジトリの観測不能と取り違えさせないため、値型にしない。
	Rulesets *Rulesets
}

// General はリポジトリ一般設定のうち、reco が観測する値である。管理範囲を知らないため、未管理設定の値も含む。
type General struct {
	AllowAutoMerge           bool
	DeleteBranchOnMerge      bool
	AllowSquashMerge         bool
	AllowMergeCommit         bool
	AllowRebaseMerge         bool
	AllowUpdateBranch        bool
	SquashMergeCommitDefault SettingObservation[setting.SquashMergeCommitDefault]
	MergeCommitDefault       SettingObservation[setting.MergeCommitDefault]
	HasIssues                bool
	HasProjects              bool
	HasWiki                  bool
	WebCommitSignoffRequired SettingObservation[bool]
}

// NamedResources は、観測を使う側が名指した関連リソースの名前を種類ごとに持つ。関連リソースごとの読み取りを送る範囲を決める。
type NamedResources struct {
	Environments []setting.EnvironmentName
	Rulesets     []setting.RulesetName
}

// Setting は、管理の単位 unit の設定の観測結果を返す。取得条件を持たない設定は、常に観測値になる。
// o は unit の設定の領域を読み取った観測の結果に限り、読み取らなかった領域の値は補わずに nil 参照で止まる。
// unit が Environment の中の設定か変数なら、その Environment を読む範囲に含めた観測に限り、含めなかった Environment の値は補わずに panic で止まる。
// unit が Ruleset なら、その名前を読む範囲に含めた観測に限り、値を読まなかった Ruleset の値は補わずに panic で止まる。
// 未知の設定は不変条件の破れとして panic する。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (o ObservationResult) Setting(unit setting.Unit) SettingObservation[setting.Value] {
	switch unit.Key {
	case setting.DeleteBranchOnMerge:
		return Observed(setting.BoolValue(o.General.DeleteBranchOnMerge))
	case setting.AllowAutoMerge:
		return Observed(setting.BoolValue(o.General.AllowAutoMerge))
	case setting.AllowSquashMerge:
		return Observed(setting.BoolValue(o.General.AllowSquashMerge))
	case setting.AllowMergeCommit:
		return Observed(setting.BoolValue(o.General.AllowMergeCommit))
	case setting.AllowRebaseMerge:
		return Observed(setting.BoolValue(o.General.AllowRebaseMerge))
	case setting.AllowUpdateBranch:
		return Observed(setting.BoolValue(o.General.AllowUpdateBranch))
	case setting.SquashMergeCommit:
		return valueObservation(o.General.SquashMergeCommitDefault, setting.SquashMergeCommitDefaultValue)
	case setting.MergeCommit:
		return valueObservation(o.General.MergeCommitDefault, setting.MergeCommitDefaultValue)
	case setting.HasIssues:
		return Observed(setting.BoolValue(o.General.HasIssues))
	case setting.HasProjects:
		return Observed(setting.BoolValue(o.General.HasProjects))
	case setting.HasWiki:
		return Observed(setting.BoolValue(o.General.HasWiki))
	case setting.WebCommitSignoffRequired:
		return valueObservation(o.General.WebCommitSignoffRequired, setting.BoolValue)
	case setting.ActionsEnabled:
		return Observed(setting.BoolValue(o.ActionsPermissions.Enabled))
	case setting.AllowedActions:
		return valueObservation(o.ActionsPermissions.AllowedActions, setting.AllowedActionsPolicyValue)
	case setting.SHAPinningRequired:
		return valueObservation(o.ActionsPermissions.SHAPinningRequired, setting.BoolValue)
	case setting.SelectedActions:
		return valueObservation(o.ActionsPermissions.SelectedActions, setting.SelectedActionsCriteriaValue)
	case setting.DefaultWorkflowPermissions:
		return valueObservation(o.WorkflowPermissions.DefaultWorkflowPermissions, setting.PermissionLevelValue)
	case setting.CanApprovePullRequestReviews:
		return Observed(setting.BoolValue(o.WorkflowPermissions.CanApprovePullRequestReviews))
	case setting.ActionsAccessLevel:
		return valueObservation(o.ActionsAccess.AccessLevel, setting.AccessLevelValue)
	case setting.ForkPullRequestApproval:
		return valueObservation(o.ForkPullRequestApproval.ApprovalPolicy, setting.ForkPullRequestApprovalPolicyValue)
	case setting.PrivateForkPullRequestWorkflows:
		return valueObservation(o.PrivateForkPullRequestWorkflows.Conditions, setting.PrivateForkPullRequestWorkflowConditionsValue)
	case setting.RetentionDays:
		return Observed(setting.RetentionDaysValue(o.Retention.Days))
	case setting.OIDCSubject:
		customization := o.OIDCSubject.Customization
		if customization.UseImmutableSubject == nil {
			// 観測値がこの項目を持たないと、同じ値の判定が項目を比べずに一致とする
			panic("github: observed oidc subject has no use_immutable_subject")
		}
		return Observed(setting.OIDCSubjectCustomizationValue(customization))
	case setting.DependabotAlerts:
		return Observed(setting.BoolValue(o.Dependabot.Alerts))
	case setting.DependabotSecurityUpdates:
		return Observed(setting.DependabotSecurityUpdatesValue(o.Dependabot.SecurityUpdates))
	case setting.SecretScanningEnabled:
		return valueObservation(o.SecretScanning.Enabled, setting.BoolValue)
	case setting.RepositoryPushProtection:
		return valueObservation(o.SecretScanning.PushProtection, setting.BoolValue)
	case setting.PrivateVulnerabilityReportingEnabled:
		return valueObservation(o.PrivateVulnerabilityReporting.Enabled, setting.BoolValue)
	case setting.RepositoryVariable:
		return o.RepositoryVariables.Lookup(unit.Variable)
	case setting.EnvironmentDeploymentBranchPolicy:
		return o.Environments.Lookup(unit.Environment).DeploymentBranchPolicy()
	case setting.EnvironmentRequiredReviewers:
		return o.Environments.Lookup(unit.Environment).RequiredReviewers()
	case setting.EnvironmentPreventSelfReview:
		return o.Environments.Lookup(unit.Environment).PreventSelfReview()
	case setting.EnvironmentWaitTimer:
		return o.Environments.Lookup(unit.Environment).WaitTimer()
	case setting.EnvironmentVariable:
		return o.Environments.Lookup(unit.Environment).Variable(unit.Variable)
	case setting.EnvironmentResource:
		return o.Environments.Lookup(unit.Environment).Existence()
	case setting.RulesetResource:
		return o.Rulesets.Lookup(unit.Ruleset).SettingObservation()
	}
	panic(fmt.Sprintf("unknown setting %q", unit.Key))
}

// ObserveRepository は reference のリポジトリを観測する。areas で、General に加えて読み取る設定の領域を選ぶ。
// General は常に読み取るため、areas に AreaGeneral を含めても結果は変わらない。
// 読み取り要求は areas の並びによらず、設計で定めた固定の順に送る。
// Environment ごとの読み取りと Ruleset ごとの詳細の読み取りは、一覧のすべての Environment と Ruleset に送る。
func (c *Client) ObserveRepository(ctx context.Context, reference repository.Reference, areas ...setting.Area) (ObservationResult, error) {
	return c.observe(ctx, reference, allResources(), areas)
}

// ObserveRepositoryForNamedResources は、ObserveRepository と同じく観測し、関連リソースごとの読み取りを、種類ごとに named が名指したどれかと同じ名前の関連リソースに限る。
// 範囲の外の Environment は、存在と一覧の要素の値だけを持ち、個別パターンと変数を読んでいない状態になる。
// 範囲の外の Ruleset は、存在と ID と名前だけを持ち、値を読んでいない状態になる。
// どの種類の一覧も、範囲によらず、その設定の領域を選べば読む。
func (c *Client) ObserveRepositoryForNamedResources(ctx context.Context, reference repository.Reference, named NamedResources, areas ...setting.Area) (ObservationResult, error) {
	return c.observe(ctx, reference, scopeOfNamed(named), areas)
}

func (c *Client) observe(ctx context.Context, reference repository.Reference, scope resourceScope, areas []setting.Area) (ObservationResult, error) {
	o, attributes, err := c.observeRepository(ctx, reference)
	if err != nil {
		return ObservationResult{}, fmt.Errorf("fetch repository %s: %w", reference, err)
	}
	// Secret scanning はリポジトリの応答から読むため、ほかの要求より先に判定し、失敗は同じ取得の失敗として包む
	if slices.Contains(areas, setting.AreaSecretScanning) {
		ss, err := decodeSecretScanning(attributes.private, attributes.securityAndAnalysis)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch repository %s: %w", reference, err)
		}
		o.SecretScanning = &ss
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
		aa, err := c.observeActionsAccess(ctx, reference, attributes.private)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch actions access %s: %w", reference, err)
		}
		o.ActionsAccess = &aa
	}
	if slices.Contains(areas, setting.AreaForkPullRequestApproval) {
		approval, err := c.observeForkPullRequestApproval(ctx, reference, attributes.private)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch fork pull request approval %s: %w", reference, err)
		}
		o.ForkPullRequestApproval = &approval
	}
	if slices.Contains(areas, setting.AreaPrivateForkPullRequestWorkflows) {
		workflows, err := c.observePrivateForkPullRequestWorkflows(ctx, reference, attributes.private)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch private fork pull request workflows %s: %w", reference, err)
		}
		o.PrivateForkPullRequestWorkflows = &workflows
	}
	if slices.Contains(areas, setting.AreaRetention) {
		retention, err := c.observeRetention(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch retention %s: %w", reference, err)
		}
		o.Retention = &retention
	}
	if slices.Contains(areas, setting.AreaOIDCSubject) {
		oidcSubject, err := c.observeOIDCSubject(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch oidc subject %s: %w", reference, err)
		}
		o.OIDCSubject = &oidcSubject
	}
	if slices.Contains(areas, setting.AreaRepositoryVariables) {
		variables, err := c.observeRepositoryVariables(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch repository variables %s: %w", reference, err)
		}
		o.RepositoryVariables = &variables
	}
	// Environment Variables は属する先の Environment を同じ観測で確かめてから読むため、Environments の領域も読む
	if slices.Contains(areas, setting.AreaEnvironments) || slices.Contains(areas, setting.AreaEnvironmentVariables) {
		environments, err := c.observeEnvironments(ctx, reference, scope)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch environments %s: %w", reference, err)
		}
		o.Environments = &environments
	}
	if slices.Contains(areas, setting.AreaEnvironmentVariables) {
		if err := c.observeEnvironmentVariables(ctx, reference, o.Environments.Existing, scope); err != nil {
			return ObservationResult{}, fmt.Errorf("fetch environment variables %s: %w", reference, err)
		}
	}
	// Dependabot の二つの設定は、片方だけの観測結果を作らない
	if slices.Contains(areas, setting.AreaDependabot) {
		alerts, err := c.observeDependabotAlerts(ctx, reference, attributes.admin)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch dependabot alerts %s: %w", reference, err)
		}
		updates, err := c.observeDependabotSecurityUpdates(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch dependabot security updates %s: %w", reference, err)
		}
		o.Dependabot = &Dependabot{Alerts: alerts, SecurityUpdates: updates}
	}
	if slices.Contains(areas, setting.AreaPrivateVulnerabilityReporting) {
		privateVulnerabilityReporting, err := c.observePrivateVulnerabilityReporting(ctx, reference, attributes.private)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch private vulnerability reporting %s: %w", reference, err)
		}
		o.PrivateVulnerabilityReporting = &privateVulnerabilityReporting
	}
	if slices.Contains(areas, setting.AreaRulesets) {
		rulesets, err := c.observeRulesets(ctx, reference, attributes.private, scope)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch rulesets %s: %w", reference, err)
		}
		o.Rulesets = &rulesets
	}
	return o, nil
}

// observeRepository はリポジトリを読み取り、観測結果と、観測結果に変換しない値を返す。
func (c *Client) observeRepository(ctx context.Context, reference repository.Reference) (ObservationResult, repositoryAttributes, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.repositoryURL(reference), nil, http.StatusOK)
	if err != nil {
		return ObservationResult{}, repositoryAttributes{}, err
	}
	return decodeRepository(body)
}

// valueObservation は、設定ごとの型の観測結果を、設定の値の観測結果に写す。観測不能は同じ理由のまま写す。
func valueObservation[T any](o SettingObservation[T], toValue func(T) setting.Value) SettingObservation[setting.Value] {
	if v, ok := o.Value(); ok {
		return Observed(toValue(v))
	}
	return Unavailable[setting.Value](o.UnavailableReason())
}
