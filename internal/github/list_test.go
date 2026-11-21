package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// TestListProgress は、1ページを読んだあとの読み終えたかの判定を確かめる。
// 次のページを読むか、読み終えたか、失敗かは、件数の組だけで決まる。
//
//trace:checks FR-004 QR-002
func TestListProgress(t *testing.T) {
	cases := []struct {
		name         string
		firstTotal   int
		pageTotal    int
		readBefore   int
		pageItems    int
		wantComplete bool
		wantErr      bool
	}{
		{name: "empty first page with total 0 is complete", firstTotal: 0, pageTotal: 0, readBefore: 0, pageItems: 0, wantComplete: true},
		{name: "all items in the first page is complete", firstTotal: 3, pageTotal: 3, readBefore: 0, pageItems: 3, wantComplete: true},
		{name: "a full page that reaches the total is complete without checking the next page", firstTotal: 100, pageTotal: 100, readBefore: 0, pageItems: 100, wantComplete: true},
		{name: "last page reaching the total is complete", firstTotal: 150, pageTotal: 150, readBefore: 100, pageItems: 50, wantComplete: true},
		{name: "full page before the total continues", firstTotal: 150, pageTotal: 150, readBefore: 0, pageItems: 100},
		{name: "short page before the total continues", firstTotal: 150, pageTotal: 150, readBefore: 0, pageItems: 50},
		{name: "empty first page before the total is unexpected response", firstTotal: 5, pageTotal: 5, readBefore: 0, pageItems: 0, wantErr: true},
		{name: "empty later page before the total is unexpected response", firstTotal: 5, pageTotal: 5, readBefore: 3, pageItems: 0, wantErr: true},
		{name: "more items than the total is unexpected response", firstTotal: 3, pageTotal: 3, readBefore: 0, pageItems: 4, wantErr: true},
		{name: "items across pages beyond the total is unexpected response", firstTotal: 5, pageTotal: 5, readBefore: 3, pageItems: 3, wantErr: true},
		{name: "negative total with items is unexpected response", firstTotal: -1, pageTotal: -1, readBefore: 0, pageItems: 1, wantErr: true},
		{name: "negative total without items is unexpected response", firstTotal: -1, pageTotal: -1, readBefore: 0, pageItems: 0, wantErr: true},
		{name: "total that grew on a later page is unexpected response even if the new total is reached", firstTotal: 5, pageTotal: 6, readBefore: 3, pageItems: 3, wantErr: true},
		{name: "total that shrank on a later page is unexpected response even if the new total is reached", firstTotal: 5, pageTotal: 4, readBefore: 2, pageItems: 2, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			complete, err := listProgress(tc.firstTotal, tc.pageTotal, tc.readBefore, tc.pageItems)
			if tc.wantErr {
				if !errors.Is(err, ErrUnexpectedResponse) {
					t.Fatalf("listProgress() error = %v, want %v", err, ErrUnexpectedResponse)
				}
				return
			}
			if err != nil {
				t.Fatalf("listProgress() error = %v", err)
			}
			if complete != tc.wantComplete {
				t.Errorf("listProgress() complete = %t, want %t", complete, tc.wantComplete)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeListPage(t *testing.T) {
	t.Run("total and items are returned in the order they arrived", func(t *testing.T) {
		total, items, err := decodeListPage([]byte(`{"total_count": 7, "things": [{"n": 2}, {"n": 1}], "other": [{"n": 9}]}`), "things")
		if err != nil {
			t.Fatalf("decodeListPage() error = %v", err)
		}
		if total != 7 {
			t.Errorf("decodeListPage() total = %d, want 7", total)
		}
		if len(items) != 2 || string(items[0]) != `{"n": 2}` || string(items[1]) != `{"n": 1}` {
			t.Errorf("decodeListPage() items = %q, want the two items in the arrival order", items)
		}
	})

	t.Run("empty items are returned as no items", func(t *testing.T) {
		total, items, err := decodeListPage([]byte(`{"total_count": 0, "things": []}`), "things")
		if err != nil {
			t.Fatalf("decodeListPage() error = %v", err)
		}
		if total != 0 || len(items) != 0 {
			t.Errorf("decodeListPage() = %d, %q, want 0 and no items", total, items)
		}
	})

	failures := []struct {
		name        string
		body        string
		errContains []string
	}{
		{name: "missing total_count is unexpected response", body: `{"things": []}`, errContains: []string{"missing fields", "total_count"}},
		{name: "null total_count is unexpected response", body: `{"total_count": null, "things": []}`, errContains: []string{"missing fields", "total_count"}},
		{name: "string total_count is unexpected response", body: `{"total_count": "1", "things": []}`},
		{name: "fractional total_count is unexpected response", body: `{"total_count": 1.5, "things": []}`},
		{name: "missing items array is unexpected response", body: `{"total_count": 0}`, errContains: []string{"missing fields", "things"}},
		{name: "null items array is unexpected response", body: `{"total_count": 0, "things": null}`, errContains: []string{"missing fields", "things"}},
		{name: "object items is unexpected response", body: `{"total_count": 0, "things": {}}`},
		{name: "array under a different name only is unexpected response", body: `{"total_count": 0, "other": []}`, errContains: []string{"things"}},
		{name: "empty object names both missing fields", body: `{}`, errContains: []string{"total_count", "things"}},
		{name: "array body is unexpected response", body: `[]`},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, items, err := decodeListPage([]byte(tc.body), "things")
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeListPage() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if items != nil {
				t.Errorf("decodeListPage() items = %q, want none on failure", items)
			}
			for _, s := range tc.errContains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeListPage() error = %q, want it to contain %q", err, s)
				}
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestListPageURL(t *testing.T) {
	u, err := url.Parse("http://127.0.0.1:1/repos/owner/repo/environments/a%2Fb/deployment-branch-policies")
	if err != nil {
		t.Fatal(err)
	}

	got := listPageURL(u, 3)

	query := got.Query()
	if query.Get("per_page") != "100" || query.Get("page") != "3" || len(query) != 2 {
		t.Errorf("listPageURL() query = %q, want per_page=100 and page=3 only", got.RawQuery)
	}
	if got.EscapedPath() != u.EscapedPath() || got.Host != u.Host || got.Scheme != u.Scheme {
		t.Errorf("listPageURL() = %q, want the same origin and path as %q", got, u)
	}
	if u.RawQuery != "" {
		t.Errorf("listPageURL() changed the given URL to %q", u)
	}
}

// listPageBody は、listAll のテストが使う一覧の1ページの応答本文である。
func listPageBody(total int, ns ...int) string {
	items := make([]string, len(ns))
	for i, n := range ns {
		items[i] = fmt.Sprintf(`{"n": %d}`, n)
	}
	return fmt.Sprintf(`{"total_count": %d, "things": [%s]}`, total, strings.Join(items, ", "))
}

// TestListAll は、全ページを reco が組み立てた URL に順に送り、要素をつなぐことを確かめる。
//
//trace:checks FR-003 FR-004 QR-001 QR-002
func TestListAll(t *testing.T) {
	const path = "/repos/owner/repo/things"
	var foreign atomic.Int32
	foreignServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		foreign.Add(1)
	}))
	t.Cleanup(foreignServer.Close)
	link := fmt.Sprintf(`<%s/repositories/1/things?per_page=100&page=2>; rel="next"`, foreignServer.URL)

	listURL := func(c *Client) *url.URL {
		u := c.repositoryURL(mustReference(t, "owner/repo"))
		u.Path += "/things"
		return u
	}

	t.Run("pages are requested in order on the same path and their items are joined", func(t *testing.T) {
		var requests []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.Path+"?"+r.URL.Query().Encode())
			w.Header().Set("Link", link)
			switch r.URL.Query().Get("page") {
			case "1":
				_, _ = w.Write([]byte(listPageBody(3, 1, 2)))
			case "2":
				_, _ = w.Write([]byte(listPageBody(3, 3)))
			default:
				t.Errorf("unexpected request %s", r.URL)
				w.WriteHeader(http.StatusInternalServerError)
			}
		})

		items, err := c.listAll(t.Context(), listURL(c), "things")
		if err != nil {
			t.Fatalf("listAll() error = %v", err)
		}
		var got []string
		for _, item := range items {
			got = append(got, string(item))
		}
		want := []string{`{"n": 1}`, `{"n": 2}`, `{"n": 3}`}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("listAll() items = %q, want %q", got, want)
		}
		wantRequests := []string{path + "?page=1&per_page=100", path + "?page=2&per_page=100"}
		if strings.Join(requests, "|") != strings.Join(wantRequests, "|") {
			t.Errorf("listAll() requests = %q, want %q", requests, wantRequests)
		}
		if n := foreign.Load(); n != 0 {
			t.Errorf("the Link header target received %d requests, want 0", n)
		}
	})

	t.Run("reaching the total does not send another request", func(t *testing.T) {
		var count atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			count.Add(1)
			_, _ = w.Write([]byte(listPageBody(2, 1, 2)))
		})
		items, err := c.listAll(t.Context(), listURL(c), "things")
		if err != nil {
			t.Fatalf("listAll() error = %v", err)
		}
		if len(items) != 2 || count.Load() != 1 {
			t.Errorf("listAll() returned %d items with %d requests, want 2 items with 1 request", len(items), count.Load())
		}
	})

	failures := []struct {
		name    string
		page2   http.HandlerFunc
		wantErr error
	}{
		{name: "502 on the second page is unexpected response", page2: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }, wantErr: ErrUnexpectedResponse},
		{name: "403 on the second page is forbidden", page2: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }, wantErr: ErrForbidden},
		{name: "301 on the second page is redirect and is not followed", page2: func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, foreignServer.URL+"/moved", http.StatusMovedPermanently)
		}, wantErr: ErrRedirect},
		{name: "total that changed on the second page is unexpected response", page2: func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(listPageBody(4, 3, 4)))
		}, wantErr: ErrUnexpectedResponse},
		{name: "non-JSON second page is unexpected response", page2: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`<html>`)) }, wantErr: ErrUnexpectedResponse},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "1" {
					_, _ = w.Write([]byte(listPageBody(3, 1, 2)))
					return
				}
				tc.page2(w, r)
			})
			items, err := c.listAll(t.Context(), listURL(c), "things")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("listAll() error = %v, want %v", err, tc.wantErr)
			}
			if items != nil {
				t.Errorf("listAll() items = %q, want none when a page fails", items)
			}
		})
	}
	if n := foreign.Load(); n != 0 {
		t.Errorf("the redirect target received %d requests, want 0", n)
	}

	t.Run("a list that never reaches the total stops at the page limit", func(t *testing.T) {
		var count atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			_, _ = w.Write([]byte(listPageBody(1_000_000, page)))
		})
		items, err := c.listAll(t.Context(), listURL(c), "things")
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("listAll() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if items != nil {
			t.Errorf("listAll() items = %d items, want none", len(items))
		}
		if got := int(count.Load()); got != maxListPages {
			t.Errorf("listAll() sent %d requests, want %d", got, maxListPages)
		}
	})

	t.Run("the list that reaches the total at the page limit is complete", func(t *testing.T) {
		var count atomic.Int32
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			_, _ = w.Write([]byte(listPageBody(maxListPages, page)))
		})
		items, err := c.listAll(t.Context(), listURL(c), "things")
		if err != nil {
			t.Fatalf("listAll() error = %v", err)
		}
		if len(items) != maxListPages || int(count.Load()) != maxListPages {
			t.Errorf("listAll() returned %d items with %d requests, want %d of each", len(items), count.Load(), maxListPages)
		}
	})
}
