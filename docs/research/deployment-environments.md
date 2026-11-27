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

REST API のドキュメントは、Environment、Environment のシークレット、デプロイ保護ルールは、現行のすべての料金プランで公開リポジトリに利用できると記載しています。非公開・internal のリポジトリで Environment、Environment のシークレット、デプロイ元の制限を使うには、GitHub Pro、GitHub Team、GitHub Enterprise が必要とされ、GitHub Free、GitHub Pro、GitHub Team では、待機時間や必須レビュアーなどのほかの保護ルールは公開リポジトリにだけ利用できるとされています。Environment の管理のドキュメントも、GitHub Free の利用者は公開リポジトリにだけ Environment を構成できると記載しています。利用できないリポジトリに要求を送ったときの応答は、記載がありません。同じドキュメントは、存在しない Environment を参照するワークフローを実行すると、参照した名前の Environment が作られ、その Environment は保護ルールもシークレットも持たないと記載しています。例外として、暗黙のページのビルドが作る Environment は、ソースブランチを保護ルールとして持ちます。

> [!NOTE]
> 次は、reco の開発者が個人アカウント所有の非公開リポジトリで実際の動作を確認した事項です。利用者によると、アカウントは GitHub Free を使っている。API の応答からは料金プランを確かめていない。
>
> - Environment の一覧と取得は `200` を返し、作成の `PUT` も `200` で Environment が作られた。`deployment_branch_policy` を `custom_branch_policies: true` にする更新も `200` で反映され、デプロイ元パターンの作成もできた。上の記載と異なり、利用できないことを示す応答は返らなかった
> - `wait_timer` だけ、または `reviewers` だけを送る `PUT` は `422` を返した。本文の `message` は、それぞれ `Failed to create the environment protection rule. Please ensure the billing plan supports the wait timer protection rule.` と、同じ文の `wait timer` を `required reviewers` にしたものだった。直後の取得で、既存の `deployment_branch_policy` は変わっていなかった
> - 保護ルールを外す値も同じだった：担当者を外す `reviewers: []` と `reviewers: null`、待機時間を外す `wait_timer: 0` も、保護ルールのない Environment で、上と同じ `422` を返した。`prevent_self_review` と `reviewers` を一緒に送った `PUT` は `reviewers` の `422`、`deployment_branch_policy` と `wait_timer` を一緒に送った `PUT` は `wait_timer` の `422` を返し、方式は変わらなかった
> - `prevent_self_review` だけを送る `PUT` は、`true` も `false` も、公開リポジトリで担当者がいないときと同じ `422`（4.1 の注釈）を返した
> - `can_admins_bypass: false` は `200` を返して取得に反映され、`can_admins_bypass: true` で戻せた
> - 取得は、保護ルールのない Environment で `protection_rules: []` と `can_admins_bypass` を返した

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
> - `..`：パスの要素を `%2E%2E` にした `PUT` は `404` を返し、Environment は作られなかった。同じ本文で名前を `a..` にした `PUT` は `200` を返した
> - 大文字と小文字のほかに同じ Environment とみなす違い：`reco-check-café` を作成したあと、`reco-check-cafe`（アクセントなし）への取得と本文なしの `PUT` は、どちらも同じ `id` の Environment に当たり、新しい Environment は作られなかった。`reco-check-cafÉ`（ASCII 以外の大文字）と、`é` を `e` と結合文字 U+0301 に分けた名前への取得も同じ `id` を返した。既存の `staging` に対しては、`stàging`、全角の `ｓ` で始まる名前、`ſ`（U+017F）で始まる名前、末尾に空白を付けた名前、末尾に U+0001 を付けた名前への取得が、どれも `staging` を返した
> - 別の Environment とみなした違い：`staging` に対し、先頭に空白かタブを付けた名前、内部に空白を挟んだ名前、`sta-ging`、`sta.ging`、末尾に `_`、`.`、`'`、`~` を付けた名前への取得は、どれも `404` を返した。`reco-check-018-u27` に対し、`-` を `_`、`.`、`~`、`+` に入れ替えた名前への取得も、`reco-check-018 a'b` に対し、内部の空白を二つにした名前、`'` を `` ` `` か `"` に入れ替えた名前、空白を `-` に入れ替えた名前への取得も、どれも `404` を返した
> - `%`、`?`、`#`：`reco-check-018-%?#` を `%25%3F%23` にエンコードした名前で作成と取得ができ、`name` は元の名前で返った。`reco-check-018-%` と `reco-check-018-` への取得は `404` を返した

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
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで、Environment が1〜7件の状態で確認）。
>
> - `total_count`：ページの大きさによらず、Environment の全件数を返した。`per_page` を省略したときの要素数と一致した
> - ページング：`per_page=1` では各ページが1件を返した。7件の状態では、`per_page=2` で4ページ（2、2、2、1件）、`per_page=3` で3ページ（3、3、1件）に分かれ、どのページの `total_count` も7で、ページをまたいだ名前の重複と欠けはなかった。応答ヘッダー `Link` は `next`、`last`、`prev`、`first` を含み、最後のページには `next` がなかった。`Link` の URL は `https://api.github.com/repositories/{repository_id}/environments?per_page=1&page=2` の形で、要求に使った `/repos/{owner}/{repo}` の形ではなかった。最後のページより後のページへの要求は `200` を返し、`environments` は空の配列、`total_count` は全件数だった
> - 本文の大きさ：`curl` の既定の `User-Agent` では字下げした本文を、`User-Agent` を `reco-check` にすると空白のない本文を返した。空白のない本文での一覧の要素1件は、保護ルールのない Environment で約450バイト、`branch_policy` の保護ルールだけを持つ Environment で約580バイト、レビュアー（ユーザー1人）と待機時間と `branch_policy` の保護ルールを持つ Environment で約1,700バイトだった
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
> - レビュアーの形：`reviewers` に `type: User` とユーザーの `id` を送ると、`reviewers` の要素は `type: User` と、そのユーザーを `reviewer` に持つ形で返った。`reviewer.id` は送った `id` と同じだった。`reviewer` は利用者の `login` も含んだ。`prevent_self_review` を `false` で送った場合も、`required_reviewers` の要素は `prevent_self_review: false` を含んだ
> - 保護ルールを外す更新：`wait_timer: 0` と `reviewers: []` を送ると、`wait_timer` と `required_reviewers` の要素はどちらも `protection_rules` からなくなった。値が `0` の待機時間の要素や、`reviewers` が空のレビュアーの要素は返らなかった。待機時間を一度も設定していない Environment にも、`wait_timer` の要素は含まれなかった
> - 担当者のいないレビュアーの要素：リポジトリの権限を持たない利用者だけを `reviewers` に送ると（4.1 の注釈）、`reviewers` が空の配列で `prevent_self_review: false` の `required_reviewers` の要素が返った。この要素は `reviewers: []` か `reviewers: null` を送るとなくなった。`prevent_self_review: true` の要素にリポジトリの権限を持たない利用者だけを送ると、`reviewers` が空の配列で `prevent_self_review: true` の要素が返った。この要素に担当者にできる利用者を送ると、同じ `id` で `prevent_self_review: true` のまま担当者が戻った
> - 保護ルールの `id`：担当者を入れ替える更新、`prevent_self_review` だけの更新、待機時間の値だけを変える更新では、それぞれの要素の `id` は変わらなかった。要素をなくしてから設定し直すと、別の `id` になった

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

