package changeplandisplay

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
)

// ChangePlanText は、変更計画を CLI 仕様の plan の形式で返す。
// CLI 仕様は変更不要の対象を確認した件数でだけ示すため、変更予定のないリポジトリの計画には節を書かない。
// リポジトリの計画を持たない変更計画は changeplan が生成しない。何も確認していないのに No changes. と示さないよう、不変条件の破れとして panic する。
func ChangePlanText(configPath string, p changeplan.Plan) string {
	if len(p.Repositories) == 0 {
		panic("changeplandisplay: change plan has no repository plan")
	}
	var sections strings.Builder
	repositories, changes, notComparables := 0, 0, 0
	for _, r := range p.Repositories {
		if !r.HasPlannedChanges() {
			continue
		}
		repositories++
		changes += r.Count(changeplan.Mismatch)
		notComparables += r.Count(changeplan.NotComparable)
		fmt.Fprintf(&sections, "\nRepository: %s\n", r.Target)
		for _, s := range r.Settings {
			sections.WriteString(PlannedChangeText(s))
		}
	}
	if repositories == 0 {
		return fmt.Sprintf("No changes.\nRepositories checked: %d\n", len(p.Repositories))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\n%s", configPath, sections.String())
	fmt.Fprintf(&b, "\nPlan: %s to update, %s, %s\nRepositories checked: %d\n",
		pluralize(repositories, "repository", "repositories"),
		pluralize(changes, "known change", "known changes"),
		pluralize(notComparables, "not-comparable setting", "not-comparable settings"),
		len(p.Repositories))
	return b.String()
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
