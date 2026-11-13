# GitHub Actions 権限 API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for GitHub Actions permissions」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、Actions permissions・Selected actions・Workflow permissions の取得・更新に必要な権限、HTTP 応答、フィールドです。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が、リポジトリの名前と ID は [GitHub リポジトリ API 仕様](repositories.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

本ドキュメントのエンドポイントでは、OAuth app のトークンと personal access token (classic) に `repo` scope が必要です。fine-grained personal access token に必要な repository permissions は次のとおりです。Actions permissions 系の設定を変更できるトークン所有者のロールは、リポジトリロールの一覧に記載がありません。

| 用途 | メソッド・パス | Fine-grained token permissions |
|---|---|---|
| Actions permissions の取得 | `GET /repos/{owner}/{repo}/actions/permissions` | Administration: read |
| Actions permissions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions` | Administration: write |
| Selected actions の取得 | `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` | Administration: read |
| Selected actions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` | Administration: write |
| Workflow permissions の取得 | `GET /repos/{owner}/{repo}/actions/permissions/workflow` | Administration: read |
| Workflow permissions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions/workflow` | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}/actions/permissions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/workflow` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/workflow` | `204` | `409` 所有 Organization によって設定の変更が妨げられた場合の競合応答 |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-administration) | External Document |
| `grounding` | [Repository roles for an organization](https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization) | External Document |

## 2. 更新操作の単位と結果の確認

本ドキュメントの更新エンドポイントは、それぞれ独立したリクエストです。Actions permissions・Selected actions・Workflow permissions を、一つのリクエストでまとめて更新するエンドポイントは公開ドキュメントにありません。

更新エンドポイントの本文フィールドの多くは任意ですが、省略したフィールドの既存値が保持されるかは、いずれのドキュメントにも記載がありません。また、どの `PUT` も本文のない `204` を返すため、更新後の値を知るには対応する `GET` で取得し直す必要があります。条件付きリクエストに対応する旨の記載は、いずれの更新エンドポイントのドキュメントにもありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |

## 3. Actions permissions

### 3.1 `GET /repos/{owner}/{repo}/actions/permissions`

- 用途：GitHub Actions の有効状態と、実行を許可する Actions・再利用可能なワークフローの範囲の取得

| 応答フィールド | 型・値 | 必須 |
|---|---|---|
| `enabled` | boolean | 必須 |
| `allowed_actions` | `all` / `local_only` / `selected` | 任意 |
| `selected_actions_url` | 文字列 | 任意 |
| `sha_pinning_required` | boolean | 任意 |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（非公開リポジトリで確認）。
>
> - Actions が有効のときの応答：`sha_pinning_required` は、必須ではないフィールドだが常に含まれていた
> - Actions が無効のときの応答：`enabled` が `false` のとき、`allowed_actions` と `selected_actions_url` は応答に含まれない。`sha_pinning_required` は保存された値によらず `false` で返る。`enabled` だけを `true` にして有効に戻すと、無効にする前の `allowed_actions` と `sha_pinning_required` が返る

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-github-actions-permissions-for-a-repository) | External Document |

### 3.2 `PUT /repos/{owner}/{repo}/actions/permissions`

- 用途：GitHub Actions の有効状態と、実行を許可する Actions・再利用可能なワークフローの範囲の設定

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `enabled` | boolean | 必須 | リポジトリで GitHub Actions を有効にするか |
| `allowed_actions` | `all` / `local_only` / `selected` | 任意 | 実行を許可する Actions・再利用可能なワークフローの範囲 |
| `sha_pinning_required` | boolean | 任意 | Actions を完全な長さのコミット SHA に固定することを必須にするか |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 省略したフィールドの保持：`sha_pinning_required` を省略して `allowed_actions` を変えると、`sha_pinning_required` は変わらない。`enabled` だけを送ると、`allowed_actions` と `sha_pinning_required` は変わらない（非公開リポジトリで確認）
> - `allowed_actions` に `selected` を送ったときの副作用：現在の値がすでに `selected` でも、4章の `github_owned_allowed` が `true` に変わる。`verified_allowed` と `patterns_allowed` は変わらない。`enabled` だけを送った場合は変わらない（公開・非公開リポジトリで確認）
> - Actions が無効の状態からの変更：`enabled: true` と `allowed_actions` を一つのリクエストで送ると、有効化とポリシーの変更が同時に反映され、省略した `sha_pinning_required` は無効にする前の値のまま保持される。無効にする前のポリシーが `selected` だった場合、`selected` を送ると4章の3項目は `github_owned_allowed` が `true` に変わるほかは残り、`all` を送ると3項目は残らない（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-github-actions-permissions-for-a-repository) | External Document |

## 4. Selected actions

Selected actions の取得・更新は、どちらもリポジトリの `allowed_actions` が `selected` であることを利用の前提とします。`allowed_actions` が `all` または `local_only` のときにこれらのエンドポイントを呼んだ場合の応答（ステータスコードや本文）は、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 前提を満たさないときの応答：`allowed_actions` が `all` または `local_only` のとき、および Actions が無効のとき、取得は `409` を返し、値は返らない。本文の `errors` は、それぞれ `All actions and workflows are allowed on this repository`、`Only local actions and workflows are allowed on this repository`、`GitHub Actions is disabled on this repository` である。`all` のときは更新も同じ `409` を返す。`local_only` のときと Actions が無効のときの更新は確認していない（非公開リポジトリで確認）
> - `selected` から別のポリシーへの切替：`all` または `local_only` を経て `selected` に戻すと、切替前の値は残らず、`github_owned_allowed` が `true`、`verified_allowed` が `false`、`patterns_allowed` が `[]` になる（非公開リポジトリで `all`、公開リポジトリで `local_only` を経由して確認）。`all` から `selected` にした直後の値も同じだった
> - Actions の無効化：無効にしてから `enabled` だけを `true` にして有効に戻すと、無効にする前の3項目の値が返る（非公開リポジトリで確認）
> - 省略したフィールドの保持：`github_owned_allowed` だけを送ると、`verified_allowed` と `patterns_allowed` は変わらない（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Set allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |

### 4.1 `GET /repos/{owner}/{repo}/actions/permissions/selected-actions`

- 用途：選択方式で許可する Actions・再利用可能なワークフローの条件の取得
- 応答フィールド：`github_owned_allowed`（boolean）、`verified_allowed`（boolean）、`patterns_allowed`（文字列の配列）。いずれも必須ではない

### 4.2 `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions`

- 用途：選択方式で許可する Actions・再利用可能なワークフローの条件の設定

| 本文フィールド | 型 | 指定条件 | 意味 |
|---|---|---|---|
| `github_owned_allowed` | boolean | 任意 | GitHub が所有する Actions を許可するか |
| `verified_allowed` | boolean | 任意 | GitHub Marketplace の認証済み作成者の Actions を許可するか |
| `patterns_allowed` | 文字列の配列 | 任意 | 許可する Actions・再利用可能なワークフローを指定する文字列パターン。ワイルドカード・タグ・SHA を使用可能 |

`patterns_allowed` の設定は **公開リポジトリにのみ適用される** と記載されています。非公開リポジトリで `patterns_allowed` を送ったときに値が保存されるか、取得時に返るか、空配列がどう扱われるかは記載がありません。

> [!NOTE]
> 非公開リポジトリでも、`patterns_allowed` に送った空でない配列と `[]` は保存され、4.1 の取得でそのまま返ることを、reco の開発者が実際の動作で確認しています。公開リポジトリでも同じです。保存された値が非公開リポジトリのワークフロー実行に適用されるかは確認していません。

## 5. Workflow permissions

### 5.1 `GET /repos/{owner}/{repo}/actions/permissions/workflow`

- 用途：ワークフロー実行時に `GITHUB_TOKEN` へ与えられる既定の権限と、GitHub Actions が Pull Request を承認するレビューを送信できるかの取得
- 応答フィールド：`default_workflow_permissions`（`read` / `write`、必須）、`can_approve_pull_request_reviews`（boolean、必須）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-default-workflow-permissions-for-a-repository) | External Document |

### 5.2 `PUT /repos/{owner}/{repo}/actions/permissions/workflow`

- 用途：5.1 で取得する項目の設定

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `default_workflow_permissions` | `read` / `write` | 任意 | ワークフロー実行時に `GITHUB_TOKEN` へ与えられる既定の権限 |
| `can_approve_pull_request_reviews` | boolean | 任意 | GitHub Actions が Pull Request を承認できるか。有効にするとセキュリティ上のリスクになり得ると記載されている |

`default_workflow_permissions` が定めるのは **既定の** 権限です。ワークフローやジョブの `permissions` キーは、この既定の権限に対してアクセスを追加・削除できます。したがって、`default_workflow_permissions` を `read` にしても、`permissions` キーで書き込み権限を与えたワークフローは書き込めます。ただし、フォークしたリポジトリについては、`permissions` キーで読み取り権限を追加・削除できるものの、通常は書き込み権限を与えられません。例外は、管理者が GitHub Actions の設定で「Send write tokens to workflows from pull requests」を選択している場合です。

`permissions` キーでいずれかの権限を指定すると、指定しなかった権限は `none` になります。

`can_approve_pull_request_reviews` について、REST API のドキュメントは承認にしか触れていません。リポジトリ設定のドキュメントは、設定画面の「Allow GitHub Actions to create and approve pull requests」という項目で、GitHub Actions による Pull Request の作成と承認を許可・禁止できると記載しています。

> [!NOTE]
> `can_approve_pull_request_reviews` が設定画面の上記の項目に対応し、Pull Request の承認だけでなく作成も対象に含むことは、reco の開発者が実際の動作で確認した事項です。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-default-workflow-permissions-for-a-repository) | External Document |
| `grounding` | [Workflow syntax for GitHub Actions: permissions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Preventing GitHub Actions from creating or approving pull requests](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#preventing-github-actions-from-creating-or-approving-pull-requests) | External Document |
