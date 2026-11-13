package apply

import (
	"fmt"
	"io"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// writeChangePlan は、承認を求める変更計画を CLI 仕様の apply の形式で、changes の並びのまま書く。
func writeChangePlan(w io.Writer, configPath string, target repository.Reference, changes []changeplan.LogicalChange) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\nRepository: %s\n", configPath, target)
	for _, change := range changes {
		fmt.Fprintf(&b, "  %s: %s -> %s\n", change.Key, change.Observed.String(), change.Desired.String())
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func writeNoChanges(w io.Writer) error {
	_, err := io.WriteString(w, "No changes.\nRepositories checked: 1\n")
	return err
}

// writeChangeReport は、一つの対象リポジトリの変更結果を CLI 仕様の apply の形式で書く。
func writeChangeReport(w io.Writer, target repository.Reference, records []changeRecord) error {
	var b strings.Builder
	switch {
	case allCompleted(records):
		fmt.Fprintf(&b, "  %s: %s\n\nApply complete.\nRepositories updated: 1\n", target, resultLabel(completed))
	case len(records) == 1:
		fmt.Fprintf(&b, "  %s: %s\n", target, resultLabel(records[0].result()))
		writeDetails(&b, "    ", details(records[0]))
	default:
		fmt.Fprintf(&b, "  %s\n", target)
		for _, record := range records {
			fmt.Fprintf(&b, "    %s: %s\n", record.change.Key, resultLabel(record.result()))
			if record.result() != completed {
				writeDetails(&b, "      ", details(record))
			}
		}
	}
	if note := changeNote(records); note != "" {
		b.WriteString("\n" + note + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func allCompleted(records []changeRecord) bool {
	for _, record := range records {
		if record.result() != completed {
			return false
		}
	}
	return true
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
			return fmt.Errorf("change %s %s: %w", record.change.Key, resultLabel(result), record.cause)
		}
		return fmt.Errorf("%s: change %s %s: %s", target, record.change.Key, resultLabel(result), strings.Join(details(record), "; "))
	}
	return nil
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
		lines = append(lines, "Update request: "+updateOutcomeText(record.update), "Desired state: "+confirmationText(record.confirmation, record.update))
	} else {
		lines = append(lines, notAttemptedReason(record))
	}
	if cause := possibleCause(record); cause != "" {
		lines = append(lines, "Possible cause: "+cause)
	}
	for _, c := range record.unexpectedOutsideChanges {
		lines = append(lines, fmt.Sprintf("Other settings: %s changed %s -> %s (cause not determined)", c.Key, c.Before.String(), c.After.String()))
	}
	return lines
}

// possibleCause は、設定に既知の原因の候補を返す。候補がなければ空文字列を返す。
func possibleCause(record changeRecord) string {
	if record.update != github.UpdateSucceeded || record.confirmation != desiredStateMismatch || record.change.Key != setting.AllowAutoMerge {
		return ""
	}
	return "auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it"
}

// notAttemptedReason は、未実行の理由を、先行する異常による停止と変更前提の不成立に分けて表示の 1 行で返す。
// 未実行の記録だけを受け取るため、更新操作を開始した記録は不変条件の破れとして panic する。
func notAttemptedReason(record changeRecord) string {
	switch record.precondition {
	case preconditionNotChecked:
		return "Stopped: a preceding change did not complete"
	case preconditionUnconfirmed:
		return "Precondition: not confirmed (read failed)"
	case preconditionIdentityMismatch:
		return "Precondition: repository ID differs from the approved repository"
	case preconditionCurrentValueChanged:
		return "Precondition: " + string(record.change.Key) + " changed since approval"
	default:
		panic(fmt.Sprintf("apply: %s has no reason for not attempting the update", record.change.Key))
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
func confirmationText(d desiredStateConfirmation, update github.UpdateOutcome) string {
	switch d {
	case desiredStateMatches:
		if update == github.UpdateUnknown {
			return "matches (update outcome still unknown)"
		}
		return "matches"
	case desiredStateMismatch:
		return "does not match"
	case desiredStateUnconfirmed:
		return "not confirmed (read failed)"
	case desiredStateRepositoryChanged:
		return "not confirmed (repository ID changed)"
	default:
		return "not checked"
	}
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
			unknown = unknown || record.update == github.UpdateUnknown
		}
	}
	if !started || allCompleted(records) {
		return ""
	}
	if unknown {
		return "Note: No automatic retry was performed."
	}
	return "Note: Some changes may already have been applied. No automatic rollback was performed."
}
