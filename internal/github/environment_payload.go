package github

import (
	"encoding/json"
	"fmt"
	"slices"
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
// 必須の項目が一つでも欠ければ、既定値で補わず契約外の応答とする。
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
	protection, uninterpreted, err := decodeEnvironmentProtection(*p.ProtectionRules, *p.CanAdminsBypass)
	if err != nil {
		return Environment{}, err
	}
	return Environment{
		ID:                           EnvironmentID(*p.ID),
		Name:                         *p.Name,
		DeploymentBranchPolicyMode:   mode,
		Protection:                   protection,
		UninterpretedProtectionRules: uninterpreted,
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

// decodeEnvironmentProtection は保護ルールの一覧を、保護設定と、解釈しなかった保護ルールに変換する。
// wait_timer と required_reviewers は、同じ種類が二つあれば契約外の応答とする。
// branch_policy はデプロイ元制限として deployment_branch_policy から読むため、読み飛ばす。
// それ以外の種類も保護設定には読まず、type だけを解釈しなかったものとして応答の順に、同じ type は一つだけ返す。
// GitHub が保護ルールの種類を足しても観測を失敗させず、削除がその保護ルールも消すことを示せないと判定できるようにするため。
// wait_timer の保護ルールがなければ待機時間を 0 とする。GitHub は 0 を保存せず、保護ルールをなくして表すため。
// required_reviewers の保護ルールがなければ、担当者の集合を空集合の観測値、自己承認の禁止を偽とする。
// GitHub は担当者をすべて外すと保護ルールごと値をなくし、担当者だけを設定し直すと自己承認の禁止を偽で作るため。
// 担当者のいない保護ルールは、自己承認の禁止を偽とみなさず保護ルールが持つ値にする。GitHub はこの保護ルールに真を残し、担当者を戻すと真のまま戻すため。
func decodeEnvironmentProtection(rules []json.RawMessage, canAdminsBypass bool) (EnvironmentProtection, []UninterpretedPart, error) {
	protection := EnvironmentProtection{ReviewerSet: Observed(setting.ReviewerSetOfRule(nil)), CanAdminsBypass: canAdminsBypass}
	var uninterpreted []UninterpretedPart
	var hasWaitTimer, hasRequiredReviewers bool
	for _, raw := range rules {
		var rule protectionRulePayload
		if err := json.Unmarshal(raw, &rule); err != nil {
			return EnvironmentProtection{}, nil, fmt.Errorf("%w: protection_rules: %w", ErrUnexpectedResponse, err)
		}
		if rule.Type == nil {
			return EnvironmentProtection{}, nil, fmt.Errorf("%w: missing fields: protection_rules.type", ErrUnexpectedResponse)
		}
		switch *rule.Type {
		case "wait_timer":
			if rule.WaitTimer == nil {
				return EnvironmentProtection{}, nil, fmt.Errorf("%w: missing fields: protection_rules.wait_timer", ErrUnexpectedResponse)
			}
			if hasWaitTimer {
				return EnvironmentProtection{}, nil, fmt.Errorf("%w: duplicate protection rule: wait_timer", ErrUnexpectedResponse)
			}
			hasWaitTimer = true
			protection.WaitTimer = *rule.WaitTimer
		case "required_reviewers":
			reviewerSet, preventSelfReview, err := decodeRequiredReviewers(rule)
			if err != nil {
				return EnvironmentProtection{}, nil, err
			}
			if hasRequiredReviewers {
				return EnvironmentProtection{}, nil, fmt.Errorf("%w: duplicate protection rule: required_reviewers", ErrUnexpectedResponse)
			}
			hasRequiredReviewers = true
			protection.ReviewerSet = reviewerSet
			protection.PreventSelfReview = preventSelfReview
		case "branch_policy":
		default:
			part := UninterpretedPart{Kind: UninterpretedProtectionRuleType, Name: *rule.Type}
			if !slices.Contains(uninterpreted, part) {
				uninterpreted = append(uninterpreted, part)
			}
		}
	}
	return protection, uninterpreted, nil
}

// decodeRequiredReviewers は required_reviewers の保護ルールを、担当者の集合の観測結果と自己承認の禁止の値に変換する。
// 担当者の type は GitHub が値を足し得る列挙として読み、すべての担当者を取得契約と照らしたうえで、知らない種類の担当者が一人でもいれば担当者の集合を観測不能にする。
// reviewer の形は担当者の種類ごとに違い得るため、知らない種類の担当者の取得契約は type だけとする。
// 知らない種類の担当者を除いた残りを観測値にすると、照合と変更前提がその担当者を見落とすため。
// 自己承認の禁止は同じ保護ルールの boolean で、担当者の種類によらず観測値にする。
func decodeRequiredReviewers(rule protectionRulePayload) (SettingObservation[setting.ReviewerSet], bool, error) {
	var missing []string
	if rule.Reviewers == nil {
		missing = append(missing, "protection_rules.reviewers")
	}
	if rule.PreventSelfReview == nil {
		missing = append(missing, "protection_rules.prevent_self_review")
	}
	if len(missing) > 0 {
		return SettingObservation[setting.ReviewerSet]{}, false, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	reviewers := make([]setting.Reviewer, len(*rule.Reviewers))
	var logins map[setting.Reviewer]string
	var unknownType bool
	for i, r := range *rule.Reviewers {
		if r == nil {
			return SettingObservation[setting.ReviewerSet]{}, false, fmt.Errorf("%w: unsupported value: protection_rules.reviewers", ErrUnexpectedResponse)
		}
		var missingReviewer []string
		var reviewerType setting.ReviewerType
		if r.Type == nil {
			missingReviewer = append(missingReviewer, "protection_rules.reviewers.type")
		} else {
			known, ok := setting.ParseReviewerType(*r.Type)
			if !ok {
				unknownType = true
				continue
			}
			reviewerType = known
		}
		if r.Reviewer == nil || r.Reviewer.ID == nil {
			missingReviewer = append(missingReviewer, "protection_rules.reviewers.reviewer.id")
		}
		if len(missingReviewer) > 0 {
			return SettingObservation[setting.ReviewerSet]{}, false, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missingReviewer, ", "))
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
	if unknownType {
		return Unavailable[setting.ReviewerSet](UnavailableUnknownValue), *rule.PreventSelfReview, nil
	}
	return Observed(setting.ReviewerSetOfRule(&setting.RequiredReviewers{Reviewers: reviewers, Logins: logins})), *rule.PreventSelfReview, nil
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

// decodeDeploymentPatterns は個別パターンの一覧の全要素を、一覧の並びのまま個別パターンの観測結果に変換する。
// 同じ種類と名前の個別パターンが二つ以上あれば失敗する。ID の対応が黙って上書きされ、取り除く要求の宛先を誤らせるため。
// すべての要素を取得契約と照らしたうえで、知らない種類の個別パターンが一つでもあれば観測不能にする。
// 知らない種類の個別パターンを除いた残りを観測値にすると、照合と変更前提がその個別パターンを見落とすため。
func decodeDeploymentPatterns(items []json.RawMessage) (SettingObservation[DeploymentPatterns], error) {
	patterns := make([]setting.DeploymentPattern, 0, len(items))
	ids := make(map[setting.DeploymentPattern]DeploymentPatternID, len(items))
	var unknownType bool
	for _, item := range items {
		pattern, id, known, err := decodeDeploymentPattern(item)
		if err != nil {
			return SettingObservation[DeploymentPatterns]{}, err
		}
		if !known {
			unknownType = true
			continue
		}
		if _, duplicate := ids[pattern]; duplicate {
			return SettingObservation[DeploymentPatterns]{}, fmt.Errorf("%w: duplicate deployment pattern", ErrUnexpectedResponse)
		}
		patterns = append(patterns, pattern)
		ids[pattern] = id
	}
	if unknownType {
		return Unavailable[DeploymentPatterns](UnavailableUnknownValue), nil
	}
	return Observed(DeploymentPatterns{Existing: patterns, IDs: ids}), nil
}

// deploymentPatternPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type deploymentPatternPayload struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

// decodeDeploymentPattern は個別パターンの一覧の要素を、値と GitHub の ID と、type が reco の知る値かに変換する。
// name は空文字列でもそのまま値にする。type は GitHub が値を足し得る列挙として読み、知らない値なら失敗にせず、値をゼロ値にして偽を返す。
func decodeDeploymentPattern(data json.RawMessage) (setting.DeploymentPattern, DeploymentPatternID, bool, error) {
	var p deploymentPatternPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.DeploymentPattern{}, 0, false, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
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
		return setting.DeploymentPattern{}, 0, false, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	patternType, ok := setting.ParseDeploymentPatternType(*p.Type)
	if !ok {
		return setting.DeploymentPattern{}, DeploymentPatternID(*p.ID), false, nil
	}
	return setting.DeploymentPattern{Type: patternType, Name: *p.Name}, DeploymentPatternID(*p.ID), true, nil
}
