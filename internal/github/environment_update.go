package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// CreateEnvironment は、reference のリポジトリに name の Environment を作る PUT を、本文なしで一度だけ送る。再送はしない。
// 同じ名前の Environment が存在すれば、この要求は何も変えずに成功を応答する。
func (c *Client) CreateEnvironment(ctx context.Context, reference repository.Reference, name string) (UpdateOutcome, error) {
	return c.putEnvironment(ctx, "create environment", reference, name, nil)
}

// CreateEnvironmentWithDeploymentBranchPolicy は、reference のリポジトリに、デプロイ元制限の方式が mode の name の Environment を作る PUT を一度だけ送る。再送はしない。
// 要求は UpdateEnvironmentDeploymentBranchPolicy と同じだが、失敗を作成として示すために分ける。利用者が承認したのは作成で、方式の変更ではないため。
// 本文の組み方と、未定義の方式での panic は UpdateEnvironmentDeploymentBranchPolicy と同じである。
func (c *Client) CreateEnvironmentWithDeploymentBranchPolicy(ctx context.Context, reference repository.Reference, name string, mode setting.DeploymentBranchPolicyMode) (UpdateOutcome, error) {
	return c.putEnvironment(ctx, "create environment with deployment branch policy", reference, name, encodeDeploymentBranchPolicyMode(mode))
}

// UpdateEnvironmentDeploymentBranchPolicy は、reference のリポジトリの name の Environment のデプロイ元制限の方式を mode にする PUT を一度だけ送る。再送はしない。
// name の Environment が存在しなければ、この要求は方式が mode の Environment を作る。作成として送るときは CreateEnvironmentWithDeploymentBranchPolicy を使う。
// 本文は deployment_branch_policy だけで、保護設定を併送しない。
// selected は方式だけを送り、個別パターンは登録しない。登録は AddDeploymentPattern で一つずつ送る。
// 未定義の方式は、本文を組めない不変条件の破れとして panic する。
func (c *Client) UpdateEnvironmentDeploymentBranchPolicy(ctx context.Context, reference repository.Reference, name string, mode setting.DeploymentBranchPolicyMode) (UpdateOutcome, error) {
	return c.putEnvironment(ctx, "set deployment branch policy of environment", reference, name, encodeDeploymentBranchPolicyMode(mode))
}

// UpdateEnvironmentRequiredReviewers は、reference のリポジトリの name の Environment の担当者を reviewers にする PUT を一度だけ送る。再送はしない。
// 本文は reviewers だけで、ほかの保護設定とデプロイ元制限を併送しない。GitHub は本文にない設定を保つ。
// 空の集合は、担当者をすべて外す。担当者のいない保護ルールは宣言できない値で、空の集合として送ると保護ルールを外す要求になるため、不変条件の破れとして panic する。
func (c *Client) UpdateEnvironmentRequiredReviewers(ctx context.Context, reference repository.Reference, name string, reviewers setting.ReviewerSet) (UpdateOutcome, error) {
	if reviewers.IsRuleWithoutReviewers() {
		panic(fmt.Sprintf("github: cannot send a rule without reviewers as the reviewers of environment %q", name))
	}
	return c.putEnvironment(ctx, "set required reviewers of environment", reference, name, encodeRequiredReviewers(reviewers))
}

// AddDeploymentPattern は、reference のリポジトリの environment の Environment に、個別パターン pattern を加える POST を一度だけ送る。再送はしない。
// 応答の扱いはアーキテクチャ設計の HTTP 通信とリダイレクトの信頼境界と、Environments の設計の更新方式に従う。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) AddDeploymentPattern(ctx context.Context, reference repository.Reference, environment string, pattern setting.DeploymentPattern) (UpdateOutcome, error) {
	u, ok := c.deploymentBranchPoliciesURL(reference, environment)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	status, data, err := c.send(ctx, http.MethodPost, u, encodeDeploymentPattern(pattern), http.StatusOK)
	if err == nil {
		_, _, err = decodeDeploymentPattern(data)
	}
	if status == http.StatusSeeOther {
		// classifyFailure のリポジトリの名前の変更の案内は、この応答の原因と異なり、利用者を誤らせる
		err = fmt.Errorf("%w (HTTP %d): GitHub returns this when the pattern already exists", ErrRedirect, status)
	}
	return describeUpdateOutcome(fmt.Sprintf("add deployment pattern %s %q to environment %q", pattern.Type, pattern.Name, environment), reference, status, err)
}

