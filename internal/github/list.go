package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// listPerPage は一覧の1ページで求める件数。GitHub が受け付ける上限。
	listPerPage = 100
	// maxListPages は読むページ数の上限。異常な応答で要求を送り続けないためにある。
	maxListPages = 100
)

// listAll は u の一覧を全ページ読み、itemsField の配列の要素を届いた順につないで返す。
// 失敗したときは、読めたページの要素を返さない。読めた範囲だけから、存在しないとは判断させないため。
// 次のページへの要求は reco が組み立てた URL にだけ送り、応答の Link ヘッダーは読まない。Link の URL をたどると、送信先を応答の内容に委ねるため。
func (c *Client) listAll(ctx context.Context, u *url.URL, itemsField string) ([]json.RawMessage, error) {
	var items []json.RawMessage
	var firstTotal int
	for page := 1; page <= maxListPages; page++ {
		_, body, err := c.send(ctx, http.MethodGet, listPageURL(u, page), nil, http.StatusOK)
		if err != nil {
			return nil, err
		}
		total, pageItems, err := decodeListPage(body, itemsField)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			firstTotal = total
		}
		complete, err := listProgress(firstTotal, total, len(items), len(pageItems))
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if complete {
			return items, nil
		}
	}
	return nil, fmt.Errorf("%w: list not complete after %d pages", ErrUnexpectedResponse, maxListPages)
}

// listProgress は、1ページを読んだあとに、全件を読み終えたかを返す。
// 全件数は1ページ目の total_count で、読んでいるあいだに変わった一覧は、ページの境目で要素を読み落とし得るため失敗とする。
func listProgress(firstTotal, pageTotal, readBefore, pageItems int) (complete bool, err error) {
	if pageTotal != firstTotal {
		return false, fmt.Errorf("%w: total_count changed from %d to %d while reading the list", ErrUnexpectedResponse, firstTotal, pageTotal)
	}
	read := readBefore + pageItems
	if read > pageTotal {
		return false, fmt.Errorf("%w: read %d items, more than total_count %d", ErrUnexpectedResponse, read, pageTotal)
	}
	if read == pageTotal {
		return true, nil
	}
	if pageItems == 0 {
		return false, fmt.Errorf("%w: empty page after %d of %d items", ErrUnexpectedResponse, readBefore, pageTotal)
	}
	return false, nil
}

// decodeListPage は一覧の1ページの応答本文から、total_count と itemsField の配列の要素を取り出す。
// 項目名は一覧ごとに違い、要素の中身はこの関数では読まない。
func decodeListPage(data []byte, itemsField string) (total int, items []json.RawMessage, err error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	rawTotal := fields["total_count"]
	if isAbsent(rawTotal) {
		missing = append(missing, "total_count")
	}
	rawItems := fields[itemsField]
	if isAbsent(rawItems) {
		missing = append(missing, itemsField)
	}
	if len(missing) > 0 {
		return 0, nil, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	if err := json.Unmarshal(rawTotal, &total); err != nil {
		return 0, nil, fmt.Errorf("%w: total_count: %w", ErrUnexpectedResponse, err)
	}
	if err := json.Unmarshal(rawItems, &items); err != nil {
		return 0, nil, fmt.Errorf("%w: %s: %w", ErrUnexpectedResponse, itemsField, err)
	}
	return total, items, nil
}

// isAbsent は、キーの欠落か null の値かを返す。欠落した値は nil、null の値は "null" の RawMessage になる。
func isAbsent(raw json.RawMessage) bool {
	return raw == nil || string(raw) == "null"
}

// listPageURL は u の写しに、1ページの件数と page 番号の検索条件を付けて返す。u は書き換えない。
func listPageURL(u *url.URL, page int) *url.URL {
	paged := *u
	paged.RawQuery = fmt.Sprintf("per_page=%d&page=%d", listPerPage, page)
	return &paged
}
