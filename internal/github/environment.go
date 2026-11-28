package github

import (
	"context"
	"fmt"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Environments は、Environment の一覧の全ページを読めた観測で確認できた、存在するすべての Environment である。
type Environments struct {
	// Existing は GitHub が返した順のまま持つ。
	Existing []Environment
}

// Environment は、存在すると確かめた一つの Environment の観測結果である。
type Environment struct {
	ID EnvironmentID
	// Name は GitHub が返した綴りのまま持つ。
	Name string
	// DeploymentBranchPolicyMode は一覧の要素から読むため、読む範囲によらず持つ。個別パターンを含む値は DeploymentBranchPolicy で読む。
	DeploymentBranchPolicyMode setting.DeploymentBranchPolicyMode
	// DeploymentPatterns は、方式が selected で、個別パターンを読んだ Environment でだけ持つ。
	// 方式が selected で nil なのは読む範囲に含めなかった Environment で、個別パターンが一つもないことや、個別パターンの観測不能とは異なる。
	DeploymentPatterns *SettingObservation[DeploymentPatterns]
	Protection         EnvironmentProtection
	// UninterpretedProtectionRules は、reco が種類を知らない保護ルールの type を、応答の順に、同じものは最初の一つだけ持つ。
	// 保護設定の値に入れないのは、存在の宣言をした Environment の照合と更新の前後の比較を変えないためで、削除で失われる値の全体を示せるかの判定にだけ使う。
	UninterpretedProtectionRules []UninterpretedPart
	// Variables は、Environment Variables の領域を選ばなかった観測と、読む範囲に含めなかった Environment では nil になる。
	// Variable が一つもないことと取り違えさせないため、値型にしない。
	Variables *Variables
	// Secrets は、削除で失われるものまで読む範囲に含めた Environment でだけ持つ。ほかでは nil で、Secret が一つもないこととは異なる。
	Secrets *EnvironmentSecrets
}

// EnvironmentProtection は、Environment の保護設定の観測結果。reco は観測し、can_admins_bypass のほかは宣言を受け付ける。
// 担当者の集合だけを観測結果で持つ。同じ保護ルールのほかの項目と別に、観測不能になり得るため。
type EnvironmentProtection struct {
	// WaitTimer はデプロイを待たせる分数。wait_timer の保護ルールがなければ 0。
	WaitTimer int
	// ReviewerSet は、required_reviewers の保護ルールがなければ空集合の観測値。
	ReviewerSet SettingObservation[setting.ReviewerSet]
	// PreventSelfReview は、required_reviewers の保護ルールがなければ偽。
	PreventSelfReview bool
	CanAdminsBypass   bool
}

// DeploymentPatterns は、方式が selected の一つの Environment について、個別パターンの一覧の全ページを読めた観測で確認できた、すべての個別パターンである。
type DeploymentPatterns struct {
	// Existing は GitHub が返した順のまま持つ。
	Existing []setting.DeploymentPattern
	// IDs は個別パターンごとの GitHub の ID。
	IDs map[setting.DeploymentPattern]DeploymentPatternID
}

// EnvironmentID は、GitHub が Environment の実体に付ける ID。同じ名前で作り直すと変わる。
type EnvironmentID int64

// DeploymentPatternID は、GitHub が個別パターンに付ける ID。値の一部ではなく、個別パターンを取り除く要求の宛先に使う。
type DeploymentPatternID int64

// DeploymentBranchPolicy は、デプロイ元制限の観測結果を返す。方式が selected なら、読んだ個別パターンを読んだ順で含む。
// 方式が selected で個別パターンが観測不能なら、方式と個別パターンの組を同じ理由の観測不能にする。方式だけの値は、個別パターンのない宣言と同じ値に読めるため。
func (e Environment) DeploymentBranchPolicy() SettingObservation[setting.DeploymentBranchPolicy] {
	if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected {
		return Observed(setting.DeploymentBranchPolicy{Mode: e.DeploymentBranchPolicyMode})
	}
	observation := e.readDeploymentPatterns()
	patterns, ok := observation.Value()
	if !ok {
		return Unavailable[setting.DeploymentBranchPolicy](observation.UnavailableReason())
	}
	return Observed(setting.DeploymentBranchPolicy{Mode: e.DeploymentBranchPolicyMode, Patterns: patterns.Existing})
}

// DeletedValue は、削除で失われる値の観測結果を返す。担当者は観測した login を持つ。
// 知らない種類の保護ルールがあれば、削除がその保護ルールも消すことを値で示せないため、解釈しない部分を理由にした観測不能にする。
// そうでなく、デプロイ元制限か担当者の集合が観測不能なら、同じ理由（知らない値）の観測不能にする。観測不能を方式だけの値や空集合で埋めると、失われる値を少なく示すため。
// 方式が selected で個別パターンを読んでいなければ、DeploymentBranchPolicy と同じく panic する。
func (e Environment) DeletedValue() SettingObservation[setting.EnvironmentDeletedValue] {
	if len(e.UninterpretedProtectionRules) > 0 {
		return Unavailable[setting.EnvironmentDeletedValue](UnavailableUninterpretedParts)
	}
	policy := e.DeploymentBranchPolicy()
	deploymentBranchPolicy, ok := policy.Value()
	if !ok {
		return Unavailable[setting.EnvironmentDeletedValue](policy.UnavailableReason())
	}
	reviewerSet, ok := e.Protection.ReviewerSet.Value()
	if !ok {
		return Unavailable[setting.EnvironmentDeletedValue](e.Protection.ReviewerSet.UnavailableReason())
	}
	return Observed(setting.EnvironmentDeletedValue{
		DeploymentBranchPolicy: deploymentBranchPolicy,
		ReviewerSet:            reviewerSet,
		PreventSelfReview:      e.Protection.PreventSelfReview,
		WaitTimer:              e.Protection.WaitTimer,
		CanAdminsBypass:        e.Protection.CanAdminsBypass,
	})
}

// DeploymentPatternID は、読んだ個別パターン pattern の ID を返す。方式が selected でないか、個別パターンが観測不能か、pattern を読んでいなければ偽を返す。
func (e Environment) DeploymentPatternID(pattern setting.DeploymentPattern) (DeploymentPatternID, bool) {
	if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected {
		return 0, false
	}
	// 観測不能なら Value はゼロ値を返し、ID を持たないため偽になる。取り除く更新操作は、変更前提が先に不成立になり到達しない
	patterns, _ := e.readDeploymentPatterns().Value()
	id, ok := patterns.IDs[pattern]
	return id, ok
}

// readDeploymentPatterns は、方式が selected の Environment の個別パターンの観測結果を返す。
// 読む範囲に含めなかった Environment は、個別パターンが一つもないことや観測不能と取り違えないよう、不変条件の破れとして panic する（ドメインモデルの存在の観測）。
func (e Environment) readDeploymentPatterns() SettingObservation[DeploymentPatterns] {
	if e.DeploymentPatterns == nil {
		panic(fmt.Sprintf("github: deployment patterns of environment %q were not read in this observation", e.Name))
	}
	return *e.DeploymentPatterns
}

// Lookup は、同一性を判定できる名前 name の Environment を、一覧の全体と突き合わせて引く。
// 呼び出し側が Existing を綴りで探すと、存在する Environment を存在しないと判断し得るため、ここでだけ引く。
// 同じ名前の要素が二つ以上ないことは、observeEnvironments が取得契約として確かめている。
func (e Environments) Lookup(name setting.EnvironmentName) EnvironmentObservation {
	undetermined := false
	for _, env := range e.Existing {
		switch name.Compare(env.Name) {
		case setting.NameSame:
			return EnvironmentObservation{state: resourceExisting, environment: env}
		case setting.NameUndetermined:
			undetermined = true
		case setting.NameDifferent:
		}
	}
	if undetermined {
		return EnvironmentObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined}
	}
	return EnvironmentObservation{state: resourceAbsent}
}

