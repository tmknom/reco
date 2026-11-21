package apply

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

const (
	// securityConfigPrefix は、owner/repo の security の設定を続けて書く設定ファイルの書き出し。
	securityConfigPrefix = "repositories:\n  - name: owner/repo\n    security:\n"
	// getRepository は、リポジトリの取得の要求。
	getRepository = "GET /repos/owner/repo"
)

// runSecurity は、設定ファイル source で Run を承認つきで実行し、観測ごとの応答の並び observations を順に返す stub に向ける。
// 戻り値は、出力、承認を求めた時点の出力、stub、Run の error である。
func runSecurity(t *testing.T, source string, delays *delayRecorder, observations ...[]response) (*bytes.Buffer, string, *githubtest.Server, error) {
	t.Helper()
	baseURL, server := newSequenceServer(t, slices.Concat(observations...)...)
	var out bytes.Buffer
	var shown string
	cfg := Config{
		Token:         "test-token",
		Configuration: configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(source)}}, Name: "reco.yaml", Path: "./reco.yaml"},
		Out:           &out,
		Approve: func(context.Context) error {
			shown = out.String()
			return nil
		},
		After:         delays.after,
		GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
	}
	err := Run(context.Background(), cfg)
	return &out, shown, server, err
}
