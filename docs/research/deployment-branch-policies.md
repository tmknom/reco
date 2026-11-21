# GitHub デプロイ元パターン API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for deployment branch policies」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、Environment ごとに、デプロイできるブランチとタグを名前のパターンで指定する設定（以下、デプロイ元パターン）の一覧・取得・作成・削除に必要な権限、HTTP 応答、フィールドです。Environment そのものと、デプロイ元の制限の種類（`deployment_branch_policy`）は [GitHub デプロイ環境 API 仕様](deployment-environments.md) が、すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

一覧と取得は、リポジトリの read 権限を持つ利用者なら使えると記載されています。OAuth app のトークンと personal access token (classic) は、一覧と取得を非公開リポジトリに使うとき、作成と削除にはリポジトリによらず、`repo` scope が必要です。GitHub App の権限の一覧では、一覧と取得が Actions の read、作成と削除が Administration の write に分類されています。

| 用途 | メソッド・パス | GitHub App の権限 |
|---|---|---|
| デプロイ元パターンの一覧 | `GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` | Actions: read |
| デプロイ元パターンの取得 | `GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies/{branch_policy_id}` | Actions: read |
| デプロイ元パターンの作成 | `POST /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` | Administration: write |
| デプロイ元パターンの削除 | `DELETE /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies/{branch_policy_id}` | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET .../deployment-branch-policies` | `200` | 記載なし |
| `GET .../deployment-branch-policies/{branch_policy_id}` | `200` | 記載なし |
| `POST .../deployment-branch-policies` | `200` | `303` 同じ名前のパターンがすでにある場合、`404` Not Found または Environment の `deployment_branch_policy.custom_branch_policies` が `false` の場合 |
| `DELETE .../deployment-branch-policies/{branch_policy_id}` | `204` | 記載なし |

パスの `environment_name` は、[GitHub デプロイ環境 API 仕様](deployment-environments.md) 2章と同じく URL エンコードして渡します。ドキュメントは、これらのエンドポイントを使うには Environment の `deployment_branch_policy.custom_branch_policies` が `true` でなければならないと記載しています。`true` でないときの応答が記載されているのは作成だけです。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つ personal access token (classic) で実際の動作を確認した事項です（公開リポジトリで確認）。
>
> - `custom_branch_policies` が `true` でないとき：`deployment_branch_policy` が `null` のときも `protected_branches: true` のときも、一覧、取得、作成はどれも `404` を返した。本文は `message` が `Not Found` の Basic Error だった。作成の `404` のあと、Environment の `deployment_branch_policy` は変わっていなかった
> - 名前に `/` を含む Environment：`%2F` にエンコードした Environment の名前で、一覧と作成ができた
> - 名前に `..` と `/` を含む Environment：`reco-check-a/../staging` を `reco-check-a%2F..%2Fstaging` にエンコードした一覧、作成、削除は、その名前の Environment のパターンにだけ作用した。同じリポジトリの `staging` の Environment のパターンは変わらず、この経路で `staging` のパターンの `id` を削除すると `404` を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for deployment branch policies](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2026-03-10) | External Document |

## 2. パターンの名前と種類

パターンは、ブランチかタグかの種類（`type`）と、名前のパターン（`name`）を持ちます。ドキュメントは、ワイルドカードは `/` に一致しないこと、構文は Ruby の `File.fnmatch` に従うことを記載しています。デプロイの資料は、パターンはワークフロー実行の `GITHUB_REF` と照合されること、ブランチとタグのパターンはそれぞれ別に構成することを記載しています。名前の大文字と小文字を区別するか、同じ名前のブランチとタグのパターンを両方持てるかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 同じ名前のブランチとタグ：`main` のブランチのパターンと `main` のタグのパターンは、別々のパターンとして作成でき、一覧と取得の `type` で区別できた
> - 大文字と小文字：`main` のブランチのパターンがあるところに `MAIN` のブランチのパターンを作成すると `200` を返し、別のパターンとして作られた。名前の大文字と小文字は区別される

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a deployment branch policy](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#create-a-deployment-branch-policy) | External Document |
| `grounding` | [Deployments and environments: Deployment branches and tags](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments#deployment-branches-and-tags) | External Document |

## 3. デプロイ元パターンの一覧と取得

### 3.1 `GET .../deployment-branch-policies`

- 用途：Environment のデプロイ元パターンの一覧の取得
- クエリパラメータ：`per_page`（既定 `30`、最大 `100`）、`page`（既定 `1`）

| 応答フィールド | 型 | 必須 |
|---|---|---|
| `total_count` | integer | 必須 |
| `branch_policies` | 3.2 のパターンの配列 | 必須 |

要素の並び順と、ページの終わりの判定方法は記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで、パターンが0〜4件の状態で確認）。
>
> - ページング：`per_page=1` では各ページが1件を返し、`total_count` は全件数だった。応答ヘッダー `Link` は [GitHub デプロイ環境 API 仕様](deployment-environments.md) 3.1 の一覧と同じく `/repositories/{repository_id}/...` の形の URL を持ち、最後のページには `next` がなかった
> - 並び順：名前の昇順だった（`MAIN`、`main`、`v*` の順）。名前が同じブランチとタグのパターンでは、タグが先に並んだ（ブランチを先に作った1例）
> - パターンがないとき：`custom_branch_policies` が `true` でパターンがない Environment では、`200` で `total_count: 0` と空の配列を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [List deployment branch policies](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#list-deployment-branch-policies) | External Document |

### 3.2 `GET .../deployment-branch-policies/{branch_policy_id}`

- 用途：一つのデプロイ元パターンの取得

| 応答フィールド | 型・値 | 必須 |
|---|---|---|
| `id` | integer | 任意 |
| `node_id` | 文字列 | 任意 |
| `name` | 文字列 | 任意 |
| `type` | `branch` / `tag` | 任意 |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - フィールド：一覧の要素と取得の本文は、どちらも `id`、`node_id`、`name`、`type` を含んだ

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a deployment branch policy](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#get-a-deployment-branch-policy) | External Document |

## 4. デプロイ元パターンの作成と削除

### 4.1 `POST .../deployment-branch-policies`

- 用途：Environment へのデプロイ元パターンの追加

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `name` | 文字列 | 必須 | ブランチまたはタグが、Environment にデプロイするために一致しなければならない名前のパターン |
| `type` | `branch` / `tag` | 任意 | パターンがブランチとタグのどちらを対象にするか |

応答の本文は 3.2 と同じ形です。`type` を省略したときの扱いと、`303` の応答の本文とヘッダーは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認。作成の `200` は非公開リポジトリでも確認）。
>
> - 作成の応答：`200` を返し、本文は作ったパターンの `id`、`node_id`、`name`、`type` だった
> - `type` の省略：`type` を省略すると `branch` のパターンとして作られた
> - 同じ名前と種類のパターン：すでにある名前と種類で作成すると `303` を返し、本文は空だった。`Location` は `https://api.github.com/repositories/{repository_id}/environments/{environment_name}/deployment-branch-policies` で、一覧の URL だった。新しいパターンは作られず、一覧の件数は変わらなかった
> - 作り直したときの `id`：パターンを削除して同じ名前と種類で作り直すと、別の `id` になった
> - 空白と制御文字を含む名前：`release `（末尾に半角空白）のブランチ、` release`（先頭に半角空白）のタグ、`rel ease`（途中に半角空白）のブランチの作成は、どれも `422` を返し、本文の `message` は `Name is invalid` だった。パターンは作られなかった。途中に U+0000、U+0001、U+0008、タブ、改行、U+000B、U+000C、CR、U+001B、U+001F を含む名前も、同じ `422` だった。一方、U+007F、U+0080、U+0085、U+009F、U+00A0、U+2003、U+2028、U+200B、U+3000 を途中に含む名前と、U+00A0、U+3000 を先頭か末尾に持つ名前、U+0085 を先頭に持つ名前は `200` を返し、送ったとおりの名前で作られた
> - 作成直後の取得：作成（約20回）の約1秒以内の一覧と取得は、どれも作成したパターンを含んだ

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a deployment branch policy](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#create-a-deployment-branch-policy) | External Document |

### 4.2 `DELETE .../deployment-branch-policies/{branch_policy_id}`

- 用途：Environment からのデプロイ元パターンの削除

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認。削除の `204` は非公開リポジトリでも確認）。
>
> - 削除の応答：`204` を返し、本文はなかった。直後の一覧は、削除したパターンを含まなかった
> - 削除済みのパターン：同じ `id` への2回目の削除は `404` を返した
> - Environment のほかのフィールド：`wait_timer` の保護ルールを持ち `can_admins_bypass` が `false` の Environment で、パターンを作成したあとも削除したあとも、Environment の個別取得の本文はパターンを作成する前と一致した（`updated_at`、保護ルールの `id` を含む）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Delete a deployment branch policy](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#delete-a-deployment-branch-policy) | External Document |
