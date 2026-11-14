// Package display は、複数の操作が同じ文言で示す表示のうち、観測の概念だけで決まるものを持つ。
package display

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
)

// UnavailableText は観測不能を CLI 仕様の unavailable (<理由>) の形で返す。未定義の理由は不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func UnavailableText(reason github.UnavailableReason) string {
	switch reason {
	case github.UnavailableActionsDisabled:
		return "unavailable (GitHub Actions is disabled)"
	case github.UnavailableNotSelected:
		return `unavailable (allowed_actions is not "selected")`
	}
	panic(fmt.Sprintf("display: undefined unavailable reason %d", int(reason)))
}
