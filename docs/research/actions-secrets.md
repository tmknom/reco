# GitHub Actions シークレット API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for GitHub Actions Secrets」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、Environment ごとのシークレット（Environment Secrets）の一覧に必要な権限、HTTP 応答、フィールド、名前の規則です。シークレットの作成・更新・削除と、リポジトリと Organization のシークレットは扱いません。Environment そのものは [GitHub デプロイ環境 API 仕様](deployment-environments.md) が、すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

シークレットの作成・更新・読み取りには、認証した利用者がリポジトリの collaborator であることが必要と記載されています。OAuth app のトークンと personal access token (classic) には `repo` scope が必要です。GitHub App と fine-grained personal access token の権限の一覧では、Environment Secrets の一覧は Environments の read に分類されています。

| 用途 | メソッド・パス | GitHub App の権限 |
|---|---|---|
| Environment Secrets の一覧 | `GET /repos/{owner}/{repo}/environments/{environment_name}/secrets` | Environments: read |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}/environments/{environment_name}/secrets` | `200` | 記載なし |

パスの `environment_name` は URL エンコードして渡すと記載されています（`/` は `%2F`）。Environment が存在しないとき、権限がないときの応答は記載がありません。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つ personal access token (classic) で実際の動作を確認した事項です（公開リポジトリと、非公開リポジトリで確認）。
>
> - 存在しない Environment：削除した Environment の名前への一覧は `404` を返し、本文は `message` が `Not Found` の Basic Error だった
> - 非公開リポジトリ：利用者によると GitHub Free のアカウントが所有する非公開リポジトリで、シークレットのない Environment への一覧は `200` を返し、`{"total_count":0,"secrets":[]}` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions Secrets](https://docs.github.com/en/rest/actions/secrets?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10) | External Document |

## 2. シークレットの名前と件数

シークレットの名前は、英数字（`[a-z]`、`[A-Z]`、`[0-9]`）と `_` だけからなり、空白を含まないと記載されています。参照するときは大文字と小文字を区別せず、GitHub は入力の大文字小文字によらず名前を大文字で保存すると記載されています。名前は、作られたリポジトリ、Organization、Enterprise の中で一意でなければなりません。一つの Environment に置けるシークレットは100件まで、一つのシークレットの大きさは 48 KB までと記載されています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 小文字の名前：`RECO_A` と `reco_b` を GitHub CLI の `gh secret set --env` で登録すると、一覧には `RECO_A` と `RECO_B` として現れた

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Secrets reference](https://docs.github.com/en/actions/reference/security/secrets) | External Document |

## 3. Environment Secrets の一覧

### 3.1 `GET /repos/{owner}/{repo}/environments/{environment_name}/secrets`

- 用途：Environment で利用できるすべてのシークレットを、暗号化した値を示さずに一覧する
- クエリパラメータ：`per_page`（既定 `30`、最大 `100`）、`page`（既定 `1`）

| 応答フィールド | 型 | 必須 |
|---|---|---|
| `total_count` | integer | 必須 |
| `secrets` | 次の表のシークレットの配列 | 必須 |

| シークレットのフィールド | 型 | 必須 |
|---|---|---|
| `name` | 文字列 | 必須 |
| `created_at` | 文字列（日時） | 必須 |
| `updated_at` | 文字列（日時） | 必須 |

一覧の要素が Environment のシークレットをすべて含むか、要素の並び順、ページの終わりの判定方法、`updated_at` がいつ変わるかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認。一部は API バージョンを指定せずに確認）。
>
> - 要素のフィールド：シークレットを1件持つ Environment の一覧は `{"total_count":1,"secrets":[...]}` で、要素は `name`、`created_at`、`updated_at` だけを持ち、値を含まなかった。シークレットのない Environment の一覧は `{"total_count":0,"secrets":[]}` だった
> - ページング：シークレットを2件持つ Environment で、`per_page=1` の1ページ目と2ページ目はそれぞれ1件を返し、3ページ目は空の配列を返した。`total_count` はどのページも `2` だった
> - 登録し直し：登録済みのシークレットを同じ名前、別の値で登録し直すと、`created_at` は変わらず、`updated_at` が新しい日時に変わった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [List environment secrets](https://docs.github.com/en/rest/actions/secrets?apiVersion=2026-03-10#list-environment-secrets) | External Document |