// observeEnvironments は Environment の一覧の全ページを読み、scope にある方式が selected の Environment にだけ、一覧の並びで個別パターンを読む。
// 個別パターンの要求を送る前にすべての要素を変換し、契約外の要素があれば要求を送らずに失敗する。
func (c *Client) observeEnvironments(ctx context.Context, reference repository.Reference, scope resourceScope) (Environments, error) {
	items, err := c.listAll(ctx, c.environmentsURL(reference), listPaging{itemsField: "environments", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return Environments{}, err
	}
	existing := make([]Environment, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		existing[i], err = decodeEnvironment(item)
		if err != nil {
			return Environments{}, err
		}
		// GitHub は大文字小文字だけが違う名前を同じ Environment とみなすため、二つ並ぶと照合と更新の相手が決まらない。
		// strings.EqualFold が同じとみなす名前は GitHub も同じとみなす（GitHub の同一判定はより広い）ため、別の Environment を重複と誤らない。
		key := simpleFoldKey(existing[i].Name)
		if _, duplicate := seen[key]; duplicate {
			return Environments{}, fmt.Errorf("environments: %w: duplicate environment name %q", ErrUnexpectedResponse, existing[i].Name)
		}
		seen[key] = struct{}{}
	}
	for i := range existing {
		e := &existing[i]
		if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected || !scope.includesEnvironment(e.Name) {
			continue
		}
		patterns, err := c.observeDeploymentPatterns(ctx, reference, e.Name)
		if err != nil {
			return Environments{}, err
		}
		e.DeploymentPatterns = &patterns
	}
	return Environments{Existing: existing}, nil
}

// observeEnvironmentDeletions は、environments の並びで、削除で失われるものまで読む scope の範囲にある Environment ごとに、
// 変数をまだ読んでいなければ変数の一覧を、続けて Secret の一覧を読み、その Environment に入れる。
// environments は同じ観測で一覧から読んだ Environment に限る。範囲にあっても一覧にない名前には要求を送らない。
func (c *Client) observeEnvironmentDeletions(ctx context.Context, reference repository.Reference, environments []Environment, scope resourceScope) error {
	for i := range environments {
		e := &environments[i]
		if !scope.includesEnvironmentDeletion(e.Name) {
			continue
		}
		if e.Variables == nil {
			variables, err := c.observeVariablesOfEnvironment(ctx, reference, e.Name)
			if err != nil {
				return err
			}
			e.Variables = &variables
		}
		secrets, err := c.observeEnvironmentSecrets(ctx, reference, e.Name)
		if err != nil {
			return err
		}
		e.Secrets = &secrets
	}
	return nil
}

// observeDeploymentPatterns は、方式が selected の Environment の個別パターンを一覧の並びのまま読む。
// selected と観測したあとの失敗は、二つの要求のあいだに方式が変わった可能性があっても、404 を含めて観測不能にせず error として返す。
// パスに組み込めない名前の Environment は、要求を送らずに失敗する。
// 失敗の文言に Environment の名前を入れるのは、どの Environment で失敗したかを示す対象の識別子で、秘密情報ではないため。%q で制御文字をエスケープする。
func (c *Client) observeDeploymentPatterns(ctx context.Context, reference repository.Reference, name string) (SettingObservation[DeploymentPatterns], error) {
	u, ok := c.deploymentBranchPoliciesURL(reference, name)
	if !ok {
		return SettingObservation[DeploymentPatterns]{}, fmt.Errorf("deployment branch policies of environment %q: %w: the name cannot be sent as a path element", name, ErrUnexpectedResponse)
	}
	items, err := c.listAll(ctx, u, listPaging{itemsField: "branch_policies", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return SettingObservation[DeploymentPatterns]{}, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
	}
	patterns, err := decodeDeploymentPatterns(items)
	if err != nil {
		return SettingObservation[DeploymentPatterns]{}, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
	}
	return patterns, nil
}
