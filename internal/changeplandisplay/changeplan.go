package changeplandisplay

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
)

// Text は、変更計画を表示仕様の plan の形式で返す。
// 表示仕様は変更不要の対象を確認した件数でだけ示すため、変更予定のないリポジトリの計画には節を書かない。
// リポジトリの計画を持たない変更計画は changeplan が生成しない。何も確認していないのに No changes. と示さないよう、不変条件の破れとして panic する。
func Text(configPath string, p changeplan.Plan) string {
	if len(p.Repositories) == 0 {
		panic("changeplandisplay: change plan has no repository plan")
	}
	var sections strings.Builder
	repositories, changes := 0, 0
	notApplicable := false
	for _, r := range p.Repositories {
		if !r.HasPlannedChanges() {
			continue
		}
		repositories++
		changes += r.Count(changeplan.Mismatch) + r.Count(changeplan.NotComparable)
		fmt.Fprintf(&sections, "\nRepository: %s\n", r.Target)
		for _, s := range r.Settings {
			sections.WriteString(PlannedChangeText(s))
		}
		notApplicable = notApplicable || len(r.NotApplicableChanges()) > 0
	}
	if repositories == 0 {
		return fmt.Sprintf("No changes.\nRepositories checked: %d\n", len(p.Repositories))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\n%s", configPath, sections.String())
	fmt.Fprintf(&b, "\nPlan: %s to update, %s\nRepositories checked: %d\n",
		pluralize(repositories, "repository", "repositories"),
		pluralize(changes, "change", "changes"),
		len(p.Repositories))
	if notApplicable {
		b.WriteString("\nPlan cannot be applied.\n")
	}
	return b.String()
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
