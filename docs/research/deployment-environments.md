# GitHub デプロイ環境 API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for deployment environments」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、Environment の一覧・取得・作成と更新に必要な権限、HTTP 応答、フィールド、名前の扱いです。Environment ごとに、デプロイできるブランチとタグを名前のパターンで指定する設定（deployment branch policies。以下、デプロイ元パターン）は [GitHub デプロイ元パターン API 仕様](deployment-branch-policies.md) が、すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

一覧と取得は、リポジトリの read 権限を持つ利用者なら使えると記載されています。OAuth app のトークンと personal access token (classic) は、一覧と取得を非公開リポジトリに使うとき、作成と更新にはリポジトリによらず、`repo` scope が必要です。GitHub App の権限の一覧では、一覧と取得が Actions の read、作成と更新が Administration の write に分類されています。

| 用途 | メソッド・パス | GitHub App の権限 |
|---|---|---|
| Environment の一覧 | `GET /repos/{owner}/{repo}/environments` | Actions: read |
| Environment の取得 | `GET /repos/{owner}/{repo}/environments/{environment_name}` | Actions: read |
| Environment の作成と更新 | `PUT /repos/{owner}/{repo}/environments/{environment_name}` | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}/environments` | `200` | 記載なし |
| `GET /repos/{owner}/{repo}/environments/{environment_name}` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/environments/{environment_name}` | `200` | `422` 名前が不正な場合、または `deployment_branch_policy` の `protected_branches` と `custom_branch_policies` に同じ値を送った場合の検証エラー |

存在しない名前への取得の応答は、記載がありません。`PUT` が作成と更新のどちらでも同じ `200` を返すため、応答のステータスからは作成したのか既存の Environment を更新したのかを区別できません。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つ personal access token (classic) で実際の動作を確認した事項です（公開・非公開リポジトリで確認）。
>
> - 存在しない名前への取得：`404` を返す。本文は `message` が `Not Found` の Basic Error で、存在しないリポジトリへの一覧の `404` と、`documentation_url` のほかは同じだった
> - 権限のないリポジトリへの取得：トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリ（`cli/cli`）では、一覧と、存在する名前への取得が `200` を返し、`protection_rules` の `reviewers` も含まれた。存在しない名前への取得は、上と同じ `404` だった。権限の不足による拒否は起きなかった
> - 作成と更新の応答：作成のときも既存の Environment の更新のときも `200` を返し、本文は直後の取得と同じだった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for deployment environments](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2026-03-10) | External Document |

### 1.1 利用できる料金プランとリポジトリ

REST API のドキュメントは、Environment、Environment のシークレット、デプロイ保護ルールは、現行のすべての料金プランで公開リポジトリに利用できると記載しています。非公開・internal のリポジトリで Environment、Environment のシークレット、デプロイ元の制限を使うには、GitHub Pro、GitHub Team、GitHub Enterprise が必要とされ、GitHub Free、GitHub Pro、GitHub Team では、待機時間や必須レビュアーなどのほかの保護ルールは公開リポジトリにだけ利用できるとされています。Environment の管理のドキュメントも、GitHub Free の利用者は公開リポジトリにだけ Environment を構成できると記載しています。利用できないリポジトリに要求を送ったときの応答は、記載がありません。

