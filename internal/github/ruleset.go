package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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

// Ruleset は、存在すると確かめた一つの Ruleset の観測結果で、詳細を読んだ Ruleset では、値か、1件以上の解釈しなかったもののどちらか一方を持つ。
// 後者は、ドメインモデルの、値が解釈しない部分を含むことを理由とする観測不能に当たる。
// 読む範囲に含めず詳細を読まなかった Ruleset は、どちらも持たない。
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
	// UninterpretedEnforcement は、disabled、active、evaluate 以外の enforcement の値。GitHub が値を足し得る列挙で、足された値の意味を reco は知らない。
	UninterpretedEnforcement
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
	// UninterpretedRuleParameter は、reco が扱うルール型の、値を解釈しないか reco が読まないパラメータ。
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

// unreadRuleset は、一覧で存在を確かめたが、詳細を読まなかった Ruleset の観測結果を作る。値も解釈しなかったものも持たない。
func unreadRuleset(id RulesetID, name string) Ruleset {
	return Ruleset{ID: id, Name: name}
}

// Value は値と真を返す。解釈しない部分を含むときはゼロ値と偽を返し、呼び出し側に値として読む前の確認を強いる。
// 値を読んでいない Ruleset とゼロ値では、値が空であることとも解釈しない部分を含むこととも取り違えないよう、不変条件の破れとして panic する（ドメインモデルの存在の観測）。
func (r Ruleset) Value() (setting.Ruleset, bool) {
	if !r.interpreted && len(r.uninterpreted) == 0 {
		panic(fmt.Sprintf("github: the value of ruleset %q was not read in this observation", r.Name))
	}
	return r.value, r.interpreted
}

// UninterpretedParts は解釈しなかったものを返す。全体を解釈できた Ruleset とゼロ値では、不変条件の破れとして panic する。
func (r Ruleset) UninterpretedParts() []UninterpretedPart {
	if len(r.uninterpreted) == 0 {
		panic(fmt.Sprintf("github: ruleset %q has no uninterpreted parts", r.Name))
	}
	return r.uninterpreted
}

// Lookup は、同一性を判定できる名前 name の Ruleset を、一覧の全体と突き合わせて引く。
// 呼び出し側が一覧を綴りで探すと、存在する Ruleset を存在しないと判断し得るため、ここでだけ引く。
// 同じ名前の要素が二つ以上ないことは、observeRulesets が取得契約として確かめている。
// 一覧が観測不能なら、同じ理由の観測不能にする。
func (r Rulesets) Lookup(name setting.RulesetName) RulesetObservation {
	existing, ok := r.Existing.Value()
	if !ok {
		return RulesetObservation{state: resourceUnavailable, reason: r.Existing.UnavailableReason()}
	}
	undetermined := false
	for _, ruleset := range existing {
		switch name.Compare(ruleset.Name) {
		case setting.NameSame:
			return RulesetObservation{state: resourceExisting, ruleset: ruleset}
		case setting.NameUndetermined:
			undetermined = true
		case setting.NameDifferent:
		}
	}
	if undetermined {
		return RulesetObservation{state: resourceUnavailable, reason: UnavailableNameUndetermined}
	}
	return RulesetObservation{state: resourceAbsent}
}

// String は、解釈しなかったもの一つを、種類と、引用符で囲んでエスケープした名前の一行で返す。形は CLI 仕様の解釈しなかったものの表に従う。
// 表示の形を外部クライアントの package に置くのは、状態確認（show）と変更計画の表示（changeplandisplay）が同じ形を使い、
// show は変更計画の package に依存しないため、両方が使える置き場が解釈しなかったものの型を持つこの package だけだからである。
// 名前は GitHub が返した任意の文字列のため、strconv.Quote で改行や表示の向きを変える文字をエスケープし、一行に収める。
// 種類を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かず、定義にない値は panic する。
func (p UninterpretedPart) String() string {
	name := strconv.Quote(p.Name)
	switch p.Kind {
	case UninterpretedTarget:
		return "target: " + name
	case UninterpretedEnforcement:
		return "enforcement: " + name
	case UninterpretedConditionsKey:
		return "conditions key: " + name
	case UninterpretedBypassActorType:
		return "bypass actor type: " + name
	case UninterpretedBypassMode:
		return "bypass mode: " + name
	case UninterpretedBypassActorKey:
		return "bypass actor key: " + name
	case UninterpretedRuleType:
		return "rule type: " + name
	case UninterpretedRuleKey:
		return string(p.Rule) + " key: " + name
	case UninterpretedRuleParameter:
		return string(p.Rule) + " parameter: " + name
	}
	panic(fmt.Sprintf("github: undefined uninterpreted kind %d", int(p.Kind)))
}

// observeRulesets は、公開範囲が取得条件に当たれば Ruleset の要求を一つも送らずに一覧を観測不能とし、当たらなければ一覧を読む。
// 詳細は scope にある Ruleset にだけ一覧の並びで読み、ほかの Ruleset は値を読まないまま一覧の並びに残す。
// 公開と観測したあとの失敗の扱いは、Rulesets の設計の読み取り要求に従う。
func (c *Client) observeRulesets(ctx context.Context, reference repository.Reference, private bool, scope resourceScope) (Rulesets, error) {
	if reason, unavailable := rulesetsUnavailability(private); unavailable {
		return Rulesets{Existing: Unavailable[[]Ruleset](reason)}, nil
	}
	items, err := c.listSinglePage(ctx, c.rulesetListURL(reference))
	if err != nil {
		return Rulesets{}, err
	}
	summaries, err := decodeRulesetSummaries(items)
	if err != nil {
		return Rulesets{}, err
	}
	existing := make([]Ruleset, len(summaries))
	for i, summary := range summaries {
		if !scope.includesRuleset(summary.name) {
			existing[i] = unreadRuleset(summary.id, summary.name)
			continue
		}
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
