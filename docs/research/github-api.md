# GitHub API 仕様

## 1. 目的と適用範囲

本ドキュメントは、reco が利用する GitHub.com REST API の外部契約を、GitHub の公開ドキュメントから抜粋して整理します。対象は、リポジトリの一般設定と GitHub Actions の権限設定を取得・更新するエンドポイントと、それらに共通する認証、API バージョン、リダイレクト、失敗応答、レート制限、条件付きリクエストです。

GitHub API の契約の正本は、GitHub の公開ドキュメントです。本ドキュメントは、開発のたびに公開ドキュメントを参照しなくて済むよう、必要な事実を一か所に集めます。reco の要求・仕様・設計の文書は GitHub API の事実を本ドキュメントから参照し、本ドキュメントは GitHub の公開ドキュメント以外に依存しません。事実を reco でどう扱うか（管理範囲、入力検証、更新の安全性、停止条件など）は、参照する側の文書が定めます。

記述は、GitHub REST API `2026-03-10` 版と、版を持たない REST API ガイドの公開ドキュメントを、2026-11-09 に確認した内容に基づきます。各セクションの根拠は、そのセクションの Relationship テーブルに示します。公開ドキュメントに記載のない挙動は「記載がない」と明示し、GitHub の保証として扱いません。公開ドキュメントの記載が不十分で、reco の開発者が実際の動作を確認した事項に限り、`> [!NOTE]` の注釈で公開ドキュメントの記載と区別して示します。

## 2. REST API の共通仕様

### 2.1 リクエストの URL・ヘッダー・パスパラメータ

