package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/setting"
)

// environmentPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
// deployment_branch_policy は、null が all を表し、欠落と区別するため json.RawMessage で受ける。
type environmentPayload struct {
	ID                     *int64             `json:"id"`
	Name                   *string            `json:"name"`
	CanAdminsBypass        *bool              `json:"can_admins_bypass"`
	ProtectionRules        *[]json.RawMessage `json:"protection_rules"`
	DeploymentBranchPolicy json.RawMessage    `json:"deployment_branch_policy"`
}

// decodeEnvironment は Environment の一覧の要素を観測値に変換する。個別パターンは別の要求で読むため、持たない状態で返す。
// 必須の項目が一つでも欠ければ、既定値で補わず契約外の応答とする。定義外の値は応答本文の一部のため、文言に入れない。
func decodeEnvironment(data json.RawMessage) (Environment, error) {
	var p environmentPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return Environment{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.ID == nil {
		missing = append(missing, "id")
	}
	if p.Name == nil {
		missing = append(missing, "name")
	}
	if p.CanAdminsBypass == nil {
		missing = append(missing, "can_admins_bypass")
	}
	if p.ProtectionRules == nil {
		missing = append(missing, "protection_rules")
	}
	if len(p.DeploymentBranchPolicy) == 0 {
		missing = append(missing, "deployment_branch_policy")
	}
	if len(missing) > 0 {
		return Environment{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	if *p.Name == "" {
		return Environment{}, fmt.Errorf("%w: unsupported value: name", ErrUnexpectedResponse)
	}
	mode, err := decodeDeploymentBranchPolicyMode(p.DeploymentBranchPolicy)
	if err != nil {
		return Environment{}, err
	}
	protection, err := decodeEnvironmentProtection(*p.ProtectionRules, *p.CanAdminsBypass)
	if err != nil {
		return Environment{}, err
	}
	return Environment{
		ID:                         EnvironmentID(*p.ID),
		Name:                       *p.Name,
		DeploymentBranchPolicyMode: mode,
		Protection:                 protection,
	}, nil
}

// deploymentBranchPolicyPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type deploymentBranchPolicyPayload struct {
	ProtectedBranches    *bool `json:"protected_branches"`
	CustomBranchPolicies *bool `json:"custom_branch_policies"`
}

// decodeDeploymentBranchPolicyMode は deployment_branch_policy の値から、デプロイ元制限の方式を決める。
// 両方が同じ値の組は GitHub が更新で受け付けないため、契約外の応答とする。
func decodeDeploymentBranchPolicyMode(raw json.RawMessage) (setting.DeploymentBranchPolicyMode, error) {
	if string(raw) == "null" {
		return setting.DeploymentBranchPolicyAll, nil
	}
	var p deploymentBranchPolicyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("%w: deployment_branch_policy: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.ProtectedBranches == nil {
		missing = append(missing, "deployment_branch_policy.protected_branches")
	}
	if p.CustomBranchPolicies == nil {
		missing = append(missing, "deployment_branch_policy.custom_branch_policies")
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	switch {
	case *p.ProtectedBranches && !*p.CustomBranchPolicies:
		return setting.DeploymentBranchPolicyProtectedBranches, nil
	case !*p.ProtectedBranches && *p.CustomBranchPolicies:
		return setting.DeploymentBranchPolicySelected, nil
	}
	return "", fmt.Errorf("%w: unsupported value: deployment_branch_policy", ErrUnexpectedResponse)
}

// protectionRulePayload は欠落と null を検出するため、各フィールドをポインタで受ける。
// 保護ルールの種類ごとに持つフィールドが違うため、すべてを一つの型で受ける。
type protectionRulePayload struct {
	Type              *string             `json:"type"`
	WaitTimer         *int                `json:"wait_timer"`
	PreventSelfReview *bool               `json:"prevent_self_review"`
	Reviewers         *[]*reviewerPayload `json:"reviewers"`
}

type reviewerPayload struct {
	Type     *string `json:"type"`
	Reviewer *struct {
		ID    *int64  `json:"id"`
		Login *string `json:"login"`
	} `json:"reviewer"`
}

// decodeEnvironmentProtection は保護ルールの一覧を保護設定に変換する。
// wait_timer と required_reviewers は、同じ種類が二つあれば契約外の応答とする。
// branch_policy はデプロイ元制限として deployment_branch_policy から読むため、読み飛ばす。
// それ以外の種類も読み飛ばす。GitHub が保護ルールの種類を足しても、reco が値を持たない種類のために観測を失敗させないため。
func decodeEnvironmentProtection(rules []json.RawMessage, canAdminsBypass bool) (setting.EnvironmentProtection, error) {
	protection := setting.EnvironmentProtection{CanAdminsBypass: canAdminsBypass}
	for _, raw := range rules {
		var rule protectionRulePayload
		if err := json.Unmarshal(raw, &rule); err != nil {
			return setting.EnvironmentProtection{}, fmt.Errorf("%w: protection_rules: %w", ErrUnexpectedResponse, err)
		}
		if rule.Type == nil {
			return setting.EnvironmentProtection{}, fmt.Errorf("%w: missing fields: protection_rules.type", ErrUnexpectedResponse)
		}
		switch *rule.Type {
		case "wait_timer":
			if rule.WaitTimer == nil {
				return setting.EnvironmentProtection{}, fmt.Errorf("%w: missing fields: protection_rules.wait_timer", ErrUnexpectedResponse)
			}
			if protection.WaitTimer != nil {
				return setting.EnvironmentProtection{}, fmt.Errorf("%w: duplicate protection rule: wait_timer", ErrUnexpectedResponse)
			}
			protection.WaitTimer = rule.WaitTimer
		case "required_reviewers":
			required, err := decodeRequiredReviewers(rule)
			if err != nil {
				return setting.EnvironmentProtection{}, err
			}
			if protection.RequiredReviewers != nil {
				return setting.EnvironmentProtection{}, fmt.Errorf("%w: duplicate protection rule: required_reviewers", ErrUnexpectedResponse)
			}
			protection.RequiredReviewers = &required
		}
	}
	return protection, nil
}

// decodeRequiredReviewers は required_reviewers の保護ルールを読む。
func decodeRequiredReviewers(rule protectionRulePayload) (setting.RequiredReviewers, error) {
	var missing []string
	if rule.Reviewers == nil {
		missing = append(missing, "protection_rules.reviewers")
	}
	if rule.PreventSelfReview == nil {
		missing = append(missing, "protection_rules.prevent_self_review")
	}
	if len(missing) > 0 {
		return setting.RequiredReviewers{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	reviewers := make([]setting.Reviewer, len(*rule.Reviewers))
	var logins map[setting.Reviewer]string
	for i, r := range *rule.Reviewers {
		if r == nil {
			return setting.RequiredReviewers{}, fmt.Errorf("%w: unsupported value: protection_rules.reviewers", ErrUnexpectedResponse)
		}
		var missingReviewer []string
		if r.Type == nil {
			missingReviewer = append(missingReviewer, "protection_rules.reviewers.type")
		}
		if r.Reviewer == nil || r.Reviewer.ID == nil {
			missingReviewer = append(missingReviewer, "protection_rules.reviewers.reviewer.id")
		}
		if len(missingReviewer) > 0 {
			return setting.RequiredReviewers{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missingReviewer, ", "))
		}
		reviewerType, ok := setting.ParseReviewerType(*r.Type)
		if !ok {
			return setting.RequiredReviewers{}, fmt.Errorf("%w: unsupported value: protection_rules.reviewers.type", ErrUnexpectedResponse)
		}
		reviewers[i] = setting.Reviewer{Type: reviewerType, ID: *r.Reviewer.ID}
		// login は表示にだけ使うため、欠落や形の不一致は失敗にせず持たない
		if reviewerType == setting.ReviewerUser && r.Reviewer.Login != nil && isLogin(*r.Reviewer.Login) {
			if logins == nil {
				logins = map[setting.Reviewer]string{}
			}
			logins[reviewers[i]] = *r.Reviewer.Login
		}
	}
	return setting.RequiredReviewers{PreventSelfReview: *rule.PreventSelfReview, Reviewers: reviewers, Logins: logins}, nil
}

// isLogin は s が、表示の行にそのまま書ける login の形かを返す。
// API が返した文字列を表示に書くため、改行などで行を偽装できる文字を含むものは受け付けない。
func isLogin(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		isAlphanumeric := r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !isAlphanumeric && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// deploymentPatternPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type deploymentPatternPayload struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

// decodeDeploymentPattern は個別パターンの一覧の要素を、値と GitHub の ID に変換する。
// name は空文字列でもそのまま値にする。定義外の type は応答本文の一部のため、文言に入れない。
func decodeDeploymentPattern(data json.RawMessage) (setting.DeploymentPattern, DeploymentPatternID, error) {
	var p deploymentPatternPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.DeploymentPattern{}, 0, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.ID == nil {
		missing = append(missing, "id")
	}
	if p.Name == nil {
		missing = append(missing, "name")
	}
	if p.Type == nil {
		missing = append(missing, "type")
	}
	if len(missing) > 0 {
		return setting.DeploymentPattern{}, 0, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	patternType, ok := setting.ParseDeploymentPatternType(*p.Type)
	if !ok {
		return setting.DeploymentPattern{}, 0, fmt.Errorf("%w: unsupported value: type", ErrUnexpectedResponse)
	}
	return setting.DeploymentPattern{Type: patternType, Name: *p.Name}, DeploymentPatternID(*p.ID), nil
}
