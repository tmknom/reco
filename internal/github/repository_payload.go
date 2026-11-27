package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// repositoryPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type repositoryPayload struct {
	ID                       *int64  `json:"id"`
	AllowAutoMerge           *bool   `json:"allow_auto_merge"`
	DeleteBranchOnMerge      *bool   `json:"delete_branch_on_merge"`
	AllowSquashMerge         *bool   `json:"allow_squash_merge"`
	AllowMergeCommit         *bool   `json:"allow_merge_commit"`
	AllowRebaseMerge         *bool   `json:"allow_rebase_merge"`
	AllowUpdateBranch        *bool   `json:"allow_update_branch"`
	SquashMergeCommitTitle   *string `json:"squash_merge_commit_title"`
	SquashMergeCommitMessage *string `json:"squash_merge_commit_message"`
	MergeCommitTitle         *string `json:"merge_commit_title"`
	MergeCommitMessage       *string `json:"merge_commit_message"`
	HasIssues                *bool   `json:"has_issues"`
	HasProjects              *bool   `json:"has_projects"`
	HasWiki                  *bool   `json:"has_wiki"`
	// WebCommitSignoffRequired は、省略を許される欠落と、許されない null を区別するため、未解釈のまま受ける。
	WebCommitSignoffRequired json.RawMessage `json:"web_commit_signoff_required"`
	Private                  *bool           `json:"private"`
	// Permissions は Dependabot の領域を読まない観測では使わないため、取得契約に含めず、欠落と null を許す。
	Permissions *repositoryPermissionsPayload `json:"permissions"`
	// SecurityAndAnalysis は Secret scanning の領域を読む観測だけが解釈する。ほかの設定の領域の観測が、この領域の応答の形で失敗しないよう、型を合わせず未解釈のまま受ける。
	SecurityAndAnalysis json.RawMessage `json:"security_and_analysis"`
}

type repositoryPermissionsPayload struct {
	Admin *bool `json:"admin"`
}

// repositoryAttributes は、リポジトリの応答のうち、decodeRepository が観測結果に変換しない値である。
type repositoryAttributes struct {
	private             bool
	admin               bool
	securityAndAnalysis json.RawMessage
}