> [!NOTE]
> 次は、reco の開発者が個人アカウント所有の非公開リポジトリで実際の動作を確認した事項です。利用者によると、アカウントは GitHub Free を使っている。API の応答からは料金プランを確かめていない。
>
> - Environment の一覧と取得は `200` を返し、作成の `PUT` も `200` で Environment が作られた。`deployment_branch_policy` を `custom_branch_policies: true` にする更新も `200` で反映され、デプロイ元パターンの作成もできた。上の記載と異なり、利用できないことを示す応答は返らなかった
> - `wait_timer` だけ、または `reviewers` だけを送る `PUT` は `422` を返した。本文の `message` は、それぞれ `Failed to create the environment protection rule. Please ensure the billing plan supports the wait timer protection rule.` と、同じ文の `wait timer` を `required reviewers` にしたものだった。直後の取得で、既存の `deployment_branch_policy` は変わっていなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for deployment environments: About deployment environments](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#about-deployment-environments) | External Document |
| `grounding` | [Managing environments for deployment](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments) | External Document |
| `grounding` | [Deployments and environments](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments) | External Document |

## 2. Environment の名前

Environment の名前は、パスパラメータ `environment_name` に URL エンコードして渡すと記載されています。例として、名前に含まれる `/` は `%2F` に置き換えるとされています。Environment の管理のドキュメントは、Environment の名前は大文字と小文字を区別せず、255 文字を超えられず、リポジトリの中で一意でなければならないと記載しています。名前に使える文字は記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 大文字と小文字：`reco-check-case` を作成したあと、大文字に変えた名前への取得は同じ `id` の Environment を返し、`name` は作成したときの綴りのままだった。大文字に変えた名前への `PUT` も、本文の有無によらず同じ Environment を更新し、新しい Environment は作られなかった
> - 空白と `/`：`%20` と `%2F` にエンコードした名前で、作成と取得ができ、`name` は空白と `/` を含む元の名前で返った。`/` をエンコードせずに送った取得は `404` を返し、`documentation_url` は Environment の取得の節ではなく `https://docs.github.com/rest` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for deployment environments: Get an environment](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#get-an-environment) | External Document |
| `grounding` | [Managing environments for deployment: Creating an environment](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments#creating-an-environment) | External Document |

## 3. Environment の一覧と取得

### 3.1 `GET /repos/{owner}/{repo}/environments`

- 用途：リポジトリの Environment の一覧の取得
- クエリパラメータ：`per_page`（既定 `30`、最大 `100`）、`page`（既定 `1`）

| 応答フィールド | 型 | 必須 |
|---|---|---|
| `total_count` | integer | 任意 |
| `environments` | 3.2 の Environment の配列 | 任意 |

一覧の要素が Environment をすべて含むか、要素の並び順、ページの終わりの判定方法は、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで、Environment が1〜5件の状態で確認）。
>
> - `total_count`：ページの大きさによらず、Environment の全件数を返した。`per_page` を省略したときの要素数と一致した
> - ページング：`per_page=1` では各ページが1件を返した。応答ヘッダー `Link` は `next`、`last`、`prev`、`first` を含み、最後のページには `next` がなかった。`Link` の URL は `https://api.github.com/repositories/{repository_id}/environments?per_page=1&page=2` の形で、要求に使った `/repos/{owner}/{repo}` の形ではなかった。最後のページより後のページへの要求は `200` を返し、`environments` は空の配列、`total_count` は全件数だった
> - 並び順：名前の昇順だった
> - 一覧の要素と取得の内容：一覧の要素は、同じ時点の 3.2 の取得の本文と全フィールドが一致した（3件で比較）
> - 作成と削除の直後の一覧：4.1 の作成と、Environment の削除（`DELETE /repos/{owner}/{repo}/environments/{environment_name}`）の直後（約1秒以内）の一覧は、作成した Environment を含み、削除した Environment を含まなかった（それぞれ数回）。確かめた範囲で、一覧は存在する Environment をすべて含んだ

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [List environments](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#list-environments) | External Document |

### 3.2 `GET /repos/{owner}/{repo}/environments/{environment_name}`

- 用途：一つの Environment の取得

| 応答フィールド | 型・値 | 必須 |
|---|---|---|
| `id` | integer（int64） | 必須 |
| `node_id` | 文字列 | 必須 |
| `name` | 文字列 | 必須 |
| `url`、`html_url` | 文字列 | 必須 |
| `created_at`、`updated_at` | 文字列（日時） | 必須 |
| `protection_rules` | 保護ルールの配列。要素は次の表のいずれかの形 | 任意 |
| `deployment_branch_policy` | object または null。`protected_branches`（boolean、必須）と `custom_branch_policies`（boolean、必須）を持つ | 任意 |

| 保護ルールの形 | フィールド |
|---|---|
| 待機時間を持つ形 | `id`（必須）、`node_id`（必須）、`type`（文字列、必須）、`wait_timer`（integer） |
| レビュアーを持つ形 | `id`（必須）、`node_id`（必須）、`type`（文字列、必須）、`prevent_self_review`（boolean）、`reviewers`（`type`（`User` / `Team`）と `reviewer`（ユーザーまたはチーム）の組の配列） |
| ほかのフィールドを持たない形 | `id`（必須）、`node_id`（必須）、`type`（文字列、必須） |

保護ルールの `type` が取り得る値は、スキーマに列挙がありません。`deployment_branch_policy` の値とデプロイ元の制限の対応は、4.1 の本文フィールドの説明から読み取れます（`null` はすべてのブランチを許可する）。`id` の性質（作り直したときに変わるか）は記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - `can_admins_bypass`：スキーマにないフィールド `can_admins_bypass`（boolean）が、一覧の要素と取得の本文に常に含まれた
> - 保護ルールの `type`：待機時間の形は `wait_timer`、レビュアーの形は `required_reviewers`、デプロイ元の制限は `branch_policy`（ほかのフィールドを持たない形）だった。`branch_policy` の要素は、`deployment_branch_policy` が `null` でないときに限り含まれた
> - `deployment_branch_policy` の値：すべてのブランチを許可するときは `null`、保護ブランチに限るときは `protected_branches: true`、`custom_branch_policies: false`、指定したパターンに限るときは `protected_branches: false`、`custom_branch_policies: true` だった
> - `id`：同じ名前の Environment を削除して作り直すと、作り直すたびに別の `id` になった（2回の作り直しで確認）。デプロイ元の制限を別の値に変えると、`branch_policy` の保護ルールの `id` が変わった
> - レビュアーの形：`reviewers` に `type: User` とユーザーの `id` を送ると、`reviewers` の要素は `type: User` と、そのユーザーを `reviewer` に持つ形で返った。`reviewer.id` は送った `id` と同じだった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get an environment](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#get-an-environment) | External Document |

## 4. Environment の作成と更新

### 4.1 `PUT /repos/{owner}/{repo}/environments/{environment_name}`

- 用途：Environment の作成、または既存の Environment の保護ルールとデプロイ元の制限の更新

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `wait_timer` | integer | 任意 | ジョブが起動されてから実行を遅らせる時間（分）。0 から 43,200 の整数 |
| `prevent_self_review` | boolean | 任意 | ジョブを起動した利用者が自身のジョブを承認することを防ぐか |
| `reviewers` | object の配列または null | 任意 | ジョブを審査できる利用者またはチーム（6件まで）。要素は `type`（`User` / `Team`）と `id` |
| `deployment_branch_policy` | object または null | 任意 | デプロイ元の制限の種類。`null` はすべてのブランチを許可する。`protected_branches` と `custom_branch_policies`（どちらも boolean、必須）は、一方が `true` なら他方は `false` でなければならない |

本文はすべて任意で、省略したフィールドの既存値が保持されるかは記載がありません。本文のない `PUT` の扱いも記載がありません。`protected_branches` は保護ルールを持つブランチだけをデプロイ元とするか、`custom_branch_policies` は指定した名前のパターンに一致するブランチだけをデプロイ元とするかを表します。パターンそのものは、このエンドポイントでは送れず、[GitHub デプロイ元パターン API 仕様](deployment-branch-policies.md) のエンドポイントで作成と削除をします。

デプロイの資料は、設定画面の「Deployment branches」の選択肢として、制限のない「No restriction」、保護ルールを持つブランチだけの「Protected branches only」、指定した名前のパターンに一致するブランチとタグだけの「Selected branches and tags」を挙げています。それぞれ `deployment_branch_policy` の `null`、`protected_branches: true`、`custom_branch_policies: true` に当たります。また、設定画面の「Allow administrators to bypass configured protection rules」で、管理者が保護ルールを迂回してデプロイできるかを選べ、既定では迂回できると記載しています。この選択に当たる本文フィールドは、REST API のドキュメントに記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 本文のない作成：存在しない名前への本文のない `PUT` は `200` を返して Environment を作った。作られた Environment は、`protection_rules` が空の配列、`deployment_branch_policy` が `null`、`can_admins_bypass` が `true` だった
> - 本文つきの作成：存在しない名前に `wait_timer: 3` だけを送ると `200` を返し、作られた Environment は `wait_timer` の保護ルールを持った。`deployment_branch_policy` を送る作成は確かめていない
> - 既存の Environment への本文のない `PUT` と `{}` の `PUT`：`200` を返し、値は何も変わらなかった。`wait_timer`、`reviewers`、`prevent_self_review: true`、`can_admins_bypass: false`、`protected_branches: true` を持つ Environment で確かめた
> - `can_admins_bypass`：本文に `can_admins_bypass: false` を送ると受け付けられ、取得が `false` を返した
> - 省略したフィールドの保持：`wait_timer: 7`、`reviewers`（ユーザー1人）、`prevent_self_review: true`、`can_admins_bypass: false` を持つ Environment に `deployment_branch_policy` だけを送ると、4つの値と、待機時間とレビュアーの保護ルールの `id` は変わらなかった。`custom_branch_policies: true` で4件のパターンを持つ Environment に `wait_timer` だけを送ると、`deployment_branch_policy` と、4件のパターン（同じ `id`）は変わらなかった
> - デプロイ元の制限を変える更新：`custom_branch_policies: true` から `protected_branches: true` または `null` に変えると、パターンはすべて失われ、`custom_branch_policies: true` に戻してもパターンの一覧は空だった。`custom_branch_policies: true` のまま同じ値を送り直したときは、パターンは同じ `id` で残った。パターンのエンドポイントの応答は [GitHub デプロイ元パターン API 仕様](deployment-branch-policies.md) の注釈にある
> - 同じ値の `protected_branches` と `custom_branch_policies`：両方 `true`、両方 `false` のどちらも `422` を返し、本文の `message` は `"custom_branch_policies" and "protected_branches" cannot have the same value` だった。値は変わらなかった
> - 更新直後の取得：作成と更新（約20回）の約1秒以内の取得は、どれも更新後の値を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create or update an environment](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#create-or-update-an-environment) | External Document |
| `grounding` | [Deployments and environments: Deployment protection rules](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments#deployment-protection-rules) | External Document |
