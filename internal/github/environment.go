package github

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	Name                   string
	DeploymentBranchPolicy setting.DeploymentBranchPolicy
	// DeploymentPatternIDs は個別パターンごとの GitHub の ID。方式が selected でなければ nil。
	DeploymentPatternIDs map[setting.DeploymentPattern]DeploymentPatternID
	Protection           setting.EnvironmentProtection
}

// EnvironmentID は、GitHub が Environment の実体に付ける ID。同じ名前で作り直すと変わる。
type EnvironmentID int64

// DeploymentPatternID は、GitHub が個別パターンに付ける ID。値の一部ではなく、個別パターンを取り除く要求の宛先に使う。
type DeploymentPatternID int64

// Lookup は、宣言の名前 name の Environment を、一覧の全体と突き合わせて引く。
// 呼び出し側が Existing を綴りで探すと、存在する Environment を存在しないと判断し得るため、ここでだけ引く。
// 同じ名前の要素が二つ以上ないことは、observeEnvironments が取得契約として確かめている。
func (e Environments) Lookup(name setting.EnvironmentName) EnvironmentObservation {
	undetermined := false
	for _, env := range e.Existing {
		switch name.Compare(env.Name) {
		case setting.EnvironmentNameSame:
			return EnvironmentObservation{state: environmentExisting, environment: env}
		case setting.EnvironmentNameUndetermined:
			undetermined = true
		case setting.EnvironmentNameDifferent:
		}
	}
	if undetermined {
		return EnvironmentObservation{state: environmentUnavailable, reason: UnavailableEnvironmentNameUndetermined}
	}
	return EnvironmentObservation{state: environmentAbsent}
}

// observeEnvironments は Environment の一覧の全ページを読み、方式が selected の Environment にだけ、一覧の並びで個別パターンを読む。
// 個別パターンの要求を送る前にすべての要素を変換し、契約外の要素があれば要求を送らずに失敗する。
func (c *Client) observeEnvironments(ctx context.Context, reference repository.Reference) (Environments, error) {
	items, err := c.listAll(ctx, c.environmentsURL(reference), listPaging{itemsField: "environments", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return Environments{}, err
	}
	existing := make([]Environment, len(items))
	for i, item := range items {
		existing[i], err = decodeEnvironment(item)
		if err != nil {
			return Environments{}, err
		}
		// GitHub は大文字小文字だけが違う名前を同じ Environment とみなすため、二つ並ぶと照合と更新の相手が決まらない。
		// strings.EqualFold が同じとみなす名前は GitHub も同じとみなす（GitHub の同一判定はより広い）ため、別の Environment を重複と誤らない。
		if slices.ContainsFunc(existing[:i], func(seen Environment) bool { return strings.EqualFold(seen.Name, existing[i].Name) }) {
			return Environments{}, fmt.Errorf("environments: %w: duplicate environment name %q", ErrUnexpectedResponse, existing[i].Name)
		}
	}
	for i := range existing {
		e := &existing[i]
		if e.DeploymentBranchPolicy.Mode != setting.DeploymentBranchPolicySelected {
			continue
		}
		e.DeploymentBranchPolicy.Patterns, e.DeploymentPatternIDs, err = c.observeDeploymentPatterns(ctx, reference, e.Name)
		if err != nil {
			return Environments{}, err
		}
	}
	return Environments{Existing: existing}, nil
}

// observeDeploymentPatterns は、方式が selected の Environment の個別パターンを一覧の並びのまま読む。
// selected と観測したあとの失敗は、二つの要求のあいだに方式が変わった可能性があっても、404 を含めて観測不能にせず error として返す。
// パスに組み込めない名前の Environment は、要求を送らずに失敗する。
// 同じ種類と名前の個別パターンが二つ以上返ったら失敗する。ID の対応が黙って上書きされ、取り除く要求の宛先を誤らせるため。
// 失敗の文言に Environment の名前を入れるのは、どの Environment で失敗したかを示す対象の識別子で、秘密情報ではないため。%q で制御文字をエスケープする。
func (c *Client) observeDeploymentPatterns(ctx context.Context, reference repository.Reference, name string) ([]setting.DeploymentPattern, map[setting.DeploymentPattern]DeploymentPatternID, error) {
	u, ok := c.deploymentBranchPoliciesURL(reference, name)
	if !ok {
		return nil, nil, fmt.Errorf("deployment branch policies of environment %q: %w: the name cannot be sent as a path element", name, ErrUnexpectedResponse)
	}
	items, err := c.listAll(ctx, u, listPaging{itemsField: "branch_policies", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return nil, nil, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
	}
	patterns := make([]setting.DeploymentPattern, len(items))
	ids := make(map[setting.DeploymentPattern]DeploymentPatternID, len(items))
	for i, item := range items {
		pattern, id, err := decodeDeploymentPattern(item)
		if err != nil {
			return nil, nil, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
		}
		if _, duplicate := ids[pattern]; duplicate {
			return nil, nil, fmt.Errorf("deployment branch policies of environment %q: %w: duplicate deployment pattern", name, ErrUnexpectedResponse)
		}
		patterns[i] = pattern
		ids[pattern] = id
	}
	return patterns, ids, nil
}
