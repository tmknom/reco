package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Variables は、変数の一覧の全ページを読めた観測で確認できた、属する先に存在するすべての Variable である。
type Variables struct {
	// Existing は GitHub が返した順のまま持つ。
	Existing []Variable
}

// Variable は、存在すると確かめた一つの Variable の観測結果である。
type Variable struct {
	// Name は GitHub が返した綴りのまま持つ。
	Name string
	// Value は GitHub が返したとおりに持つ。
	Value string
}

const (
	// variablesPerPage は変数の一覧の1ページで求める件数。GitHub が受け付ける上限で、maxVariablesPageBytes の導出の前提である。
	variablesPerPage = 30
	// maxVariablesPageBytes は変数の一覧の1ページの応答本文を読む上限。導出は Actions の設計の「観測」にある。
	maxVariablesPageBytes = 9 << 20
	// maxVariablesPages は変数の一覧で読むページ数の上限。導出は Actions の設計の「観測」にある。
	maxVariablesPages = 17
)

// Lookup は、宣言の名前 name の Variable の観測結果を、一覧の全体と突き合わせて返す。
// 一覧は全ページを読めているため、名前のない Variable は存在しないと確かめたことになる。
func (v Variables) Lookup(name setting.VariableName) SettingObservation[setting.Value] {
	for _, variable := range v.Existing {
		if name.Matches(variable.Name) {
			return Observed(setting.VariableValue(variable.Value))
		}
	}
	return Observed(setting.AbsentVariable())
}

// observeRepositoryVariables は Repository Variables の一覧の全ページを読む。
func (c *Client) observeRepositoryVariables(ctx context.Context, reference repository.Reference) (Variables, error) {
	items, err := c.listAll(ctx, c.repositoryVariablesURL(reference), listPaging{itemsField: "variables", perPage: variablesPerPage, maxPageBytes: maxVariablesPageBytes, maxPages: maxVariablesPages})
	if err != nil {
		return Variables{}, err
	}
	existing := make([]Variable, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		existing[i], err = decodeVariable(item)
		if err != nil {
			return Variables{}, err
		}
		// GitHub は変数の名前を大文字小文字を区別せずに比べるため、二つ並ぶとどちらの値を観測値とするかが決まらない。
		key := setting.FoldVariableName(existing[i].Name)
		if _, duplicate := seen[key]; duplicate {
			return Variables{}, fmt.Errorf("%w: duplicate variable name %q", ErrUnexpectedResponse, existing[i].Name)
		}
		seen[key] = struct{}{}
	}
	return Variables{Existing: existing}, nil
}

// variablePayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type variablePayload struct {
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

// decodeVariable は変数の一覧の要素を観測値に変換する。
// 値の欠落を空文字列で補わない。空文字列の値と、値が読めないことを取り違えないため。応答の値は文言に入れない。
func decodeVariable(data json.RawMessage) (Variable, error) {
	var p variablePayload
	if err := json.Unmarshal(data, &p); err != nil {
		return Variable{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.Name == nil {
		missing = append(missing, "name")
	}
	if p.Value == nil {
		missing = append(missing, "value")
	}
	if len(missing) > 0 {
		return Variable{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	if *p.Name == "" {
		return Variable{}, fmt.Errorf("%w: unsupported value: name", ErrUnexpectedResponse)
	}
	return Variable{Name: *p.Name, Value: *p.Value}, nil
}
