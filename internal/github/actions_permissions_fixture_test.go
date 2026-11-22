package github

// actionsDisabledBody は Actions が無効のときの応答で、確認した動作のとおり allowed_actions を含まず sha_pinning_required が false である。
const actionsDisabledBody = `{"enabled": false, "sha_pinning_required": false}`

// actionsPermissionsBody は GET .../actions/permissions の Actions が有効のときの応答に似せ、reco が読まないフィールドも含める。
func actionsPermissionsBody(policy string) string {
	return `{"enabled": true, "allowed_actions": "` + policy + `", "selected_actions_url": "https://api.github.com/repos/owner/repo/actions/permissions/selected-actions", "sha_pinning_required": true}`
}
