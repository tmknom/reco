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

// writeChangePlan は、承認を求める変更計画を CLI 仕様の apply の形式で書く。
func writeChangePlan(w io.Writer, configPath string, target repository.Reference, change changeplan.LogicalChange) error {
	_, err := fmt.Fprintf(w, "Configuration: %s\nRepository: %s\n  %s: %t -> %t\n\n",
		configPath, target, change.Key, change.Observed, change.Desired)
	return err
}

func writeNoChanges(w io.Writer) error {
	_, err := io.WriteString(w, "No changes.\nRepositories checked: 1\n")
	return err
}

// writeChangeReport は変更結果を CLI 仕様の apply の形式で書く。
func writeChangeReport(w io.Writer, target repository.Reference, change changeplan.LogicalChange, record changeRecord) error {
	result := record.result()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s: %s\n", target, resultLabel(result))
	switch result {
	case completed:
		b.WriteString("\nApply complete.\nRepositories updated: 1\n")
	case failed:
		writeDetails(&b, details(record, change))
		b.WriteString("\n" + failureNote(record.update) + "\n")
	default:
		writeDetails(&b, details(record, change))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// changeError は、完了以外の変更結果を error にする。
// cause の文言は対象名と失敗した操作を含むため、変更結果だけを前置する。cause がなければ対象名と理由を書く。
func changeError(record changeRecord, target repository.Reference, change changeplan.LogicalChange) error {
	result := record.result()
	if result == completed {
		return nil
	}
	if record.cause != nil {
		return fmt.Errorf("change %s: %w", resultLabel(result), record.cause)
	}
	return fmt.Errorf("%s: change %s: %s", target, resultLabel(result), strings.Join(details(record, change), "; "))
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
func details(record changeRecord, change changeplan.LogicalChange) []string {
	var lines []string
	if record.updateStarted() {
		lines = append(lines, "Update request: "+updateOutcomeText(record.update), "Desired state: "+confirmationText(record.confirmation, record.update))
	} else {
		lines = append(lines, "Precondition: "+preconditionReason(record.precondition, change.Key))
	}
	if cause := possibleCause(record, change.Key); cause != "" {
		lines = append(lines, "Possible cause: "+cause)
	}
	for _, c := range record.unexpectedUnmanagedChanges {
		lines = append(lines, fmt.Sprintf("Unmanaged settings: %s changed %t -> %t (cause not determined)", c.Key, c.Before, c.After))
	}
	return lines
}

// possibleCause は、設定に既知の原因の候補を返す。候補がなければ空文字列を返す。
func possibleCause(record changeRecord, key setting.Key) string {
	if record.update != github.UpdateSucceeded || record.confirmation != desiredStateMismatch || key != setting.AllowAutoMerge {
		return ""
	}
	return "auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it"
}

func preconditionReason(p preconditionCheck, key setting.Key) string {
	switch p {
	case preconditionIdentityMismatch:
		return "repository ID differs from the approved repository"
	case preconditionCurrentValueChanged:
		return string(key) + " changed since approval"
	default:
		return "not confirmed (read failed)"
	}
}

func updateOutcomeText(u github.UpdateOutcome) string {
	switch u {
	case github.UpdateSucceeded:
		return "succeeded"
	case github.UpdateRejected:
		return "rejected"
	default:
		return "unknown (response unavailable)"
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

func writeDetails(b *strings.Builder, lines []string) {
	for _, line := range lines {
		b.WriteString("    " + line + "\n")
	}
}

func failureNote(update github.UpdateOutcome) string {
	if update == github.UpdateUnknown {
		return "Note: No automatic retry was performed."
	}
	return "Note: Some changes may already have been applied. No automatic rollback was performed."
}
