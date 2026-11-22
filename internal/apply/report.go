package apply

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// writeChangeReport は、選択したすべての対象リポジトリの変更結果を、宣言順に CLI 仕様の apply の形式で書く。
// 注記は、先に完了した対象リポジトリの変更も GitHub に残るため、すべての対象リポジトリの記録から判定する。
func writeChangeReport(w io.Writer, records []repositoryRecord) error {
	var b strings.Builder
	var all []changeRecord
	updated := 0
	for _, r := range records {
		writeRepositoryReport(&b, r)
		all = append(all, r.changes...)
		if r.result() == repositoryCompleted {
			updated++
		}
	}
	if allCompleted(all) {
		fmt.Fprintf(&b, "\nApply complete.\nRepositories updated: %d\n", updated)
	}
	if note := changeNote(all); note != "" {
		b.WriteString("\n" + note + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeRepositoryReport は、一つの対象リポジトリの変更結果を書く。
// 先行する対象リポジトリの異常で開始しなかった対象リポジトリは、どの論理的な設定変更も同じ理由の未実行で、
// 分けて示しても情報が増えないため、論理的な設定変更の数によらずリポジトリの行に一つ示す。
func writeRepositoryReport(b *strings.Builder, r repositoryRecord) {
	switch r.result() {
	case repositoryNoChange:
		fmt.Fprintf(b, "  %s: no change\n", r.target)
	case repositoryCompleted:
		fmt.Fprintf(b, "  %s: %s\n", r.target, resultLabel(completed))
	case repositoryStopped:
		fmt.Fprintf(b, "  %s: %s\n", r.target, resultLabel(notAttempted))
		writeDetails(b, "    ", []string{"Stopped: a preceding repository did not complete"})
	case repositoryNotCompleted:
		if len(r.changes) == 1 {
			fmt.Fprintf(b, "  %s: %s\n", r.target, resultLabel(r.changes[0].result()))
			writeDetails(b, "    ", details(r.changes[0]))
			return
		}
		fmt.Fprintf(b, "  %s\n", r.target)
		for _, record := range r.changes {
			fmt.Fprintf(b, "    %s: %s\n", changeLabel(record.change), resultLabel(record.result()))
			if record.result() != completed {
				writeDetails(b, "      ", details(record))
			}
		}
	}
}

// repositoriesError は、宣言順で最初の完了以外の変更結果を error にする。
// 停止した対象リポジトリは、その原因が先行する対象リポジトリの記録にあるため error を足さない。
// 停止した対象リポジトリより前には必ず完了以外を含む対象リポジトリがあり、最初の error で返せば停止の記録に届かない。
func repositoriesError(records []repositoryRecord) error {
	for _, r := range records {
		if err := changesError(r.changes, r.target); err != nil {
			return err
		}
	}
	return nil
}

// changesError は、最初の完了以外の変更結果を error にする。
// 先行する異常による停止の記録は、その原因が先行する記録にあるため error を足さない。
// cause の文言は対象名と失敗した操作を含むため、設定と変更結果だけを前置する。cause がなければ対象名と理由を書く。
func changesError(records []changeRecord, target repository.Reference) error {
	for _, record := range records {
		result := record.result()
		if result == completed {
			continue
		}
		if record.cause != nil {
			return fmt.Errorf("change %s %s: %w", changeLabel(record.change), resultLabel(result), record.cause)
		}
		return fmt.Errorf("%s: change %s %s: %s", target, changeLabel(record.change), resultLabel(result), strings.Join(details(record), "; "))
	}
	return nil
}

// changeLabel は、変更結果で論理的な設定変更を示す名前を返す。変える管理設定の名前を、期待状態の並びで " + " でつなぐ。
func changeLabel(change changeplan.LogicalChange) string {
	units := change.Units()
	names := make([]string, len(units))
	for i, unit := range units {
		names[i] = unit.String()
	}
	return strings.Join(names, " + ")
}

func resultLabel(r changeResult) string {
	switch r {
	case completed:
		return "updated"
	case failed:
		return "failed"
	default:
		return "not attempted"
	}
}

// details は、完了以外の変更結果の理由を表示の 1 行ずつで返す。
func details(record changeRecord) []string {
	var lines []string
	if record.updateStarted() {
		lines = append(lines, updateRequestLines(record)...)
		lines = append(lines, "Desired state: "+confirmationText(record))
	} else {
		lines = append(lines, notAttemptedReason(record))
	}
	if cause := possibleCause(record); cause != "" {
		lines = append(lines, "Possible cause: "+cause)
	}
	for _, c := range record.unexpectedOutsideChanges {
		lines = append(lines, fmt.Sprintf("Other settings: %s changed %s -> %s (cause not determined)", c.Setting, c.Before, c.After))
	}
	return lines
}

// updateRequestRow は、変更結果の Update request の 1 行が表す、続けて並ぶ更新操作のまとまり。
// operation は先頭の更新操作で、patterns は個別パターンの更新操作のときだけ意味を持つ、まとめた要素の実行順の並び。
// precondition は、まとまりの中に変更前提の不成立で開始しなかった更新操作があるときの、その理由の行である。
type updateRequestRow struct {
	operation    changeplan.UpdateOperation
	patterns     []setting.DeploymentPattern
	outcome      string
	precondition string
}

// updateRequestLines は、開始した論理的な設定変更の更新操作の結果を表示の行で返す。
// 更新操作が一つなら、どの更新操作かを示さずに 1 行で示す。
// 二つ以上なら更新操作の結果を実行順に示し、開始しなかった更新操作は、変更前提を確かめていれば、それを含む行の次に不成立の理由の行を続ける。
// 個別パターンの更新操作は数が多くなり得るため、加えるか取り除くかと結果が同じで続けて並ぶものを 1 行にまとめる。
func updateRequestLines(record changeRecord) []string {
	if len(record.change.Operations) == 1 {
		return []string{"Update request: " + updateOutcomeText(record.operation(0).update)}
	}
	var rows []updateRequestRow
	for i, operation := range record.change.Operations {
		o := record.operation(i)
		outcome := "not started"
		if o.started() {
			outcome = updateOutcomeText(o.update)
		}
		last := len(rows) - 1
		switch {
		case !changesDeploymentPattern(operation):
			rows = append(rows, updateRequestRow{operation: operation, outcome: outcome})
		case last >= 0 && rows[last].operation.Kind == operation.Kind && rows[last].outcome == outcome:
			rows[last].patterns = append(rows[last].patterns, operation.Pattern)
		default:
			rows = append(rows, updateRequestRow{operation: operation, patterns: []setting.DeploymentPattern{operation.Pattern}, outcome: outcome})
		}
		if !o.started() && o.precondition != preconditionNotChecked {
			rows[len(rows)-1].precondition = preconditionText(record.change, i, o)
		}
	}
	var lines []string
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("Update request (%s): %s", updateRequestLabel(row.operation, row.patterns), row.outcome))
		if row.precondition != "" {
			lines = append(lines, row.precondition)
		}
	}
	return lines
}

// updateRequestLabel は、Update request の行の括弧の中に示す対象を返す。
// 個別パターンの更新操作は、変える管理設定がまとまりの中で同じため、加えるか取り除くかと要素 patterns で示す。要素は plan と同じ表記にする。
// それ以外の更新操作は、更新操作の管理の単位の名前で示す。
// 未定義の種類は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func updateRequestLabel(operation changeplan.UpdateOperation, patterns []setting.DeploymentPattern) string {
	switch operation.Kind {
	case changeplan.SettingUpdate, changeplan.EnvironmentCreation, changeplan.EnvironmentCreationWithMode, changeplan.DeploymentBranchPolicyModeUpdate,
		changeplan.RepositoryVariableCreation, changeplan.RepositoryVariableValueUpdate,
		changeplan.EnvironmentVariableCreation, changeplan.EnvironmentVariableValueUpdate:
		return operation.Unit.String()
	case changeplan.DeploymentPatternAddition:
		return "patterns to add " + setting.DeploymentBranchPolicy{Patterns: patterns}.PatternsString()
	case changeplan.DeploymentPatternRemoval:
		return "patterns to remove " + setting.DeploymentBranchPolicy{Patterns: patterns}.PatternsString()
	}
	panic(fmt.Sprintf("apply: undefined kind %d of an update operation", int(operation.Kind)))
}

