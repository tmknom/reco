package apply

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

// TestRun_EnvironmentDeletion は、存在しないことを宣言した Environment の削除を、計画の生成から変更結果まで通したつなぎ目を確かめる。
// 更新直前の観測で削除で失われるものまで読んで基準と比べ、成立したときだけ DELETE を一度送り、成功応答だけで完了にせず更新後の観測で存在しないと確かめる。
// 失われるものの比べ方は TestMatchesAnticipatedEnvironmentLoss、宛先は TestSendUpdate、期待状態の確認は TestConfirmDesiredState_EnvironmentDeletion が確かめている。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestRun_EnvironmentDeletion(t *testing.T) {
	newConfig := func(out *bytes.Buffer, baseURL *url.URL, calls *int) Config {
		// production は存在しないことの宣言、staging は存在の宣言だけで、計画時に存在する
		source := `repositories:
  - name: owner/repo
    environments:
      - name: production
        state: absent
      - name: staging
`
		return Config{
			Token:         "test-token",
			Configuration: configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(source)}}, Name: "reco.yaml", Path: "./reco.yaml"},
			Out:           out,
			Approve: func(context.Context) error {
				*calls++
				return nil
			},
			After:         elapsedAtOnce,
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}
	production := githubtest.EnvironmentBody{ID: 7, Name: "production"}
	staging := githubtest.EnvironmentBody{ID: 8, Name: "staging"}
	// qa は宣言していない Environment
	qa := githubtest.EnvironmentBody{ID: 9, Name: "qa"}
	region := githubtest.VariableBody{Name: "REGION", Value: "ap-northeast-1"}
	const (
		getRepository    = "GET /repos/owner/repo"
		getEnvironments  = "GET /repos/owner/repo/environments"
		getVariables     = "GET /repos/owner/repo/environments/production/variables"
		getSecrets       = "GET /repos/owner/repo/environments/production/secrets" //nolint:gosec // G101: 要求のメソッドとパスで、認証情報ではない
		deleteProduction = "DELETE /repos/owner/repo/environments/production"
	)
	// read は、production が一覧にある観測の応答と要求の並び。削除で失われるものまで読むのは production だけで、存在の宣言だけの staging と宣言していない qa には送らない
	read := func(secrets ...environmentSecretBody) []response {
		return []response{okRepository(1, false, false), okEnvironments(production, staging, qa), okVariables(region), okEnvironmentSecrets(t, secrets...)}
	}
	readRequests := []string{getRepository, getEnvironments, getVariables, getSecrets}
	// gone は、production がなくなった観測の応答と要求の並び。一覧にない Environment には変数と Secret の要求を送らない
	gone := []response{okRepository(1, false, false), okEnvironments(staging, qa)}
	goneRequests := []string{getRepository, getEnvironments}
	// selectedProduction は、方式が selected の production。削除で失われる値に個別パターンを含むため、更新直前の観測では個別パターンも読む
	selectedProduction := githubtest.EnvironmentBody{ID: 7, Name: "production", DeploymentBranchPolicy: selectedBody()}
	const getPatterns = "GET /repos/owner/repo/environments/production/deployment-branch-policies"
	readSelected := []response{
		okRepository(1, false, false), okEnvironments(selectedProduction, staging, qa),
		okDeploymentPatterns(githubtest.DeploymentPatternBody{ID: 1, Name: "main", Type: "branch"}), okVariables(region), okEnvironmentSecrets(t),
	}
	readSelectedRequests := []string{getRepository, getEnvironments, getPatterns, getVariables, getSecrets}
	// delayed は、一覧の反映が遅れて production がまだある更新後の観測の応答。
	// 更新後に比べるのは一覧での存在だけのため、削除した Environment にも個別パターン、変数、Secret の要求を送らず、要求の並びは gone と同じになる
	delayed := []response{okRepository(1, false, false), okEnvironments(selectedProduction, staging, qa)}
	// remaining は、削除のあとの観測し直しをすべて production がある応答で返す応答と、その要求の並び
	var remaining []response
	var remainingRequests []string
	for range 1 + len(reobservationDelays) {
		remaining = append(remaining, okRepository(1, false, false), okEnvironments(production, staging, qa))
		remainingRequests = append(remainingRequests, goneRequests...)
	}
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantErr      bool
		wantOut      []string
	}{
		{
			// 存在の宣言だけの staging は一致で、宣言していない qa とともに DELETE を送らない
			name:         "deletion is sent once and completes when the read after the update finds the environment absent",
			responses:    slices.Concat(read(), read(), []response{noContent()}, gone),
			wantRequests: slices.Concat(readRequests, readRequests, []string{deleteProduction}, goneRequests),
			wantOut:      []string{"owner/repo: updated", "Apply complete."},
		},
		{
			// 一覧にあった Environment への 404 は観測の失敗になるため、削除した Environment の中を読むと、反映を待たずに失敗にする
			name:         "environment left in the list by a delayed reflection is read again without its contents and completes",
			responses:    slices.Concat(readSelected, readSelected, []response{noContent()}, delayed, gone),
			wantRequests: slices.Concat(readSelectedRequests, readSelectedRequests, []string{deleteProduction}, goneRequests, goneRequests),
			wantOut:      []string{"owner/repo: updated", "Apply complete."},
		},
		{
			name: "secret added after approval stops the deletion before it is sent",
			responses: slices.Concat(read(), read(environmentSecretBody{
				Name: "TOKEN", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z",
			})),
			wantRequests: slices.Concat(readRequests, readRequests),
			wantErr:      true,
			wantOut:      []string{"owner/repo: not attempted", "Precondition: environments.production changed since approval"},
		},
		{
			name:         "404 to DELETE is a failed update and is not read as absence",
			responses:    slices.Concat(read(), read(), []response{{status: http.StatusNotFound, body: `{"message": "Not Found"}`}}),
			wantRequests: slices.Concat(readRequests, readRequests, []string{deleteProduction}),
			wantErr:      true,
			wantOut:      []string{"owner/repo: failed", "Update request: rejected", "Desired state: not checked"},
		},
		{
			name:         "environment still present after the update is a mismatch with the recreation cause",
			responses:    slices.Concat(read(), read(), []response{noContent()}, remaining),
			wantRequests: slices.Concat(readRequests, readRequests, []string{deleteProduction}, remainingRequests),
			wantErr:      true,
			wantOut: []string{
				"owner/repo: failed", "Update request: succeeded", "Desired state: does not match",
				"Possible cause: a workflow that references the environment may have run after the deletion",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			var out bytes.Buffer
			calls := 0

			err := Run(context.Background(), newConfig(&out, baseURL, &calls))

			if (err != nil) != tc.wantErr {
				t.Errorf("Run() error = %v, want error %v", err, tc.wantErr)
			}
			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if calls != 1 {
				t.Errorf("approve calls = %d, want 1", calls)
			}
			for _, want := range tc.wantOut {
				if !strings.Contains(out.String(), want) {
					t.Errorf("Out = %q, want it to contain %q", out.String(), want)
				}
			}
		})
	}
}
