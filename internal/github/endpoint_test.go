package github

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestUpdateEndpointObservedSettings は、更新エンドポイントごとの、reco が観測する設定の一覧の不変条件を確かめる。
//
//trace:checks INV-001
//trace:checks FR-012
func TestUpdateEndpointObservedSettings(t *testing.T) {
	// 更新要求を持たない管理の単位は、UpdateEndpointOf が panic するため除く。
	managementUnitsWithUpdateRequest := slices.DeleteFunc(setting.ManagementUnits(), func(key setting.Key) bool { return key == setting.ActionsEnabled })
	// 更新操作を足して更新エンドポイントの一覧に足し忘れると、同じ更新エンドポイントのほかの設定を変えるときに
	// その設定を比べ漏らすため、すべての管理の単位を回して検出する。
	t.Run("every management unit is observed at its update endpoint", func(t *testing.T) {
		for _, key := range managementUnitsWithUpdateRequest {
			if got := UpdateEndpointOf(key).ObservedSettings(); !slices.Contains(got, key) {
				t.Errorf("ObservedSettings() of the update endpoint of %q = %v, want it to contain %q", key, got, key)
			}
		}
	})

	// 観測は変える設定の領域の単位で読むため、領域の外の設定を比べると、更新要求を送ったあとで観測値がなく止まる。
	t.Run("every observed setting at an update endpoint is in the area of the setting it updates", func(t *testing.T) {
		for _, key := range managementUnitsWithUpdateRequest {
			for _, observed := range UpdateEndpointOf(key).ObservedSettings() {
				if observed.Area() != key.Area() {
					t.Errorf("%q observed at the update endpoint of %q is in area %v, want %v", observed, key, observed.Area(), key.Area())
				}
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
