package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// listPerPage は、多くの一覧で GitHub が受け付ける1ページの件数の上限。上限が異なる一覧は自分の値を listPaging に渡す。
	listPerPage = 100
	// maxListPages は、多くの一覧で読むページ数の上限。異常な応答で要求を送り続けないためにある。
	maxListPages = 100
)

// listPaging は、一覧ごとに決まる、ページの求め方と読み方。
type listPaging struct {
	// itemsField は要素の配列の項目名。
	itemsField string
	// perPage は1ページで求める件数。
	perPage int
	// maxPageBytes は1ページの応答本文を読む上限。
	maxPageBytes int
	// maxPages は読むページ数の上限。
	maxPages int
}

// listAll は u の一覧を全ページ読み、paging.itemsField の配列の要素を届いた順につないで返す。
// 失敗したときは、読めたページの要素を返さない。読めた範囲だけから、存在しないとは判断させないため。
// 次のページへの要求は reco が組み立てた URL にだけ送り、応答の Link ヘッダーは読まない。Link の URL をたどると、送信先を応答の内容に委ねるため。
func (c *Client) listAll(ctx context.Context, u *url.URL, paging listPaging) ([]json.RawMessage, error) {
	var items []json.RawMessage
	var firstTotal int
	for page := 1; page <= paging.maxPages; page++ {
		_, body, err := c.sendWithBodyLimit(ctx, http.MethodGet, listPageURL(u, paging.perPage, page), nil, http.StatusOK, paging.maxPageBytes)
		if err != nil {
			return nil, err
		}
		total, pageItems, err := decodeListPage(body, paging.itemsField)
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
	return nil, fmt.Errorf("%w: list not complete after %d pages", ErrUnexpectedResponse, paging.maxPages)
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

// listSinglePage は、total_count を返さず要素の配列だけを返す一覧 u を、1ページだけ読む。u の検索条件は保ち、listPerPage の件数を求める。
// 公式資料の件数の上限が listPerPage より小さい一覧だけに使う。listPerPage ちょうどの要素を返したページは最後のページと言えず、
// 次のページを読んでも読んでいるあいだの増減を検出できないため、ページ数の上限1まで読んでも最後のページに達しない失敗とする。
func (c *Client) listSinglePage(ctx context.Context, u *url.URL) ([]json.RawMessage, error) {
	paged := *u
	query := paged.Query()
	query.Set("per_page", strconv.Itoa(listPerPage))
	paged.RawQuery = query.Encode()
	_, body, err := c.send(ctx, http.MethodGet, &paged, nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	items, err := decodeArrayPage(body)
	if err != nil {
		return nil, err
	}
	if len(items) >= listPerPage {
		return nil, fmt.Errorf("%w: list not complete after 1 page", ErrUnexpectedResponse)
	}
	return items, nil
}

// decodeArrayPage は、要素の配列だけを返す一覧の1ページの応答本文から、要素を届いた順に取り出す。要素の中身は読まない。
// null は配列でないため失敗とする。空の一覧と読むと、すべての要素を読んだと誤るため。
func decodeArrayPage(data []byte) ([]json.RawMessage, error) {
	var items *[]json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if items == nil {
		return nil, fmt.Errorf("%w: the list is not an array", ErrUnexpectedResponse)
	}
	return *items, nil
}

// isAbsent は、キーの欠落か null の値かを返す。欠落した値は nil、null の値は "null" の RawMessage になる。
func isAbsent(raw json.RawMessage) bool {
	return raw == nil || string(raw) == "null"
}

// listPageURL は u の写しに、1ページの件数 perPage と page 番号の検索条件を付けて返す。u は書き換えない。
func listPageURL(u *url.URL, perPage, page int) *url.URL {
	paged := *u
	paged.RawQuery = fmt.Sprintf("per_page=%d&page=%d", perPage, page)
	return &paged
}