| 項目 | 公開仕様 |
|---|---|
| ベース URL | `https://api.github.com`。エンドポイントのパスをこれに続けて完全な URL とする |
| `Authorization` | 多くの場合 `Bearer <token>` と `token <token>` のどちらも使用可能。JSON Web Token (JWT) を渡すときは `Bearer` が必須 |
| `Accept` | 多くのエンドポイントは `application/vnd.github+json` の指定を求める |
| `User-Agent` | すべてのリクエストで有効な値が必須。指定のないリクエストは拒否され、無効な値には `403 Forbidden` が返る |
| `X-GitHub-Api-Version` | 利用する API バージョンを指定する。省略時は `2022-11-28` 版が使われる |
| パスの `{owner}` | リポジトリ所有者のアカウント名。大文字小文字を区別しない |
| パスの `{repo}` | `.git` 拡張子を除くリポジトリ名。大文字小文字を区別しない |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Getting started with the REST API: Make a curl request](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#4-make-a-curl-request) | External Document |
| `grounding` | [Getting started with the REST API: Headers](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#headers) | External Document |
| `grounding` | [API Versions: Specifying an API version](https://docs.github.com/en/rest/about-the-rest-api/api-versions#specifying-an-api-version) | External Document |
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |

### 2.2 API バージョン

サポート対象の API バージョンは、`2026-03-10`（サポート終了日は未定）と `2022-11-28`（2028-03-10 にサポート終了）です。新しいバージョンのリリース後も、以前のバージョンは少なくとも24か月サポートされます。サポートが終了したバージョンを指定すると `410 Gone` が返ります。バージョン間の破壊的変更は、GitHub の破壊的変更の一覧に記載されます。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [API Versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions) | External Document |
| `grounding` | [Breaking changes](https://docs.github.com/en/rest/about-the-rest-api/breaking-changes) | External Document |

### 2.3 リダイレクト

GitHub は、どのリクエストもリダイレクトになり得ると想定し、リダイレクトに追従するよう推奨しています。`301` は恒久的な移動を表し、`location` ヘッダーの URL へリクエストを繰り返したうえで、以後もその URL を使うよう求めます。`302` と `307` は一時的な移動を表し、`location` ヘッダーの URL へリクエストを繰り返すものの、以後に使う URL は変えないよう求めます。ほかのリダイレクト用ステータスコードが HTTP 仕様に従って使われる場合もあります。

一方、公開ドキュメントには、リダイレクト先へ認証情報を送ってよいか、更新系メソッド（`PATCH`・`PUT` など）をリダイレクト先へ再送したときに何が起きるかの記載がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Best practices for using the REST API: Follow redirects](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#follow-redirects) | External Document |

### 2.4 失敗応答の解釈

適切に認証されていないリクエストが非公開リソースにアクセスすると、GitHub は非公開リポジトリの存在を確認させないため、`403 Forbidden` ではなく `404 Not Found` を返します。エンドポイントが対応していない HTTP メソッドを使った場合も、`405` ではなく `404` が返ります。したがって、`404` だけでは対象の不存在を断定できません。

認証の失敗は、状況によって異なるステータスコードで報告されます。無効な認証情報を使うと、最初は `401 Unauthorized` が返ります。トークンを付けない場合や、トークンの権限が不足している場合は、`404 Not Found` または `403 Forbidden` が返ります。短時間に無効な認証情報で繰り返しリクエストすると、GitHub はそのユーザーの認証を有効な認証情報も含めて一時的にすべて拒否し、`403 Forbidden` を返します。

GitHub は、存在するはずのリソースに `404` が返るときの確認事項を、トークン種別ごとに挙げています。主な項目は、トークンの scope と権限、トークン所有者の権限、アクセスできるリポジトリの範囲、トークンの有効期限と失効です。

GitHub App や fine-grained personal access token で「Resource not accessible by integration」や「Resource not accessible by personal access token」のエラーを受けたときは、応答の `X-Accepted-GitHub-Permissions` ヘッダーで必要な権限を確認できます。

各エンドポイントのドキュメントに列挙された応答コードには、共通仕様によって返るコードが含まれていない場合があります。たとえば、レート制限の超過（2.5）では `403` または `429` が返りますが、3.1 の各エンドポイントの応答コード一覧に `429` はありません。認証失敗の `401` も同様です。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Authenticating to the REST API: Failed login limit](https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api#failed-login-limit) | External Document |
| `grounding` | [Troubleshooting the REST API: 404 Not Found for an existing resource](https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api#404-not-found-for-an-existing-resource) | External Document |
| `grounding` | [Troubleshooting the REST API: Resource not accessible](https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api#resource-not-accessible) | External Document |

### 2.5 レート制限

GitHub REST API には、プライマリとセカンダリのレート制限があります。どちらの超過も `403` または `429` で報告されるため、ステータスコードだけでは区別できません。

| 応答ヘッダー | 意味 |
|---|---|
| `x-ratelimit-remaining` | 現在のレート制限枠で残っているリクエスト数 |
| `x-ratelimit-reset` | 現在のレート制限枠がリセットされる時刻（UTC エポック秒） |
| `retry-after` | 再試行までに待つべき秒数 |

プライマリ制限を超過すると `x-ratelimit-remaining` が `0` になり、`x-ratelimit-reset` の時刻まで再試行を控えるよう求められます。セカンダリ制限を超過したときは、次の順で待機時間を決めるよう推奨されています。

- `retry-after` があれば、その秒数が経過するまで再試行しない
- `x-ratelimit-remaining` が `0` なら、`x-ratelimit-reset` の時刻まで再試行しない
- それ以外は、少なくとも1分待ってから再試行する
- 再試行が失敗し続けるなら、再試行の間隔を指数的に延ばす

レート制限中にリクエストを続けると、インテグレーションが利用禁止になる場合があります。GitHub はセカンダリ制限を避けるため、リクエストを並行させず逐次に送ること、大量の `POST`・`PATCH`・`PUT`・`DELETE` を送るときは各リクエストの間に少なくとも1秒空けることを推奨しています。これらの推奨は、失敗した更新系リクエストを再送してよいかについては述べていません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Rate limits for the REST API: Checking the status of your rate limit](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#checking-the-status-of-your-rate-limit) | External Document |
| `grounding` | [Rate limits for the REST API: Exceeding the rate limit](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#exceeding-the-rate-limit) | External Document |
| `grounding` | [Best practices for using the REST API: Avoid concurrent requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#avoid-concurrent-requests) | External Document |
| `grounding` | [Best practices for using the REST API: Pause between mutative requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#pause-between-mutative-requests) | External Document |
| `grounding` | [Best practices for using the REST API: Handle rate limit errors appropriately](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#handle-rate-limit-errors-appropriately) | External Document |

### 2.6 条件付きリクエスト

多くのエンドポイントは `etag` ヘッダーを返し、`last-modified` ヘッダーを返すエンドポイントも多くあります。これらを使った条件付きリクエストで対象が変わっていなければ `304 Not Modified` が返ります。`304` が返ったリクエストは、`Authorization` ヘッダーで正しく認証していれば、プライマリ制限の枠を消費しません。

ただし、`POST`・`PUT`・`PATCH`・`DELETE` などの更新系メソッドの条件付きリクエストは、エンドポイントごとのドキュメントに記載がある場合を除いて **サポートされません** 。4〜7章の更新エンドポイントのドキュメントには、条件付きリクエストに対応する旨の記載がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Best practices for using the REST API: Use conditional requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#use-conditional-requests) | External Document |

## 3. エンドポイントと必要な権限

次の表は、各エンドポイントに必要な fine-grained personal access token の repository permissions です。トークン種別、所有者、リポジトリ側の条件によって、実際にアクセスできるかは変わり得ます。

| 用途 | メソッド・パス | Fine-grained token permissions |
|---|---|---|
| リポジトリと一般設定の取得 | `GET /repos/{owner}/{repo}` | Metadata: read（マージ関連設定には追加の権限が必要。表の後を参照） |
| 一般設定の更新 | `PATCH /repos/{owner}/{repo}` | Administration: write |
| Actions permissions の取得 | `GET /repos/{owner}/{repo}/actions/permissions` | Administration: read |
| Actions permissions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions` | Administration: write |
| Selected actions の取得 | `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` | Administration: read |
| Selected actions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` | Administration: write |
| Workflow permissions の取得 | `GET /repos/{owner}/{repo}/actions/permissions/workflow` | Administration: read |
| Workflow permissions の更新 | `PUT /repos/{owner}/{repo}/actions/permissions/workflow` | Administration: write |

`GET /repos/{owner}/{repo}` については、権限の一覧が Metadata: read だけを挙げる一方、エンドポイントのドキュメントは「マージ関連設定を見るには `contents:read` と `contents:write` の権限が必要」と記載しています。どのフィールドがマージ関連設定にあたるかと、権限が足りないときにそのフィールドがどう返るかは記載がありません。

Actions permissions 系のエンドポイント（5〜7章）では、OAuth app のトークンと personal access token (classic) に `repo` scope が必要です。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Metadata](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-metadata) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-administration) | External Document |
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |

### 3.1 公開ドキュメントに記載された HTTP 応答

| エンドポイント | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}` | `200` | `301` Moved permanently、`403` Forbidden、`404` Resource not found |
| `PATCH /repos/{owner}/{repo}` | `200` | `307` Temporary Redirect、`403` Forbidden、`404` Resource not found、`422` Validation failed, or the endpoint has been spammed |
| `GET /repos/{owner}/{repo}/actions/permissions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/workflow` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/workflow` | `204` | `409` 所有 Organization によって設定の変更が妨げられた場合の競合応答 |

`301` と `307` がどのような条件で返るかは、エンドポイントのドキュメントに記載がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Get GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-github-actions-permissions-for-a-repository) | External Document |
| `grounding` | [Set GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-github-actions-permissions-for-a-repository) | External Document |
| `grounding` | [Get allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Set allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Get default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-default-workflow-permissions-for-a-repository) | External Document |
| `grounding` | [Set default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-default-workflow-permissions-for-a-repository) | External Document |

## 4. リポジトリの一般設定

### 4.1 `GET /repos/{owner}/{repo}`

- 用途：リポジトリの情報と一般設定の取得
- 主な応答フィールド：`id`（整数、必須）、`name`（文字列、必須）、`full_name`（文字列、必須）、`owner`、`delete_branch_on_merge`（boolean、必須ではない）、`allow_auto_merge`（boolean、必須ではない）

パスの `{owner}` と `{repo}` は名前による指定であり、名前はリポジトリの名前変更や移転で変わります。GitHub は名前変更後も、旧名に対する Web 上の情報と Git の操作を新しい名前へリダイレクトします。ただし、旧名で新しいリポジトリを作るとこのリダイレクトは機能しなくなり、API へのリクエストがリダイレクトされるかは名前変更のドキュメントに記載がありません。応答の `id` について、スキーマには型（int64）以外の説明がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Renaming a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository) | External Document |

### 4.2 `PATCH /repos/{owner}/{repo}`

- 用途：リポジトリの一般設定の更新

| 本文フィールド | 型 | 指定条件 | ドキュメント上の既定値 |
|---|---|---|---|
| `delete_branch_on_merge` | boolean | 任意 | `false` |
| `allow_auto_merge` | boolean | 任意 | `false` |

`delete_branch_on_merge` は Pull Request のマージ時に head ブランチを自動で削除するか、`allow_auto_merge` は Pull Request の自動マージを許可するかの設定です。たとえば、`allow_auto_merge` だけを指定するリクエスト本文は次のとおりです。

```json
{
  "allow_auto_merge": true
}
```

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |

## 5. Actions permissions

### 5.1 `GET /repos/{owner}/{repo}/actions/permissions`

- 用途：GitHub Actions の有効状態と、実行を許可する Actions・再利用可能なワークフローの範囲の取得

| 応答フィールド | 型・値 | 必須 |
|---|---|---|
| `enabled` | boolean | 必須 |
| `allowed_actions` | `all` / `local_only` / `selected` | 任意 |
| `selected_actions_url` | 文字列 | 任意 |
| `sha_pinning_required` | boolean | 任意 |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-github-actions-permissions-for-a-repository) | External Document |

### 5.2 `PUT /repos/{owner}/{repo}/actions/permissions`

- 用途：GitHub Actions の有効状態と、実行を許可する Actions・再利用可能なワークフローの範囲の設定

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `enabled` | boolean | 必須 | リポジトリで GitHub Actions を有効にするか |
| `allowed_actions` | `all` / `local_only` / `selected` | 任意 | 実行を許可する Actions・再利用可能なワークフローの範囲 |
| `sha_pinning_required` | boolean | 任意 | Actions を完全な長さのコミット SHA に固定することを必須にするか |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-github-actions-permissions-for-a-repository) | External Document |

## 6. Selected actions

Selected actions の取得・更新は、どちらもリポジトリの `allowed_actions` が `selected` であることを利用の前提とします。`allowed_actions` が `all` または `local_only` のときにこれらのエンドポイントを呼んだ場合の応答（ステータスコードや本文）は、記載がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Set allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |

### 6.1 `GET /repos/{owner}/{repo}/actions/permissions/selected-actions`

- 用途：選択方式で許可する Actions・再利用可能なワークフローの条件の取得
- 応答フィールド：`github_owned_allowed`（boolean）、`verified_allowed`（boolean）、`patterns_allowed`（文字列の配列）。いずれも必須ではない

### 6.2 `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions`

- 用途：選択方式で許可する Actions・再利用可能なワークフローの条件の設定

| 本文フィールド | 型 | 指定条件 | 意味 |
|---|---|---|---|
| `github_owned_allowed` | boolean | 任意 | GitHub が所有する Actions を許可するか |
| `verified_allowed` | boolean | 任意 | GitHub Marketplace の認証済み作成者の Actions を許可するか |
| `patterns_allowed` | 文字列の配列 | 任意 | 許可する Actions・再利用可能なワークフローを指定する文字列パターン。ワイルドカード・タグ・SHA を使用可能 |

`patterns_allowed` の設定は **公開リポジトリにのみ適用される** と記載されています。非公開リポジトリで `patterns_allowed` を送ったときに値が保存されるか、取得時に返るか、空配列がどう扱われるかは記載がありません。

## 7. Workflow permissions

### 7.1 `GET /repos/{owner}/{repo}/actions/permissions/workflow`

- 用途：ワークフロー実行時に `GITHUB_TOKEN` へ与えられる既定の権限と、GitHub Actions が Pull Request を承認するレビューを送信できるかの取得
- 応答フィールド：`default_workflow_permissions`（`read` / `write`、必須）、`can_approve_pull_request_reviews`（boolean、必須）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-default-workflow-permissions-for-a-repository) | External Document |

### 7.2 `PUT /repos/{owner}/{repo}/actions/permissions/workflow`

- 用途：7.1 で取得する項目の設定

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

## 8. 更新操作の単位と結果の確認

4〜7章の更新エンドポイントは、それぞれ独立したリクエストです。Actions permissions・Selected actions・Workflow permissions を、一つのリクエストでまとめて更新するエンドポイントは公開ドキュメントにありません。

更新エンドポイントの本文フィールドの多くは任意ですが、省略したフィールドの既存値が保持されるかは、いずれのドキュメントにも記載がありません。4.2 の `delete_branch_on_merge` と `allow_auto_merge` には既定値 `false` が記載されていますが、これが省略時に適用される値なのかも記載がありません。

更新の成功応答から、更新後の設定を確定することもできません。5〜7章の `PUT` は本文のない `204` を返すため、更新後の値を知るには対応する `GET` で取得し直す必要があります。`PATCH /repos/{owner}/{repo}` は `200` でリポジトリの表現を返しますが、それが更新後の状態を反映しているかは記載がありません。また、更新系メソッドの条件付きリクエストはサポートされないため（2.6）、取得から更新までの間に他者が設定を変更したかを検出する手段は、公開ドキュメントにありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |
| `grounding` | [Best practices for using the REST API: Use conditional requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#use-conditional-requests) | External Document |