// RemoveDeploymentPattern は、reference のリポジトリの environment の Environment から、ID が id の個別パターン pattern を取り除く DELETE を、本文なしで一度だけ送る。再送はしない。
// pattern は error で取り除く要素を示すためだけに使う。ID だけでは、どの種類と名前の要素かが分からないため。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) RemoveDeploymentPattern(ctx context.Context, reference repository.Reference, environment string, pattern setting.DeploymentPattern, id DeploymentPatternID) (UpdateOutcome, error) {
	u, ok := c.deploymentBranchPolicyURL(reference, environment, id)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	status, _, err := c.send(ctx, http.MethodDelete, u, nil, http.StatusNoContent)
	return describeUpdateOutcome(fmt.Sprintf("remove deployment pattern %s %q (id %d) from environment %q", pattern.Type, pattern.Name, id, environment), reference, status, err)
}

// putEnvironment は name の Environment へ body を本文とする PUT を送る。body が nil なら本文を送らない。
// 成功応答の本文は、Environment の取得の契約に適合するかを確かめるためだけに解釈する。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) putEnvironment(ctx context.Context, operation string, reference repository.Reference, name string, body []byte) (UpdateOutcome, error) {
	u, ok := c.environmentURL(reference, name)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", name))
	}
	status, data, err := c.send(ctx, http.MethodPut, u, body, http.StatusOK)
	if err == nil {
		_, err = decodeEnvironment(data)
	}
	return describeUpdateOutcome(fmt.Sprintf("%s %q", operation, name), reference, status, err)
}

// encodeDeploymentBranchPolicyMode は mode を Environment の PUT の本文にする。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func encodeDeploymentBranchPolicyMode(mode setting.DeploymentBranchPolicyMode) []byte {
	switch mode {
	case setting.DeploymentBranchPolicyAll:
		return []byte(`{"deployment_branch_policy":null}`)
	case setting.DeploymentBranchPolicyProtectedBranches:
		return []byte(`{"deployment_branch_policy":{"protected_branches":true,"custom_branch_policies":false}}`)
	case setting.DeploymentBranchPolicySelected:
		return []byte(`{"deployment_branch_policy":{"protected_branches":false,"custom_branch_policies":true}}`)
	}
	panic(fmt.Sprintf("github: no request body for deployment branch policy %q", mode))
}

// encodeRequiredReviewers は reviewers を Environment の PUT の本文にする。担当者は宣言の順のまま、種類と ID だけを送る。
func encodeRequiredReviewers(reviewers setting.ReviewerSet) []byte {
	type reviewer struct {
		Type string `json:"type"`
		ID   int64  `json:"id"`
	}
	// encoding/json は nil のスライスを null にするため、空の集合を [] として送るよう長さ 0 のスライスにする
	declared := reviewers.Reviewers()
	elements := make([]reviewer, 0, len(declared))
	for _, r := range declared {
		elements = append(elements, reviewer{string(r.Type), r.ID})
	}
	return mustMarshalJSON(struct {
		Reviewers []reviewer `json:"reviewers"`
	}{elements})
}

// encodeDeploymentPattern は pattern を個別パターンを加える POST の本文にする。
// type は省略すると GitHub が branch として作るため、種類によらず必ず送る。名前は任意の文字列のため、JSON の規則でエスケープする。
func encodeDeploymentPattern(pattern setting.DeploymentPattern) []byte {
	return mustMarshalJSON(struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}{pattern.Name, string(pattern.Type)})
}
