package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/repository"
)

// EnvironmentSecrets は、Environment Secret の一覧の全ページを読めた観測で確認できた、一つの Environment のすべての Environment Secret である。
// reco は Secret の宣言を受け付けず、Environment の削除で一緒に消えるものを示し、承認のあとに変わっていないかを確かめるためだけに読む。
type EnvironmentSecrets struct {
	// Existing は GitHub が返した順のまま持つ。
	Existing []EnvironmentSecret
}

// EnvironmentSecret は、存在すると確かめた一つの Environment Secret の観測結果である。GitHub は値を返さないため、値を持たない。
type EnvironmentSecret struct {
	// Name は GitHub が返した綴りのまま持つ。
	Name string
	// UpdatedAt は GitHub が返した文字列のまま持つ。登録し直されたかを比べるだけで、時刻として解釈しない。
	UpdatedAt string
}

// observeEnvironmentSecrets は、name の Environment の Environment Secret の一覧の全ページを読む。
// 一覧にあった Environment の 404 は、二つの要求のあいだになくなった可能性があるため、Secret がないことにせず error として返す。
// パスに組み込めない名前の Environment は、要求を送らずに失敗する。
// 失敗の文言に Environment の名前を入れるのは、どの Environment で失敗したかを示す対象の識別子で、秘密情報ではないため。%q で制御文字をエスケープする。
func (c *Client) observeEnvironmentSecrets(ctx context.Context, reference repository.Reference, name string) (EnvironmentSecrets, error) {
	u, ok := c.environmentSecretsURL(reference, name)
	if !ok {
		return EnvironmentSecrets{}, fmt.Errorf("secrets of environment %q: %w: the name cannot be sent as a path element", name, ErrUnexpectedResponse)
	}
	items, err := c.listAll(ctx, u, listPaging{itemsField: "secrets", perPage: listPerPage, maxPageBytes: maxResponseBytes, maxPages: maxListPages})
	if err != nil {
		return EnvironmentSecrets{}, fmt.Errorf("secrets of environment %q: %w", name, err)
	}
	existing := make([]EnvironmentSecret, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		existing[i], err = decodeEnvironmentSecret(item)
		if err != nil {
			return EnvironmentSecrets{}, fmt.Errorf("secrets of environment %q: %w", name, err)
		}
		// GitHub は Secret の名前を大文字にして保存し、大文字小文字を区別せずに比べるため、二つ並ぶと計画時と承認後の一覧を突き合わせられない
		key := simpleFoldKey(existing[i].Name)
		if _, duplicate := seen[key]; duplicate {
			return EnvironmentSecrets{}, fmt.Errorf("secrets of environment %q: %w: duplicate secret name %q", name, ErrUnexpectedResponse, existing[i].Name)
		}
		seen[key] = struct{}{}
	}
	return EnvironmentSecrets{Existing: existing}, nil
}

// environmentSecretPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type environmentSecretPayload struct {
	Name      *string `json:"name"`
	UpdatedAt *string `json:"updated_at"`
}

// decodeEnvironmentSecret は Environment Secret の一覧の要素を観測値に変換する。値のフィールドは GitHub が返さないため求めない。
// updated_at の欠落と空文字列を受け付けない。承認のあとに登録し直されたことを、変わらない値と比べて見落とすため。
func decodeEnvironmentSecret(data json.RawMessage) (EnvironmentSecret, error) {
	var p environmentSecretPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return EnvironmentSecret{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.Name == nil {
		missing = append(missing, "name")
	}
	if p.UpdatedAt == nil {
		missing = append(missing, "updated_at")
	}
	if len(missing) > 0 {
		return EnvironmentSecret{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	if *p.Name == "" {
		return EnvironmentSecret{}, fmt.Errorf("%w: unsupported value: name", ErrUnexpectedResponse)
	}
	if *p.UpdatedAt == "" {
		return EnvironmentSecret{}, fmt.Errorf("%w: unsupported value: updated_at", ErrUnexpectedResponse)
	}
	return EnvironmentSecret{Name: *p.Name, UpdatedAt: *p.UpdatedAt}, nil
}