デプロイの資料は、設定画面の「Deployment branches」の選択肢として、制限のない「No restriction」、保護ルールを持つブランチだけの「Protected branches only」、指定した名前のパターンに一致するブランチとタグだけの「Selected branches and tags」を挙げています。それぞれ `deployment_branch_policy` の `null`、`protected_branches: true`、`custom_branch_policies: true` に当たります。また、設定画面の「Allow administrators to bypass configured protection rules」で、管理者が保護ルールを迂回してデプロイできるかを選べ、既定では迂回できると記載しています。この選択に当たる本文フィールドは、REST API のドキュメントに記載がありません。必須レビュアーについては、担当者のうち1人が承認すればジョブが進み、自己承認の禁止（Prevent self-review）を有効にすると、デプロイを開始した利用者は担当者であってもそのジョブを承認できないと記載しています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 本文のない作成：存在しない名前への本文のない `PUT` は `200` を返して Environment を作った。作られた Environment は、`protection_rules` が空の配列、`deployment_branch_policy` が `null`、`can_admins_bypass` が `true` だった
> - 本文つきの作成：存在しない名前に `wait_timer: 3` だけを送ると `200` を返し、作られた Environment は `wait_timer` の保護ルールを持った。存在しない名前に `deployment_branch_policy` の `protected_branches: true`、`custom_branch_policies: false` だけを送ると `200` を返し、作られた Environment はその方式を持った。公開と非公開のリポジトリで確かめた。公開リポジトリで、存在しない名前に担当者にできる利用者だけの `reviewers` を送ると `200` を返し、作られた Environment はその担当者と `prevent_self_review: false` の保護ルールを持った。存在しない名前に担当者にできない ID（`0`）だけの `reviewers` を送ると、担当者の `422`（`Required reviewers must have at least one reviewer.`）を返したが、Environment は作られていた。`deployment_branch_policy` だけを送って作った Environment の `protection_rules` は、方式が `protected_branches` でも `custom_branch_policies` でも `type: branch_policy` の要素だけで、`null` では空の配列で、どれも `required_reviewers` と `wait_timer` の要素を持たなかった（公開リポジトリで確認）
> - 既存の Environment への本文のない `PUT` と `{}` の `PUT`：`200` を返し、値は何も変わらなかった。`wait_timer`、`reviewers`、`prevent_self_review: true`、`can_admins_bypass: false`、`protected_branches: true` を持つ Environment で確かめた
> - `can_admins_bypass`：本文に `can_admins_bypass: false` を送ると受け付けられ、取得が `false` を返した
> - 省略したフィールドの保持：`wait_timer: 7`、`reviewers`（ユーザー1人）、`prevent_self_review: true`、`can_admins_bypass: false` を持つ Environment に `deployment_branch_policy` だけを送ると、4つの値と、待機時間とレビュアーの保護ルールの `id` は変わらなかった。`custom_branch_policies: true` で4件のパターンを持つ Environment に `wait_timer` だけを送ると、`deployment_branch_policy` と、4件のパターン（同じ `id`）は変わらなかった。`custom_branch_policies: true` で1件のパターンを持ち、保護ルールのない Environment に `reviewers`（ユーザー1人）だけを送ると、`200` の応答本文は担当者の要素を含み、`deployment_branch_policy`、パターン（同じ `id`）、`can_admins_bypass` は変わらなかった
> - デプロイ元の制限を変える更新：`custom_branch_policies: true` から `protected_branches: true` または `null` に変えると、パターンはすべて失われ、`custom_branch_policies: true` に戻してもパターンの一覧は空だった。`custom_branch_policies: true` のまま同じ値を送り直したときは、パターンは同じ `id` で残った。一度も `custom_branch_policies: true` にしたことのない Environment を `custom_branch_policies: true` に変えた直後と、`custom_branch_policies: true` を本文に含めて作った直後も、パターンの一覧は空だった（非公開リポジトリで確認）。`null` から `protected_branches: true`、`protected_branches: true` から `null`、`custom_branch_policies: true` から `null` に変えた更新では、Environment の `updated_at` は変わらなかった。パターンのエンドポイントの応答は [GitHub デプロイ元パターン API 仕様](deployment-branch-policies.md) の注釈にある
> - 同じ値の `protected_branches` と `custom_branch_policies`：両方 `true`、両方 `false` のどちらも `422` を返し、本文の `message` は `"custom_branch_policies" and "protected_branches" cannot have the same value` だった。`deployment_branch_policy` だけを送った場合は、値は変わらなかった。ほかのフィールドと一緒に送った場合は、下の「`422` のときの部分的な反映」のとおり
> - 担当者を外す表現：担当者のいる Environment に `reviewers: []` を送っても `reviewers: null` を送っても、`required_reviewers` の要素はなくなった。`reviewers` を省略した `PUT` では、担当者は変わらなかった。担当者のいない Environment への `reviewers: []` と `reviewers: null` は、`200` を返して何も変えなかった
> - 担当者の形の検証：`type` を `User` と `Team` 以外の値（`user`）にした要素と、`id` を文字列にした要素は、`422` を返した。本文の `message` は、`Invalid request.` に続けて、値が列挙にないこと、または型が integer でないことを示した。`type` を省略した要素は、`User` として扱われた
> - 担当者の件数と重複：同じ利用者を重ねた `reviewers` は、6件までは `200` を返し、担当者は1人にまとめられた。7件は `422` で、`message` は `Failed to create the environment protection rule. Required reviewers can have at most 6 reviewers.` だった。件数は、まとめる前の要素の数で数えられた
> - 担当者にできない ID：存在しない利用者の ID、`0`、負の値、`id` のない要素と、個人アカウント所有のリポジトリでの `type: Team`（`id` が `1` と大きな値）は、それだけを送ると `422` を返し、`message` は `Failed to create the environment protection rule. Required reviewers must have at least one reviewer.` だった。担当者は変わらなかった。リポジトリの権限を持たない利用者（`octocat`）だけを送ると、`200` を返し、担当者のいないレビュアーの要素（3.2 の注釈）になった。担当者にできる利用者と一緒に送ると、どの場合も `200` を返し、担当者にできない要素だけが黙って除かれた。送った順は結果を変えなかった
> - 利用者の ID を `type: Team` で送った要素：レビュアーの要素がないときは `500` を返し（2回）、要素は作られなかった。担当者のいる要素があるときは `200` を返し、担当者は変わらなかった（2回）
> - `prevent_self_review` と担当者：レビュアーの要素がないとき、または担当者のいない要素のときは、`prevent_self_review` を `true` で送っても `false` で送っても `422` を返した。`message` は `Failed to create or update the environment protection rule. Required reviewers must have at least one reviewer to set prevent_self_review.` だった。担当者と一緒に送ると、要素のない Environment でも `prevent_self_review: true` で要素が作られた。`reviewers: []` と一緒に送った `prevent_self_review: true` は `200` を返し、要素はなくなった。`prevent_self_review: true` の要素の担当者を外すと値も失われ、あとで担当者だけを送って要素を作り直すと `prevent_self_review: false` になった（`reviewers: []` と `reviewers: null` の両方で確認）。`null` と文字列の `prevent_self_review` は `422` を返した
> - 保護設定どうしの保持：`prevent_self_review` だけの `PUT` で担当者が、担当者だけの `PUT` で `prevent_self_review: true` が、`wait_timer` だけの `PUT` で担当者と `prevent_self_review` が、`can_admins_bypass` だけの `PUT` で担当者、`prevent_self_review`、`wait_timer` が、`wait_timer` だけの `PUT` で `can_admins_bypass: false` が、それぞれ変わらなかった。保護ルールの `id` も変わらなかった。待機時間、担当者、`can_admins_bypass: false`、`custom_branch_policies: true` と1件のパターンを持つ Environment に `prevent_self_review` だけを `true` と `false` で送ると、`wait_timer`、担当者、`can_admins_bypass`、`deployment_branch_policy`、パターン（同じ `id`）と、保護ルールの `id` は変わらなかった。同じ Environment に `wait_timer` だけを送ると（`0` を含む）、`prevent_self_review: true` は変わらなかった
> - 待機時間の値：`0` と負の値（`-1`、`-5`）は、どちらも `wait_timer` の要素をなくした。待機時間のない Environment への `0` と負の値は、`200` を返して何も変えなかった。`1` から `43199` までの値（確かめたのは `1`、`2`、`3`、`5`、`7`、`30`、`59`、`60`、`120`、`500`、`1000`、`1440`、`10000`、`43199`）は反映された。`43200` は `200` を返したが値は変わらず、待機時間のない Environment でも要素は作られなかった（4回）。`43201` は `422` で、`message` は `Failed to create the environment protection rule. The wait timer cannot be greater than 43200 minutes.` だった。`null`、`1.5`、`2.0`、文字列は `422` を返した
> - `can_admins_bypass` の値：`null` と文字列は `422` を返した
> - `422` のときの部分的な反映：一つの `PUT` で複数のフィールドを送り、`422` が返ったとき、一部のフィールドの変更が反映されたまま残ることがあった。`reviewers: []` と `wait_timer: 43201` は `wait_timer` の `422` を返したが、担当者はなくなった（2回）。担当者と `prevent_self_review: false` と `wait_timer: 43201` は、`prevent_self_review` が `false` になった。`can_admins_bypass: false` と `wait_timer: 9` と両方 `true` の `deployment_branch_policy` は方式の `422` を返したが、`can_admins_bypass` と `wait_timer` は反映された。一方、存在しない利用者の `reviewers` と、`wait_timer`、`prevent_self_review`、`can_admins_bypass`、`deployment_branch_policy` のどれかを一緒に送った `PUT` は、担当者の `422` を返し、何も変えなかった。`wait_timer: 43201` と `can_admins_bypass: false`、または `deployment_branch_policy` を一緒に送った `PUT` も、何も変えなかった。確かめた範囲では、担当者、待機時間、管理者による迂回、デプロイ元の制限の順に処理され、拒否されたフィールドより前に処理されたフィールドの変更が残ったように見える
> - 更新直後の取得：作成と更新（約30回）の約1秒以内の取得は、どれも更新後の値を返した。保護設定の更新を含む `PUT`（133回、うち `200` が84回）でも、応答を受け取った直後の取得は、どれも `200` の応答本文と同じだった
> - 別の名前の Environment：確認の前から存在した Environment は、別の名前の Environment を作成、更新、削除した確認の前後で、`updated_at` を含む全フィールドと、デプロイ元パターン（同じ `id`）が一致した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create or update an environment](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#create-or-update-an-environment) | External Document |
| `grounding` | [Deployments and environments: Deployment protection rules](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments#deployment-protection-rules) | External Document |
