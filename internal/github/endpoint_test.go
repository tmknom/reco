package github

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestUpdateEndpointObservedSettings は、更新エンドポイントごとの、reco が観測する設定の一覧の不変条件を確かめる。
//
//trace:checks INV-001 FR-012
func TestUpdateEndpointObservedSettings(t *testing.T) {
	// 更新操作を足して更新エンドポイントの一覧に足し忘れると、同じ更新エンドポイントのほかの設定を変えるときに
	// その設定を比べ漏らすため、すべての管理の単位を回して検出する。
	t.Run("every management unit is observed at its update endpoint", func(t *testing.T) {
		for _, key := range setting.ManagementUnits() {
			if got := UpdateEndpointOf(key).ObservedSettings(); !slices.Contains(got, key) {
				t.Errorf("ObservedSettings() of the update endpoint of %q = %v, want it to contain %q", key, got, key)
			}
		}
	})

	// 更新エンドポイントごとに本文なしの要求を送るため、ほかの設定を観測に含めると、更新後に比べる設定が増えて保全の確認が崩れる。
	t.Run("each dependabot endpoint observes only its own setting", func(t *testing.T) {
		cases := []struct {
			endpoint UpdateEndpoint
			want     []setting.Key
		}{
			{endpoint: DependabotAlertsEndpoint, want: []setting.Key{setting.DependabotAlerts}},
			{endpoint: DependabotSecurityUpdatesEndpoint, want: []setting.Key{setting.DependabotSecurityUpdates}},
		}
		for _, tc := range cases {
			if got := tc.endpoint.ObservedSettings(); !slices.Equal(got, tc.want) {
				t.Errorf("ObservedSettings() of update endpoint %d = %v, want %v", tc.endpoint, got, tc.want)
			}
		}
	})

	t.Run("undefined update endpoint panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("ObservedSettings() did not panic for an undefined update endpoint")
			}
		}()
		_ = UpdateEndpoint(0).ObservedSettings()
	})
}
