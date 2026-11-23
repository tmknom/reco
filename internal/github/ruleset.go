package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"

	"github.com/tmknom/reco/internal/setting"
)

// Rulesets は、Rulesets の領域を読み取った観測の結果である。
type Rulesets struct {
	// Existing は、一覧を読めた観測では存在するすべての Ruleset を一覧の並びで持つ。
	// 非公開リポジトリでは一覧を読まず、理由を伴う観測不能になる。Ruleset が一つもないこととは異なる。
	Existing SettingObservation[[]Ruleset]
}

// RulesetID は、GitHub が Ruleset の実体に付ける ID。同じ名前で作り直すと変わる。
type RulesetID int64

// Ruleset は、存在すると確かめた一つの Ruleset の観測結果で、値か、1件以上の解釈しなかったもののどちらか一方を持つ。
// 後者は、ドメインモデルの、値が解釈しない部分を含むことを理由とする観測不能に当たる。
// 解釈できた部分だけの値を作らないよう、フィールドを公開せずコンストラクタでだけ作る。
type Ruleset struct {
	ID RulesetID
	// Name は GitHub が返した綴りのまま持つ。
	Name          string
	value         setting.Ruleset
	interpreted   bool
	uninterpreted []UninterpretedPart
}

// UninterpretedPart は、Ruleset の値のうち reco が解釈しなかったもの一つ。
type UninterpretedPart struct {
	Kind UninterpretedKind
	// Rule は、Kind が UninterpretedRuleKey か UninterpretedRuleParameter のときだけ、そのルール型を持つ。
	Rule setting.RulesetRuleType
	// Name は GitHub が返した値かキーの名前のまま持つ。任意の文字を含み得る。
	Name string
}

// UninterpretedKind は、解釈しなかったものの種類。
type UninterpretedKind int

// ゼロ値を定義済みの種類と取り違えないよう 1 から始める。
const (
	// UninterpretedTarget は、branch 以外の target の値。
	UninterpretedTarget UninterpretedKind = iota + 1
	// UninterpretedConditionsKey は、conditions か ref_name の reco が読まないキー。ref_name の中のキーは ref_name.<キー> の名前で持つ。
	UninterpretedConditionsKey
	// UninterpretedBypassActorType は、reco が扱わない actor_type の値。
	UninterpretedBypassActorType
	// UninterpretedBypassMode は、reco が扱わない bypass_mode の値。
	UninterpretedBypassMode
	// UninterpretedBypassActorKey は、Bypass の要素の reco が読まないキー。
	UninterpretedBypassActorKey
	// UninterpretedRuleType は、reco が扱わないルール型。
	UninterpretedRuleType
	// UninterpretedRuleKey は、reco が扱うルール型の要素の reco が読まないキー。
	UninterpretedRuleKey
	// UninterpretedRuleParameter は、reco が扱うルール型の、既定値と異なるか reco が読まないパラメータ。
	UninterpretedRuleParameter
)

// InterpretedRuleset は、全体を解釈できた Ruleset の観測結果を作る。
func InterpretedRuleset(id RulesetID, name string, value setting.Ruleset) Ruleset {
	return Ruleset{ID: id, Name: name, value: value, interpreted: true}
}

// UninterpretedRuleset は、parts を解釈しなかった Ruleset の観測結果を作る。
// parts が空なら、解釈しなかったものを示せない観測不能になるため、不変条件の破れとして panic する。
func UninterpretedRuleset(id RulesetID, name string, parts []UninterpretedPart) Ruleset {
	if len(parts) == 0 {
		panic(fmt.Sprintf("github: uninterpreted ruleset %q has no uninterpreted parts", name))
	}
	return Ruleset{ID: id, Name: name, uninterpreted: parts}
}

// Value は値と真を返す。解釈しない部分を含むときはゼロ値と偽を返し、呼び出し側に値として読む前の確認を強いる。
func (r Ruleset) Value() (setting.Ruleset, bool) {
	return r.value, r.interpreted
}

// UninterpretedParts は解釈しなかったものを返す。全体を解釈できた Ruleset とゼロ値では、不変条件の破れとして panic する。
func (r Ruleset) UninterpretedParts() []UninterpretedPart {
	if len(r.uninterpreted) == 0 {
		panic(fmt.Sprintf("github: ruleset %q has no uninterpreted parts", r.Name))
	}
	return r.uninterpreted
}

// observeRulesets は、公開範囲が取得条件に当たれば Ruleset の要求を一つも送らずに一覧を観測不能とし、当たらなければすべての Ruleset を一覧の並びで読む。
// 公開と観測したあとの失敗の扱いは、Rulesets の設計の読み取り要求に従う。
func (c *Client) observeRulesets(ctx context.Context, reference repository.Reference, private bool) (Rulesets, error) {
	if reason, unavailable := rulesetsUnavailability(private); unavailable {
		return Rulesets{Existing: Unavailable[[]Ruleset](reason)}, nil
	}
	items, err := c.listSinglePage(ctx, c.rulesetsURL(reference))
	if err != nil {
		return Rulesets{}, err
	}
	summaries, err := decodeRulesetSummaries(items)
	if err != nil {
		return Rulesets{}, err
	}
	existing := make([]Ruleset, len(summaries))
	for i, summary := range summaries {
		existing[i], err = c.observeRuleset(ctx, reference, summary)
		if err != nil {
			return Rulesets{}, fmt.Errorf("ruleset %q: %w", summary.name, err)
		}
	}
	return Rulesets{Existing: Observed(existing)}, nil
}

// observeRuleset は、一覧の要素 summary の Ruleset の詳細を読み、一覧と同じ実体から読んだと確かめる。
func (c *Client) observeRuleset(ctx context.Context, reference repository.Reference, summary rulesetSummary) (Ruleset, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.rulesetURL(reference, summary.id), nil, http.StatusOK)
	if err != nil {
		return Ruleset{}, err
	}
	detail, err := decodeRuleset(body)
	if err != nil {
		return Ruleset{}, err
	}
	if err := checkRulesetDetail(summary, detail); err != nil {
		return Ruleset{}, err
	}
	return detail, nil
}

// rulesetsUnavailability は、観測した公開範囲から、Rulesets の一覧が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
// Private vulnerability reporting と同じ条件だが、別々に確かめた取得条件で片方だけが変わり得るため、共有しない。
func rulesetsUnavailability(private bool) (reason UnavailableReason, unavailable bool) {
	if private {
		return UnavailablePrivateRepository, true
	}
	return 0, false
}

// checkRulesetDetail は、詳細 detail が一覧の要素 summary と同じ実体から読んだものかを確かめる。
// ID か名前が違えば、二つの要求のあいだに作り直されたか、別の Ruleset を読んだため、値を一覧の名前に結び付けない。名前は綴りの完全一致で比べる。
func checkRulesetDetail(summary rulesetSummary, detail Ruleset) error {
	if detail.ID != summary.id || detail.Name != summary.name {
		return fmt.Errorf("%w: the ruleset differs from the one in the list", ErrUnexpectedResponse)
	}
	return nil
}
