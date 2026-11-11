package apply

import (
	"fmt"
	"io"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/repository"
)

// changeReport は、承認した変更を実行した結果。ゼロ値は、更新操作を開始していない状態を表す。
type changeReport struct {
	precondition     preconditionCheck
	update           updateOutcome
	confirmation     desiredStateConfirmation
	unmanagedChanges []unmanagedChange
	// cause は、前提の確認、更新操作、更新後の再取得のうち、失敗した操作の error。
	cause error
}

// changeResult は、変更結果の表示区分。
type changeResult int

const (
	notAttempted changeResult = iota
	failed
	completed
)

func (r changeResult) String() string {
	switch r {
	case completed:
		return "updated"
	case failed:
		return "failed"
	default:
		return "not attempted"
	}
}

// preconditionCheck は、更新直前に変更前提を確かめた結果。
type preconditionCheck int

const (
	preconditionUnconfirmed preconditionCheck = iota
	preconditionHolds
	preconditionIdentityMismatch
	preconditionCurrentValueChanged
)

// updateOutcome は、更新操作の成否。
type updateOutcome int

const (
	updateNotStarted updateOutcome = iota
	updateSucceeded
	updateFailed
	updateOutcomeUnknown
)

// desiredStateConfirmation は、更新後の観測が期待状態と一致するかの確認結果。
type desiredStateConfirmation int

const (
	desiredStateNotChecked desiredStateConfirmation = iota
	desiredStateMatches
	desiredStateMismatch
	desiredStateUnconfirmed
	desiredStateRepositoryChanged
)

// unmanagedChange は、管理しない設定について更新の前後で観測した変化。
type unmanagedChange struct {
	Setting       string
	Before, After bool
}

// result は、変更結果の表示区分を導く。
func (r changeReport) result() changeResult {
	switch {
	case r.update == updateNotStarted:
		return notAttempted
	case r.update == updateSucceeded && r.confirmation == desiredStateMatches && len(r.unmanagedChanges) == 0:
		return completed
	default:
		return failed
	}
}

// err は、完了以外の結果を error にする。
// cause の文言は対象名と失敗した操作を含むため、結果区分だけを前置する。cause がなければ対象名と理由を書く。
func (r changeReport) err(target repository.Reference, change changeplan.SettingPlan) error {
	result := r.result()
	if result == completed {
		return nil
	}
	if r.cause != nil {
		return fmt.Errorf("change %s: %w", result, r.cause)
	}
	return fmt.Errorf("%s: change %s: %s", target, result, strings.Join(r.details(change), "; "))
}

// details は、完了以外の結果の理由を表示の 1 行ずつで返す。
func (r changeReport) details(change changeplan.SettingPlan) []string {
	var lines []string
	if r.precondition != preconditionHolds {
		lines = append(lines, "Precondition: "+r.precondition.reason(change.Setting))
	}
	if r.update != updateNotStarted {
		lines = append(lines, "Update request: "+r.update.text(), "Desired state: "+r.confirmation.text(r.update))
	}
	for _, c := range r.unmanagedChanges {
		lines = append(lines, fmt.Sprintf("Unmanaged settings: %s changed %t -> %t (cause not determined)", c.Setting, c.Before, c.After))
	}
	return lines
}

func (p preconditionCheck) reason(setting string) string {
	switch p {
	case preconditionIdentityMismatch:
		return "repository ID differs from the approved repository"
	case preconditionCurrentValueChanged:
		return setting + " changed since approval"
	default:
		return "not confirmed (read failed)"
	}
}

func (u updateOutcome) text() string {
	switch u {
	case updateSucceeded:
		return "succeeded"
	case updateFailed:
		return "rejected"
	default:
		return "unknown (response unavailable)"
	}
}

// text は、更新操作の結果に応じた期待状態の表示を返す。
// 成否不明のときの一致は、更新操作の成否が判明したことにならないため、その旨を添える。
func (d desiredStateConfirmation) text(update updateOutcome) string {
	switch d {
	case desiredStateMatches:
		if update == updateOutcomeUnknown {
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

// writeChangeReport は変更結果を CLI 仕様の apply の形式で書く。
func writeChangeReport(w io.Writer, target repository.Reference, change changeplan.SettingPlan, report changeReport) error {
	result := report.result()
	var b strings.Builder
	fmt.Fprintf(&b, "  %s: %s\n", target, result)
	switch result {
	case completed:
		b.WriteString("\nApply complete.\nRepositories updated: 1\n")
	case failed:
		writeDetails(&b, report.details(change))
		b.WriteString("\n" + failureNote(report.update) + "\n")
	default:
		writeDetails(&b, report.details(change))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeDetails(b *strings.Builder, lines []string) {
	for _, line := range lines {
		b.WriteString("    " + line + "\n")
	}
}

func failureNote(update updateOutcome) string {
	if update == updateOutcomeUnknown {
		return "Note: No automatic retry was performed."
	}
	return "Note: Some changes may already have been applied. No automatic rollback was performed."
}
