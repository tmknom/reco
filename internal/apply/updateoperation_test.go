package apply

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestUpdateOperationOf は、論理的な設定変更の設定から、それを実現する更新操作を選ぶ規則を確かめる。
//
//trace:checks INV-001
//trace:checks FR-011
func TestUpdateOperationOf(t *testing.T) {
	noContent := response{status: http.StatusNoContent}
	cases := []struct {
		name     string
		key      setting.Key
		desired  setting.Value
		response response
		wantBody map[string]any
	}{
		{
			name: "delete_branch_on_merge sends only delete_branch_on_merge", key: setting.DeleteBranchOnMerge, desired: setting.BoolValue(true),
			response: okRepository(1, true, true), wantBody: map[string]any{"delete_branch_on_merge": true},
		},
		{
			name: "allow_auto_merge sends only allow_auto_merge", key: setting.AllowAutoMerge, desired: setting.BoolValue(true),
			response: okRepository(1, true, true), wantBody: map[string]any{"allow_auto_merge": true},
		},
		{
			name: "default_workflow_permissions sends only default_workflow_permissions", key: setting.DefaultWorkflowPermissions,
			desired:  setting.PermissionLevelValue(setting.PermissionRead),
			response: noContent, wantBody: map[string]any{"default_workflow_permissions": "read"},
		},
		{
			name: "can_approve_pull_request_reviews sends only can_approve_pull_request_reviews", key: setting.CanApprovePullRequestReviews,
			desired:  setting.BoolValue(false),
			response: noContent, wantBody: map[string]any{"can_approve_pull_request_reviews": false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.response)
			update, ok := updateOperationOf(tc.key)
			if !ok {
				t.Fatalf("updateOperationOf(%q) ok = false, want true", tc.key)
			}

			if _, err := update.send(context.Background(), newClient(baseURL), mustReference(t, "owner/repo"), tc.desired); err != nil {
				t.Fatalf("send() error = %v", err)
			}

			if want := []map[string]any{tc.wantBody}; !reflect.DeepEqual(rec.Bodies(), want) {
				t.Errorf("request bodies = %v, want %v", rec.Bodies(), want)
			}
		})
	}

	t.Run("setting without an update operation is not ok", func(t *testing.T) {
		if _, ok := updateOperationOf(unknownKey); ok {
			t.Errorf("updateOperationOf(%q) ok = true, want false", unknownKey)
		}
	})
}

// TestUpdateEndpointObservedSettings は、更新エンドポイントごとの、reco が観測する設定の一覧の不変条件を確かめる。
//
//trace:checks INV-001
//trace:checks FR-012
func TestUpdateEndpointObservedSettings(t *testing.T) {
	// 更新操作を足して更新エンドポイントの一覧に足し忘れると、同じ更新エンドポイントのほかの設定を変えるときに
	// その設定を比べ漏らすため、宣言を受け付けるすべての設定を回して検出する。
	t.Run("every setting with an update operation is observed at its update endpoint", func(t *testing.T) {
		for _, key := range setting.Keys() {
			update, ok := updateOperationOf(key)
			if !ok {
				continue
			}
			if got := update.endpoint.observedSettings(); !slices.Contains(got, key) {
				t.Errorf("observedSettings() of the update endpoint of %q = %v, want it to contain %q", key, got, key)
			}
		}
	})

	// 観測は変える設定の領域の単位で読むため、領域の外の設定を比べると、更新要求を送ったあとで観測値がなく止まる。
	t.Run("every observed setting at an update endpoint is in the area of the setting it updates", func(t *testing.T) {
		for _, key := range setting.Keys() {
			update, ok := updateOperationOf(key)
			if !ok {
				continue
			}
			for _, observed := range update.endpoint.observedSettings() {
				if observed.Area() != key.Area() {
					t.Errorf("%q observed at the update endpoint of %q is in area %v, want %v", observed, key, observed.Area(), key.Area())
				}
			}
		}
	})

	t.Run("undefined update endpoint panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("observedSettings() did not panic for an undefined update endpoint")
			}
		}()
		_ = updateEndpoint(0).observedSettings()
	})
}