// decodeRepository はリポジトリの応答本文を、観測結果と、観測結果に変換しない値に変換する。
// 欠落の理由を応答から確認できないため、欠けた値を既定値で補わず、契約外の応答として返す。
func decodeRepository(data []byte) (ObservationResult, repositoryAttributes, error) {
	var p repositoryPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return ObservationResult{}, repositoryAttributes{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.ID == nil {
		missing = append(missing, "id")
	}
	if p.AllowAutoMerge == nil {
		missing = append(missing, "allow_auto_merge")
	}
	if p.DeleteBranchOnMerge == nil {
		missing = append(missing, "delete_branch_on_merge")
	}
	if p.AllowSquashMerge == nil {
		missing = append(missing, "allow_squash_merge")
	}
	if p.AllowMergeCommit == nil {
		missing = append(missing, "allow_merge_commit")
	}
	if p.AllowRebaseMerge == nil {
		missing = append(missing, "allow_rebase_merge")
	}
	if p.AllowUpdateBranch == nil {
		missing = append(missing, "allow_update_branch")
	}
	if p.SquashMergeCommitTitle == nil {
		missing = append(missing, "squash_merge_commit_title")
	}
	if p.SquashMergeCommitMessage == nil {
		missing = append(missing, "squash_merge_commit_message")
	}
	if p.MergeCommitTitle == nil {
		missing = append(missing, "merge_commit_title")
	}
	if p.MergeCommitMessage == nil {
		missing = append(missing, "merge_commit_message")
	}
	if p.HasIssues == nil {
		missing = append(missing, "has_issues")
	}
	if p.HasProjects == nil {
		missing = append(missing, "has_projects")
	}
	if p.HasWiki == nil {
		missing = append(missing, "has_wiki")
	}
	if p.Private == nil {
		missing = append(missing, "private")
	}
	if len(missing) > 0 {
		return ObservationResult{}, repositoryAttributes{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	webCommitSignoffRequired, err := decodeWebCommitSignoffRequired(p.WebCommitSignoffRequired)
	if err != nil {
		return ObservationResult{}, repositoryAttributes{}, err
	}
	return ObservationResult{
		RepositoryID: repository.ID(*p.ID),
		General: General{
			AllowAutoMerge:           *p.AllowAutoMerge,
			DeleteBranchOnMerge:      *p.DeleteBranchOnMerge,
			AllowSquashMerge:         *p.AllowSquashMerge,
			AllowMergeCommit:         *p.AllowMergeCommit,
			AllowRebaseMerge:         *p.AllowRebaseMerge,
			AllowUpdateBranch:        *p.AllowUpdateBranch,
			SquashMergeCommitDefault: observeSquashMergeCommitDefault(*p.SquashMergeCommitTitle, *p.SquashMergeCommitMessage),
			MergeCommitDefault:       observeMergeCommitDefault(*p.MergeCommitTitle, *p.MergeCommitMessage),
			HasIssues:                *p.HasIssues,
			HasProjects:              *p.HasProjects,
			HasWiki:                  *p.HasWiki,
			WebCommitSignoffRequired: webCommitSignoffRequired,
		},
	}, repositoryAttributes{
		private:             *p.Private,
		admin:               p.Permissions != nil && p.Permissions.Admin != nil && *p.Permissions.Admin,
		securityAndAnalysis: p.SecurityAndAnalysis,
	}, nil
}

// decodeWebCommitSignoffRequired は、Web コミットのサインオフ必須の未解釈の値を観測結果にする。
// 欠落は公式の仕様の範囲のため観測不能にし、null と型の合わない値は取得契約に適合しない応答にする。
func decodeWebCommitSignoffRequired(raw json.RawMessage) (SettingObservation[bool], error) {
	if raw == nil {
		return Unavailable[bool](UnavailableNotReturned), nil
	}
	var required *bool
	if err := json.Unmarshal(raw, &required); err != nil {
		return SettingObservation[bool]{}, fmt.Errorf("%w: web_commit_signoff_required: %w", ErrUnexpectedResponse, err)
	}
	if required == nil {
		return SettingObservation[bool]{}, fmt.Errorf("%w: missing fields: web_commit_signoff_required", ErrUnexpectedResponse)
	}
	return Observed(*required), nil
}

// observeSquashMergeCommitDefault は、squash merge のタイトルと本文の組を観測結果にする。
// GitHub は API バージョンを変えずに列挙の値を足すため、知らない値は観測操作の失敗にせず、組だけを観測不能にする。
func observeSquashMergeCommitDefault(title, message string) SettingObservation[setting.SquashMergeCommitDefault] {
	t, titleOK := setting.ParseSquashMergeCommitTitle(title)
	m, messageOK := setting.ParseSquashMergeCommitMessage(message)
	if !titleOK || !messageOK {
		return Unavailable[setting.SquashMergeCommitDefault](UnavailableUnknownValue)
	}
	return Observed(setting.SquashMergeCommitDefault{Title: t, Message: m})
}

// observeMergeCommitDefault は、merge commit のタイトルと本文の組を観測結果にする。知らない値の扱いは observeSquashMergeCommitDefault と同じ。
func observeMergeCommitDefault(title, message string) SettingObservation[setting.MergeCommitDefault] {
	t, titleOK := setting.ParseMergeCommitTitle(title)
	m, messageOK := setting.ParseMergeCommitMessage(message)
	if !titleOK || !messageOK {
		return Unavailable[setting.MergeCommitDefault](UnavailableUnknownValue)
	}
	return Observed(setting.MergeCommitDefault{Title: t, Message: m})
}
