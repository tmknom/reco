package github

import (
	"fmt"
	"strconv"

	"github.com/tmknom/reco/internal/setting"
)

// UninterpretedPart は、リポジトリリソースの値か中の設定のうち reco が解釈しなかったもの一つ。
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
	// UninterpretedProtectionRuleType は、Environment の保護ルールのうち、wait_timer、required_reviewers、branch_policy 以外の type。
	UninterpretedProtectionRuleType
)

// String は、解釈しなかったもの一つを、種類と、引用符で囲んでエスケープした名前の一行で返す。
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
	case UninterpretedProtectionRuleType:
		return "protection rule type: " + name
	}
	panic(fmt.Sprintf("github: undefined uninterpreted kind %d", int(p.Kind)))
}
