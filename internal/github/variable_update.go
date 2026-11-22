package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// CreateRepositoryVariable は、reference のリポジトリに、名前が name で値が value の Repository Variable を作る POST を一度だけ送る。再送はしない。
// 正常応答の 201 の本文は変数を含まないため、解釈しない。
// value は error に含めない。
func (c *Client) CreateRepositoryVariable(ctx context.Context, reference repository.Reference, name setting.VariableName, value string) (UpdateOutcome, error) {
	return c.createVariable(ctx, fmt.Sprintf("create repository variable %q", name.String()), reference, c.repositoryVariablesURL(reference), name, value)
}

// UpdateRepositoryVariableValue は、reference のリポジトリの name の Repository Variable の値を value にする PATCH を一度だけ送る。再送はしない。
// 本文は value だけで、name を送らない。reco は変数の名前を変えないため。
// value は error に含めない。
func (c *Client) UpdateRepositoryVariableValue(ctx context.Context, reference repository.Reference, name setting.VariableName, value string) (UpdateOutcome, error) {
	return c.updateVariableValue(ctx, fmt.Sprintf("update repository variable %q", name.String()), reference, c.repositoryVariableURL(reference, name.String()), value)
}

// CreateEnvironmentVariable は、reference のリポジトリの environment の Environment に、名前が name で値が value の Environment Variable を作る POST を一度だけ送る。
// 本文と応答の扱いは CreateRepositoryVariable と同じ。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) CreateEnvironmentVariable(ctx context.Context, reference repository.Reference, environment string, name setting.VariableName, value string) (UpdateOutcome, error) {
	u, ok := c.environmentVariablesURL(reference, environment)
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	return c.createVariable(ctx, fmt.Sprintf("create environment variable %q in environment %q", name.String(), environment), reference, u, name, value)
}

// UpdateEnvironmentVariableValue は、reference のリポジトリの environment の Environment の name の Environment Variable の値を value にする PATCH を一度だけ送る。
// 本文と応答の扱いは UpdateRepositoryVariableValue と同じ。
// 呼び出し元は宛先にできる名前だけを渡すため、URL を組めないときは不変条件の破れとして panic する。
func (c *Client) UpdateEnvironmentVariableValue(ctx context.Context, reference repository.Reference, environment string, name setting.VariableName, value string) (UpdateOutcome, error) {
	u, ok := c.environmentVariableURL(reference, environment, name.String())
	if !ok {
		panic(fmt.Sprintf("github: environment name %q cannot be addressed", environment))
	}
	return c.updateVariableValue(ctx, fmt.Sprintf("update environment variable %q in environment %q", name.String(), environment), reference, u, value)
}

// createVariable は、変数の一覧の u へ、名前が name で値が value の変数を作る POST を送る。Repository Variable と Environment Variable で、本文と応答の扱いは同じ。
func (c *Client) createVariable(ctx context.Context, operation string, reference repository.Reference, u *url.URL, name setting.VariableName, value string) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPost, u, encodeVariableCreation(name, value), http.StatusCreated)
	return describeUpdateOutcome(operation, reference, status, err)
}

// updateVariableValue は、変数の u へ、値を value にする PATCH を送る。Repository Variable と Environment Variable で、本文と応答の扱いは同じ。
func (c *Client) updateVariableValue(ctx context.Context, operation string, reference repository.Reference, u *url.URL, value string) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPatch, u, encodeVariableValue(value), http.StatusNoContent)
	return describeUpdateOutcome(operation, reference, status, err)
}

// encodeVariableCreation は、変数を作る POST の本文にする。値は任意の文字列のため、JSON の規則でエスケープする。
func encodeVariableCreation(name setting.VariableName, value string) []byte {
	return mustMarshalJSON(struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{name.String(), value})
}

// encodeVariableValue は、変数の値だけを送る PATCH の本文にする。
func encodeVariableValue(value string) []byte {
	return mustMarshalJSON(struct {
		Value string `json:"value"`
	}{value})
}
