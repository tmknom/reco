package github

import (
	"context"
	"fmt"
	"strings"
	"unicode"

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
	// 方式が selected で nil なのは読む範囲に含めなかった Environment で、個別パターンが一つもないこととは異なる。
	DeploymentPatterns *DeploymentPatterns
	Protection         setting.EnvironmentProtection
	// Variables は、Environment Variables の領域を選ばなかった観測と、読む範囲に含めなかった Environment では nil になる。
	// Variable が一つもないことと取り違えさせないため、値型にしない。
	Variables *Variables
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

// DeploymentBranchPolicy は、デプロイ元制限の観測値を返す。方式が selected なら、読んだ個別パターンを読んだ順で含む。
func (e Environment) DeploymentBranchPolicy() setting.DeploymentBranchPolicy {
	if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected {
		return setting.DeploymentBranchPolicy{Mode: e.DeploymentBranchPolicyMode}
	}
	return setting.DeploymentBranchPolicy{Mode: e.DeploymentBranchPolicyMode, Patterns: e.readDeploymentPatterns().Existing}
}

// DeploymentPatternID は、読んだ個別パターン pattern の ID を返す。方式が selected でないか、pattern を読んでいなければ偽を返す。
func (e Environment) DeploymentPatternID(pattern setting.DeploymentPattern) (DeploymentPatternID, bool) {
	if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected {
		return 0, false
	}
	id, ok := e.readDeploymentPatterns().IDs[pattern]
	return id, ok
}

// readDeploymentPatterns は、方式が selected の Environment の読んだ個別パターンを返す。
// 読む範囲に含めなかった Environment は、個別パターンが一つもないことと取り違えないよう、不変条件の破れとして panic する（Environments の設計の観測）。
func (e Environment) readDeploymentPatterns() DeploymentPatterns {
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

// observeEnvironments は Environment の一覧の全ページを読み、scope にある方式が selected の Environment にだけ、一覧の並びで個別パターンを読む。
// 個別パターンの要求を送る前にすべての要素を変換し、契約外の要素があれば要求を送らずに失敗する。
func (c *Client) observeEnvironments(ctx context.Context, reference repository.Reference, scope environmentScope) (Environments, error) {
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
		key := environmentNameFoldKey(existing[i].Name)
		if _, duplicate := seen[key]; duplicate {
			return Environments{}, fmt.Errorf("environments: %w: duplicate environment name %q", ErrUnexpectedResponse, existing[i].Name)
		}
		seen[key] = struct{}{}
	}
	for i := range existing {
		e := &existing[i]
		if e.DeploymentBranchPolicyMode != setting.DeploymentBranchPolicySelected || !scope.includes(e.Name) {
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

// observeDeploymentPatterns は、方式が selected の Environment の個別パターンを一覧の並びのまま読む。
// selected と観測したあとの失敗は、二つの要求のあいだに方式が変わった可能性があっても、404 を含めて観測不能にせず error として返す。
// パスに組み込めない名前の Environment は、要求を送らずに失敗する。
// 同じ種類と名前の個別パターンが二つ以上返ったら失敗する。ID の対応が黙って上書きされ、取り除く要求の宛先を誤らせるため。
// 失敗の文言に Environment の名前を入れるのは、どの Environment で失敗したかを示す対象の識別子で、秘密情報ではないため。%q で制御文字をエスケープする。
func (c *Client) observeDeploymentPatterns(ctx context.Context, reference repository.Reference, name string) (DeploymentPatterns, error) {
	u, ok := c.deploymentBranchPoliciesURL(reference, name)
	if !ok {
		return DeploymentPatterns{}, fmt.Errorf("deployment branch policies of environment %q: %w: the name cannot be sent as a path element", name, ErrUnexpectedResponse)
	}
	items, err := c.listAll(ctx, u, listPaging{itemsField: "branch_policies", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return DeploymentPatterns{}, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
	}
	patterns := make([]setting.DeploymentPattern, len(items))
	ids := make(map[setting.DeploymentPattern]DeploymentPatternID, len(items))
	for i, item := range items {
		pattern, id, err := decodeDeploymentPattern(item)
		if err != nil {
			return DeploymentPatterns{}, fmt.Errorf("deployment branch policies of environment %q: %w", name, err)
		}
		if _, duplicate := ids[pattern]; duplicate {
			return DeploymentPatterns{}, fmt.Errorf("deployment branch policies of environment %q: %w: duplicate deployment pattern", name, ErrUnexpectedResponse)
		}
		patterns[i] = pattern
		ids[pattern] = id
	}
	return DeploymentPatterns{Existing: patterns, IDs: ids}, nil
}

// environmentNameFoldKey は、strings.EqualFold が同じとみなす名前どうしで、またそのときに限って一致する鍵を返す。
// 一覧の要素数の2乗にならずに重複を検査するため。EqualFold は rune ごとに unicode.SimpleFold の軌道が同じかで比べるため、
// 各 rune を軌道の最小の rune に置き換える。大文字化と小文字化の往復では、軌道の違う U+0131（LATIN SMALL LETTER DOTLESS I）と i が同じになり、使えない。
func environmentNameFoldKey(name string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			smallest = min(smallest, f)
		}
		return smallest
	}, name)
}
