package apply

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/githubtest"
)

const patchRepository = "PATCH /repos/owner/repo"

// securityAndAnalysisBody は、security_and_analysis の中の item の status だけを送る PATCH の本文。
func securityAndAnalysisBody(item, status string) map[string]any {
	return map[string]any{"security_and_analysis": map[string]any{item: map[string]any{"status": status}}}
}

//trace:checks FR-007 FR-010 FR-011 INV-001 INV-003
func TestRun_SecretScanningDisabling(t *testing.T) {
	t.Run("side effect is shown before approval and the push protection it disables is not an outside change", func(t *testing.T) {
		out, shown, server, err := runSecurity(t, securityConfigPrefix+"      secret_scanning: false\n", &delayRecorder{},
			[]response{okSecretScanningRepository(1, true, true)},
			[]response{okSecretScanningRepository(1, true, true), okSecretScanningRepository(1, false, false), okSecretScanningRepository(1, false, false)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		wantShown := "  security.secret_scanning: true -> false\n" +
			"    Side effect: GitHub also disables secret_scanning_push_protection, which is currently true\n"
		if !strings.Contains(shown, wantShown) {
			t.Errorf("Out at approval = %q, want it to contain %q", shown, wantShown)
		}
		// 承認した副作用で無効になった push protection は、同じ更新エンドポイントの設定でも予期しない変化に数えない
		if want := []string{getRepository, getRepository, patchRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("undeclared push protection enabled after approval stops the update although not shown", func(t *testing.T) {
		out, shown, server, err := runSecurity(t, securityConfigPrefix+"      secret_scanning: false\n", &delayRecorder{},
			[]response{okSecretScanningRepository(1, true, false)},
			[]response{okSecretScanningRepository(1, true, true)})
		if err == nil {
			t.Fatal("Run() error = nil, want the precondition failure")
		}
		if strings.Contains(shown, "Side effect") {
			t.Errorf("Out at approval = %q, want no side effect on the disabled push protection", shown)
		}
		if want := []string{getRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v without the PATCH", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "Precondition: security.secret_scanning_push_protection changed since approval") {
			t.Errorf("Out = %q, want the changed push protection", out.String())
		}
	})
}

//trace:checks FR-011 INV-004
func TestRun_SecretScanningOrder(t *testing.T) {
	t.Run("disabling both sends the push protection PATCH before the secret scanning PATCH with the update interval", func(t *testing.T) {
		delays := &delayRecorder{}
		out, _, server, err := runSecurity(t, securityConfigPrefix+"      secret_scanning: false\n      secret_scanning_push_protection: false\n", delays,
			[]response{okSecretScanningRepository(1, true, true)},
			[]response{okSecretScanningRepository(1, true, true), okSecretScanningRepository(1, true, false), okSecretScanningRepository(1, true, false)},
			[]response{okSecretScanningRepository(1, true, false), okSecretScanningRepository(1, false, false), okSecretScanningRepository(1, false, false)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{getRepository, getRepository, patchRepository, getRepository, getRepository, patchRepository, getRepository}
		if !slices.Equal(server.RequestLines(), want) {
			t.Fatalf("requests = %v, want %v", server.RequestLines(), want)
		}
		bodies := server.Bodies()
		if got, want := []map[string]any{bodies[2], bodies[5]}, []map[string]any{
			securityAndAnalysisBody("secret_scanning_push_protection", "disabled"), securityAndAnalysisBody("secret_scanning", "disabled"),
		}; !reflect.DeepEqual(got, want) {
			t.Errorf("PATCH bodies = %v, want %v", got, want)
		}
		if wantDelays := []time.Duration{time.Second}; !slices.Equal(delays.delays, wantDelays) {
			t.Errorf("delays = %v, want %v", delays.delays, wantDelays)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})
}

//trace:checks FR-012 INV-001
func TestRun_GeneralUpdateComparesSecretScanning(t *testing.T) {
	source := "repositories:\n  - name: owner/repo\n    general:\n      allow_auto_merge: true\n"

	t.Run("secret scanning changed by the general update fails the change", func(t *testing.T) {
		out, _, _, err := runSecurity(t, source, &delayRecorder{},
			[]response{okRepositoryBody(githubtest.RepositoryBody{ID: 1})},
			[]response{
				okRepositoryBody(githubtest.RepositoryBody{ID: 1, SecretScanning: true}),
				okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true}),
				okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true}),
			})
		if err == nil {
			t.Fatal("Run() error = nil, want the unexpected outside change")
		}
		if !strings.Contains(out.String(), "Other settings: security.secret_scanning changed true -> false") || !strings.Contains(out.String(), "owner/repo: failed") {
			t.Errorf("Out = %q, want the changed secret scanning and the failed change", out.String())
		}
	})

	t.Run("public repository without security_and_analysis before the update sends no PATCH", func(t *testing.T) {
		_, _, server, err := runSecurity(t, source, &delayRecorder{},
			[]response{okRepositoryBody(githubtest.RepositoryBody{ID: 1})},
			[]response{{status: http.StatusOK, body: githubtest.RepositoryBody{ID: 1}.JSON()}})
		if err == nil || !strings.Contains(err.Error(), "security_and_analysis") {
			t.Fatalf("Run() error = %v, want the missing security_and_analysis", err)
		}
		if want := []string{getRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v without the PATCH", server.RequestLines(), want)
		}
	})
}

//trace:checks FR-006 FR-010 INV-001 INV-003
func TestRun_RepositoryPushProtectionEnabling(t *testing.T) {
	source := securityConfigPrefix + "      secret_scanning_push_protection: true\n"

	t.Run("secret scanning disabled after approval stops the enabling", func(t *testing.T) {
		out, _, server, err := runSecurity(t, source, &delayRecorder{},
			[]response{okSecretScanningRepository(1, true, false)},
			[]response{okSecretScanningRepository(1, false, false)})
		if err == nil {
			t.Fatal("Run() error = nil, want the precondition failure")
		}
		if want := []string{getRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v without the PATCH", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "Precondition: security.secret_scanning changed since approval") {
			t.Errorf("Out = %q, want the changed secret scanning", out.String())
		}
	})
}
