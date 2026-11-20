// Package githubtest は、テストが GitHub REST API の代わりに立てる stub と、その応答本文を提供する。
// エンドポイントや応答の項目を足すときに直す場所を一か所にするため、各 package のテストはここの stub を使う。
// 本番のコードからは使わない。
package githubtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Server は、受け取った要求を届いた順に記録する GitHub の stub である。
type Server struct {
	t       testing.TB
	handler http.Handler
	url     *url.URL

	mu       sync.Mutex
	requests []Request
}

// Request は、stub が受け取った要求のうちテストが確かめる値である。
type Request struct {
	Method        string
	Path          string
	Authorization string
	// Body は要求の JSON 本文。本文のない要求では nil。
	Body map[string]any
}

// Response は、stub が一つの要求に返す応答である。
type Response struct {
	Status int
	Body   string
}

// NewServer は、要求を記録してから handler に応答させる stub を立てる。stub はテストの終わりに閉じる。
func NewServer(t testing.TB, handler http.Handler) *Server {
	t.Helper()
	s := &Server{t: t, handler: handler}
	server := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(server.Close)
	s.url = &url.URL{Scheme: "http", Host: server.Listener.Addr().String()}
	return s
}

// OK は、body を本文とする 200 の応答を返す。
func OK(body string) Response {
	return Response{Status: http.StatusOK, Body: body}
}

// Routes は、「メソッド パス」ごとに登録した応答を返す。登録のない要求は 500 を返し、テストの失敗として報告する。
func Routes(t testing.TB, routes map[string]Response) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		line := r.Method + " " + r.URL.Path
		response, ok := routes[line]
		if !ok {
			t.Errorf("got request %s, want one of the registered routes", line)
			response = Response{Status: http.StatusInternalServerError}
		}
		response.ServeHTTP(w, r)
	})
}

// Sequence は、要求が届いた順に responses を一つずつ返す。同じエンドポイントへの要求ごとに違う応答が要るテストのためのもの。
// responses を超える要求は 500 を返し、テストの失敗として報告する。
func Sequence(t testing.TB, responses ...Response) http.Handler {
	var mu sync.Mutex
	next := 0
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := next
		next++
		mu.Unlock()
		response := Response{Status: http.StatusInternalServerError}
		if n < len(responses) {
			response = responses[n]
		} else {
			t.Errorf("got request %d %s %s, want at most %d requests", n+1, r.Method, r.URL.Path, len(responses))
		}
		response.ServeHTTP(w, r)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Errorf("read request body: %v", err)
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			s.t.Errorf("decode request body %q: %v", raw, err)
		}
	}
	s.mu.Lock()
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Authorization: r.Header.Get("Authorization"), Body: body})
	s.mu.Unlock()
	s.handler.ServeHTTP(w, r)
}

// URL は stub の接続先を返す。
func (s *Server) URL() *url.URL {
	u := *s.url
	return &u
}

// Methods は、受け取った要求のメソッドを届いた順に返す。
func (s *Server) Methods() []string {
	var methods []string
	for _, r := range s.Requests() {
		methods = append(methods, r.Method)
	}
	return methods
}

// Requests は、受け取った要求を届いた順に返す。
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// RequestLines は、受け取った要求を「メソッド パス」の形で届いた順に返す。
func (s *Server) RequestLines() []string {
	var lines []string
	for _, r := range s.Requests() {
		lines = append(lines, r.Method+" "+r.Path)
	}
	return lines
}

// Bodies は、受け取った要求の本文を届いた順に返す。
func (s *Server) Bodies() []map[string]any {
	var bodies []map[string]any
	for _, r := range s.Requests() {
		bodies = append(bodies, r.Body)
	}
	return bodies
}

// ServeHTTP は、どの要求にも r を返す。
func (r Response) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(r.Status)
	// 書き込みの失敗は、要求を送ったクライアントの側で読み取りの失敗として現れ、そのテストが検出する
	_, _ = w.Write([]byte(r.Body))
}