// changesDeploymentPattern は、operation が個別パターンを一つ加えるか取り除く更新操作かを返す。
func changesDeploymentPattern(operation changeplan.UpdateOperation) bool {
	return operation.Kind == changeplan.DeploymentPatternAddition || operation.Kind == changeplan.DeploymentPatternRemoval
}

// possibleCause は、設定に既知の原因の候補を返す。候補がなければ空文字列を返す。
func possibleCause(record changeRecord) string {
	if !record.updatesSucceeded() || record.confirmation != desiredStateMismatch || !slices.Equal(record.change.Units(), []setting.Unit{{Key: setting.AllowAutoMerge}}) {
		return ""
	}
	return "auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it"
}

// notAttemptedReason は、未実行の理由を、先行する異常による停止と変更前提の不成立に分けて表示の 1 行で返す。
func notAttemptedReason(record changeRecord) string {
	if len(record.operations) == 0 {
		return "Stopped: a preceding change did not complete"
	}
	return preconditionText(record.change, 0, record.operations[0])
}

// preconditionText は、change の index 番目の更新操作の変更前提が成立しなかった理由を表示の 1 行で返す。
// 後続の更新操作の関係する現在値の変化は、先行する更新操作の反映の遅れでも外部主体の変更でも起き、
// 値が承認のあとに変わったとは限らないため、予定どおりの値でないことを示す。
// 先頭の更新操作の承認時に予定した値が、先行する論理的な設定変更の結果 (有効にした Actions) の場合も、
// その変更は完了を確かめていて観測し直しの対象にしないため、原因を断定せずに関係する現在値の変化として示す。
// ただし、後続の更新操作でも、承認時に観測した Environment そのものを基準にした変化は、先行する更新操作が変えるものではないため、承認のあとの変化として示す。
// 先頭の更新操作が作成の変更では、後続の Environment の基準は先行する作成で、
// Environment は値を持たないため、予定どおりの値ではなく、予定どおりの Environment でないことを示す。
// 不成立の記録だけを受け取るため、変更前提が成立した記録と確かめていない記録は不変条件の破れとして panic する。
func preconditionText(change changeplan.LogicalChange, index int, o operationRecord) string {
	switch o.precondition {
	case preconditionUnconfirmed:
		return "Precondition: not confirmed (read failed)"
	case preconditionIdentityMismatch:
		return "Precondition: repository ID differs from the approved repository"
	case preconditionCurrentValueChanged:
		switch {
		case index == 0:
			return "Precondition: " + o.changedSetting.String() + " changed since approval"
		case o.changedSetting.Key == setting.EnvironmentResource && change.Operations[0].CreatesEnvironment():
			return "Precondition: " + o.changedSetting.String() + " is not the environment expected after the preceding update"
		case o.changedSetting.Key == setting.EnvironmentResource:
			return "Precondition: " + o.changedSetting.String() + " changed since approval"
		default:
			return "Precondition: " + o.changedSetting.String() + " is not the value expected after the preceding update"
		}
	default:
		panic(fmt.Sprintf("apply: %s has no reason for not starting the update", change.Operations[index].Unit.String()))
	}
}

