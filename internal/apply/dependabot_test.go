package apply

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
)

const (
	getAdminRepository      = "GET /repos/owner/repo"
	getDependabotAlerts     = "GET /repos/owner/repo/vulnerability-alerts"
	getSecurityUpdates      = "GET /repos/owner/repo/automated-security-fixes"
	deleteDependabotAlerts  = "DELETE /repos/owner/repo/vulnerability-alerts"
	deleteSecurityUpdates   = "DELETE /repos/owner/repo/automated-security-fixes"
	dependabotConfigPrefix  = "repositories:\n  - name: owner/repo\n    security:\n"
	dependabotAlertsOffLine = "      dependabot_alerts: false\n"
)

// dependabotReads は、Dependabot の領域を読む観測の要求の並び。どの観測も Dependabot alerts だけを読むことはない。
var dependabotReads = []string{getAdminRepository, getDependabotAlerts, getSecurityUpdates}

// dependabotObservation は、dependabotReads に返す応答の並び。
func dependabotObservation(alerts, updates, paused bool) []response {
	return []response{okAdminRepository(1), dependabotAlerts(alerts), okSecurityUpdates(updates, paused)}
}

func runDependabot(t *testing.T, source string, delays *delayRecorder, observations ...[]response) (*bytes.Buffer, string, []string, error) {
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
	return &out, shown, server.RequestLines(), err
}

//trace:checks FR-007 FR-010 FR-011 INV-001 INV-003 INV-004
func TestRun_DependabotAlertsDisabling(t *testing.T) {
	t.Run("side effects are shown before approval and the alerts are disabled by an empty DELETE", func(t *testing.T) {
		delays := &delayRecorder{}
		out, shown, requests, err := runDependabot(t, dependabotConfigPrefix+dependabotAlertsOffLine, delays,
			dependabotObservation(true, true, true),
			dependabotObservation(true, true, true), []response{noContent()},
			dependabotObservation(false, false, false))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		wantShown := "  security.dependabot_alerts: true -> false\n" +
			"    Side effect: GitHub also disables the dependency graph\n" +
			"    Side effect: GitHub also disables dependabot_security_updates, which is currently true (paused by GitHub)\n"
		if !strings.Contains(shown, wantShown) {
			t.Errorf("Out at approval = %q, want it to contain %q", shown, wantShown)
		}
		// 承認した副作用で無効になった Dependabot security updates は、別の更新エンドポイントの設定で、予期しない変化に数えない
		want := slices.Concat(dependabotReads, dependabotReads, []string{deleteDependabotAlerts}, dependabotReads)
		if !slices.Equal(requests, want) || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("requests = %v, Out = %q, want %v and the repository updated", requests, out.String(), want)
		}
	})

	t.Run("security updates shown as a side effect and disabled before the update stop the alerts update", func(t *testing.T) {
		out, _, requests, err := runDependabot(t, dependabotConfigPrefix+dependabotAlertsOffLine, &delayRecorder{},
			dependabotObservation(true, true, false),
			dependabotObservation(true, false, false))
		if err == nil {
			t.Fatal("Run() error = nil, want the precondition failure")
		}
		if want := slices.Concat(dependabotReads, dependabotReads); !slices.Equal(requests, want) {
			t.Errorf("requests = %v, want %v without the DELETE", requests, want)
		}
		if !strings.Contains(out.String(), "Precondition: security.dependabot_security_updates changed since approval") {
			t.Errorf("Out = %q, want the changed security updates", out.String())
		}
	})

	t.Run("undeclared security updates enabled after approval stop the alerts update although not shown", func(t *testing.T) {
		out, shown, requests, err := runDependabot(t, dependabotConfigPrefix+dependabotAlertsOffLine, &delayRecorder{},
			dependabotObservation(true, false, false),
			dependabotObservation(true, true, false))
		if err == nil {
			t.Fatal("Run() error = nil, want the precondition failure")
		}
		if strings.Contains(shown, "dependabot_security_updates") {
			t.Errorf("Out at approval = %q, want no side effect on the disabled security updates", shown)
		}
		if want := slices.Concat(dependabotReads, dependabotReads); !slices.Equal(requests, want) {
			t.Errorf("requests = %v, want %v without the DELETE", requests, want)
		}
		if !strings.Contains(out.String(), "Precondition: security.dependabot_security_updates changed since approval") {
			t.Errorf("Out = %q, want the changed security updates", out.String())
		}
	})
}

//trace:checks FR-011 INV-004
func TestRun_DependabotOrder(t *testing.T) {
	t.Run("disabling both runs the security updates DELETE before the alerts DELETE with the update interval", func(t *testing.T) {
		delays := &delayRecorder{}
		out, _, requests, err := runDependabot(t, dependabotConfigPrefix+dependabotAlertsOffLine+"      dependabot_security_updates: false\n", delays,
			dependabotObservation(true, true, false),
			dependabotObservation(true, true, false), []response{noContent()}, dependabotObservation(true, false, false),
			dependabotObservation(true, false, false), []response{noContent()}, dependabotObservation(false, false, false))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := slices.Concat(dependabotReads,
			dependabotReads, []string{deleteSecurityUpdates}, dependabotReads,
			dependabotReads, []string{deleteDependabotAlerts}, dependabotReads)
		if !slices.Equal(requests, want) {
			t.Errorf("requests = %v, want %v", requests, want)
		}
		if wantDelays := []time.Duration{time.Second}; !slices.Equal(delays.delays, wantDelays) {
			t.Errorf("delays = %v, want %v", delays.delays, wantDelays)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

}

//trace:checks FR-006 INV-001
func TestRun_DependabotSecurityUpdatesWithDisabledAlerts(t *testing.T) {
	out, _, requests, err := runDependabot(t, dependabotConfigPrefix+"      dependabot_security_updates: true\n", &delayRecorder{},
		dependabotObservation(false, false, false))
	if err == nil || !strings.Contains(err.Error(), "plan cannot be applied") {
		t.Fatalf("Run() error = %v, want the plan that cannot be applied", err)
	}
	if !slices.Equal(requests, dependabotReads) {
		t.Errorf("requests = %v, want only the plan observation %v", requests, dependabotReads)
	}
	if !strings.Contains(out.String(), "Applicability: not applicable (Dependabot alerts is disabled; declare dependabot_alerts: true to enable it)") {
		t.Errorf("Out = %q, want the not-applicable reason", out.String())
	}
}