// updateOutcomeText は、更新操作の結果の表示を返す。成否不明の原因は error が示すため、ここでは区別しない。
func updateOutcomeText(u github.UpdateOutcome) string {
	switch u {
	case github.UpdateSucceeded:
		return "succeeded"
	case github.UpdateRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// confirmationText は、更新操作の結果に応じた期待状態の確認結果の表示を返す。
// 成否不明のときの一致は、更新操作の成否が判明したことにならないため、その旨を添える。
func confirmationText(record changeRecord) string {
	switch record.confirmation {
	case desiredStateMatches:
		if record.hasUnknownOutcome() {
			return "matches (update outcome still unknown)"
		}
		return "matches"
	case desiredStateMismatch:
		return "does not match"
	case desiredStateUnconfirmed:
		return "not confirmed (read failed)"
	case desiredStateIdentityMismatch:
		return "not confirmed (repository ID changed)"
	case desiredStateUnavailable:
		// 判定できない名前は、宣言した設定が効いていないのではなく reco が存在を確かめられなかったことを表すため、not in effect と示さない
		if record.desiredUnavailableReason == github.UnavailableEnvironmentNameUndetermined {
			return "not confirmed (environment existence unknown)"
		}
		return "not in effect (" + unavailableReasonText(record.desiredUnavailableReason) + ")"
	default:
		return "not checked"
	}
}

// unavailableReasonText は、期待状態の確認で観測不能だったときに、宣言した設定が効いていない理由を CLI 仕様の文言で返す。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func unavailableReasonText(reason github.UnavailableReason) string {
	switch reason {
	case github.UnavailableActionsDisabled:
		return "GitHub Actions is disabled"
	case github.UnavailableNotSelected:
		return `allowed_actions is not "selected"`
	case github.UnavailablePublicRepository:
		return "the repository is public"
	case github.UnavailableEnvironmentAbsent:
		return "the environment does not exist"
	case github.UnavailableEnvironmentNameUndetermined:
		// confirmationText が not in effect と示さないため、理由の文言を持たない
	case github.UnavailablePrivateRepository:
		return "the repository is private"
	}
	panic(fmt.Sprintf("apply: undefined unavailable reason %d", int(reason)))
}

func writeDetails(b *strings.Builder, indent string, lines []string) {
	for _, line := range lines {
		b.WriteString(indent + line + "\n")
	}
}

// changeNote は、変更結果の末尾に添える注記を返す。添えないときは空文字列を返す。
// 更新操作を一度も開始していなければ、GitHub に変更が残り得ないため添えない。
// 先に完了した変更のあとで止まった場合も適用済みの変更が残るため、最後の記録だけでなく、開始したすべての更新操作から選ぶ。
func changeNote(records []changeRecord) string {
	started, unknown := false, false
	for _, record := range records {
		if record.updateStarted() {
			started = true
			unknown = unknown || record.hasUnknownOutcome()
		}
	}
	if !started || allCompleted(records) {
		return ""
	}
	const nextStep = " Run reco plan to see the remaining changes."
	if unknown {
		return "Note: No automatic retry was performed." + nextStep
	}
	return "Note: Some changes may already have been applied. No automatic rollback was performed." + nextStep
}
