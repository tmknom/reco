# GitHub Actions 権限 API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for GitHub Actions permissions」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、Actions permissions・Selected actions・Workflow permissions・リポジトリ外のワークフローからのアクセスの取得・更新に必要な権限、HTTP 応答、フィールドです。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が、リポジトリの名前と ID は [GitHub リポジトリ API 仕様](repositories.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

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
| リポジトリ外のワークフローからのアクセスの取得 | `GET /repos/{owner}/{repo}/actions/permissions/access` | Administration: read |
| リポジトリ外のワークフローからのアクセスの更新 | `PUT /repos/{owner}/{repo}/actions/permissions/access` | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}/actions/permissions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` | `204` | 記載なし |
| `GET /repos/{owner}/{repo}/actions/permissions/workflow` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/workflow` | `204` | `409` 所有 Organization によって設定の変更が妨げられた場合の競合応答 |
| `GET /repos/{owner}/{repo}/actions/permissions/access` | `200` | 記載なし |
| `PUT /repos/{owner}/{repo}/actions/permissions/access` | `204` | 記載なし |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-administration) | External Document |
| `grounding` | [Repository roles for an organization](https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization) | External Document |

## 2. 更新操作の単位と結果の確認

本ドキュメントの更新エンドポイントは、それぞれ独立したリクエストです。Actions permissions・Selected actions・Workflow permissions・リポジトリ外のワークフローからのアクセスのうち複数を、一つのリクエストでまとめて更新するエンドポイントは公開ドキュメントにありません。

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
> 次は、reco の開発者が実際の動作で確認した事項です（公開・非公開リポジトリで確認）。
>
> - Actions が有効のときの応答：`sha_pinning_required` は、必須ではないフィールドだが常に含まれていた
> - Actions が無効のときの応答：`enabled` が `false` のとき、`allowed_actions` と `selected_actions_url` は応答に含まれない。`sha_pinning_required` は保存された値によらず `false` で返る。`enabled` だけを `true` にして有効に戻すと、無効にする前の `allowed_actions` と `sha_pinning_required` が返る。`allowed_actions` を `selected`、`sha_pinning_required` を `true` にしてから確かめた

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

リポジトリ設定のドキュメントは、Actions の無効化と、実行を許可する Actions・再利用可能なワークフローの範囲を、設定画面の「Actions」の「General」ページにある「Actions permissions」の欄で選ぶと記載しています。選択肢のうち、ドキュメントが名前を挙げているのは、4章の条件で許可するものを選ぶ「Allow *OWNER*, and select non-*OWNER*, actions and reusable workflows」だけです。`sha_pinning_required` に当たるのは、同じ欄の「Require actions to be pinned to a full-length commit SHA」で、有効にするとすべての Actions を完全な長さのコミット SHA に固定しないと使えなくなり、再利用可能なワークフローはタグでも参照できると記載しています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 省略したフィールドの保持：`sha_pinning_required` を省略して `allowed_actions` を変えると、`sha_pinning_required` は変わらない。`enabled` だけを送ると、`allowed_actions` と `sha_pinning_required` は変わらない。`enabled: true` と `sha_pinning_required` だけを送ると、`allowed_actions` と4章の3項目は変わらない（いずれも公開・非公開リポジトリで確認）。最後の場合は、`allowed_actions` を `selected` にし、3項目を既定と異なる値（`github_owned_allowed: false`、`verified_allowed: true`、空でない `patterns_allowed`）にしてから確かめた
> - `sha_pinning_required` の値の反映：Actions が有効のとき、`true` と `false` のどちらを送っても、直後の 3.1 の取得が送った値を返した（公開・非公開リポジトリで確認）
> - Actions が無効のときの `sha_pinning_required`：無効のリポジトリへ `enabled: false` と `sha_pinning_required: true` を送ると `204` を返すが、値は保存されない。`enabled` だけを `true` にして有効に戻すと、送る前の保存値 `false` が返った（非公開リポジトリで確認）
> - `allowed_actions` に `selected` を送ったときの副作用：現在の値がすでに `selected` でも、4章の `github_owned_allowed` が `true` に変わる。`verified_allowed` と `patterns_allowed` は変わらない。`enabled` だけを送った場合は変わらない（公開・非公開リポジトリで確認）
> - Actions が無効の状態からの変更：`enabled: true` と `allowed_actions` を一つのリクエストで送ると、有効化とポリシーの変更が同時に反映され、省略した `sha_pinning_required` は無効にする前の値のまま保持される（公開・非公開リポジトリで確認）。無効にする前のポリシーが `selected` だった場合、`selected` を送ると4章の3項目は `github_owned_allowed` が `true` に変わるほかは残り、`all` を送ると3項目は残らない（非公開リポジトリで確認）
> - 更新直後の取得：`allowed_actions` を `all` と `local_only` の間で変える更新、`selected` にする更新、Actions が無効の状態から `enabled: true` と `allowed_actions: all` を送って有効にする更新では、`204` の約 0.3 秒後の 3.1 の取得が更新後の値を返し、続けて約 0.6 秒おきに取得しても更新前の値には戻らなかった。`selected` にした直後の4章の取得も `409` にならず値を返した。有効にした直後の取得が `enabled: false` を返すこともなかった（公開・非公開リポジトリで、順に6回、3回、3回ずつ確認）。`enabled: true` と `sha_pinning_required` だけを送る更新（公開で7回、非公開で3回）、`enabled` だけを送って無効または有効にする更新（公開で各1回、非公開で各2回）でも、`204` を受け取ってから1秒以内、約 1.5 秒後、約 3 秒後の取得は、どれも更新後の値を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set GitHub Actions permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-github-actions-permissions-for-a-repository) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Managing GitHub Actions permissions for your repository](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#managing-github-actions-permissions-for-your-repository) | External Document |

## 4. Selected actions

Selected actions の取得・更新は、どちらもリポジトリの `allowed_actions` が `selected` であることを利用の前提とします。`allowed_actions` が `all` または `local_only` のときにこれらのエンドポイントを呼んだ場合の応答（ステータスコードや本文）は、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 前提を満たさないときの応答：`allowed_actions` が `all` または `local_only` のとき、および Actions が無効のとき、取得は `409` を返し、値は返らない。本文の `errors` は、それぞれ `All actions and workflows are allowed on this repository`、`Only local actions and workflows are allowed on this repository`、`GitHub Actions is disabled on this repository` である。更新も、3つの場合のどれでも同じ `errors` の `409` を返す（非公開リポジトリで確認）
> - `selected` から別のポリシーへの切替：`all` または `local_only` を経て `selected` に戻すと、切替前の値は残らず、`github_owned_allowed` が `true`、`verified_allowed` が `false`、`patterns_allowed` が `[]` になる（非公開リポジトリで `all`、公開リポジトリで `local_only` を経由して確認）。`all` から `selected` にした直後の値も同じだった（公開・非公開リポジトリで確認）
> - Actions の無効化：無効にしてから `enabled` だけを `true` にして有効に戻すと、無効にする前の3項目の値が返る（公開・非公開リポジトリで確認）
> - 省略したフィールドの保持：`github_owned_allowed` だけを送ると、`verified_allowed` と `patterns_allowed` は変わらない（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Set allowed actions and reusable workflows for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-allowed-actions-and-reusable-workflows-for-a-repository) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Allowing select actions and reusable workflows to run](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#allowing-select-actions-and-reusable-workflows-to-run) | External Document |

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

リポジトリ設定のドキュメントは、設定画面で「Allow *OWNER*, and select non-*OWNER*, actions and reusable workflows」を選んだときの追加の項目として、GitHub が作成した Actions を許可する「Allow actions created by GitHub」、Marketplace の認証済み作成者の Actions を許可する「Allow actions by Marketplace verified creators」、特定の Actions・再利用可能なワークフローを指定する「Allow or block specified actions and reusable workflows」を挙げています。

`patterns_allowed` の設定は **公開リポジトリにのみ適用される** と記載されています。非公開リポジトリで `patterns_allowed` を送ったときに値が保存されるか、取得時に返るか、空配列がどう扱われるかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開・非公開リポジトリで確認）。
>
> - `patterns_allowed` の保存：非公開リポジトリでも、送った空でない配列と `[]` は保存され、4.1 の取得で同じ要素が返る。保存された値が非公開リポジトリのワークフロー実行に適用されるかは確認していない
> - `patterns_allowed` の要素の順序：4.1 の取得が返す要素の順序は、送った順序と同じとは限らない。更新前の `patterns_allowed` が空でないときに複数の要素を送ると、送った順序と異なる順序で返った。`["actions/setup-go@*", "octo-org/*", "actions/checkout@*"]` を送ると `["actions/checkout@*", "actions/setup-go@*", "octo-org/*"]` が、`["tmknom/*", "actions/checkout@*"]` を送ると `["actions/checkout@*", "tmknom/*"]` が返り、どちらも文字列の昇順だった。更新前が `[]` のときと、`allowed_actions` を `selected` にした直後に送ったときは、送った順序のまま返った。順序が変わる条件と並べ方の規則は確認していない
> - ほかの設定への影響：この更新は、3.1 の `allowed_actions` と `sha_pinning_required` を変えない。`sha_pinning_required` を `true` にしてから3項目を変えて確かめた
> - 更新直後の取得：3項目を変える更新では、`204` の約 0.3 秒後の 4.1 の取得が更新後の値を返し、続けて約 0.6 秒おきに取得しても更新前の値には戻らなかった（それぞれ6回確認）

## 5. Workflow permissions

### 5.1 `GET /repos/{owner}/{repo}/actions/permissions/workflow`

- 用途：ワークフロー実行時に `GITHUB_TOKEN` へ与えられる既定の権限と、GitHub Actions が Pull Request を承認するレビューを送信できるかの取得
- 応答フィールド：`default_workflow_permissions`（`read` / `write`、必須）、`can_approve_pull_request_reviews`（boolean、必須）

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - Actions が無効のときの応答：`200` で2フィールドを返す。値は無効にする前と同じだった（非公開リポジトリで確認）
> - 権限のないリポジトリへの応答：トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリでは `403` を返す。本文の `message` は `You must have repository read permissions or have the repository Actions policies fine-grained permission.` だった（`repo` scope を持つ personal access token (classic) で確認）

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

リポジトリ設定のドキュメントは、`GITHUB_TOKEN` の既定の権限を、設定画面の「Actions」の「General」ページにある「Workflow permissions」の欄で、すべての権限の読み書き（permissive）か、`contents` と `packages` の読み取りだけ（restricted）かから選ぶと記載しています。二つの選択肢の項目名は記載がありません。

`can_approve_pull_request_reviews` について、REST API のドキュメントは承認にしか触れていません。リポジトリ設定のドキュメントは、設定画面の「Workflow permissions」の欄にある「Allow GitHub Actions to create and approve pull requests」という項目で、GitHub Actions による Pull Request の作成と承認を許可・禁止できると記載しています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - `can_approve_pull_request_reviews` の対象：設定画面の上記の項目に対応し、Pull Request の承認だけでなく作成も対象に含む
> - 省略したフィールドの保持：`default_workflow_permissions` だけを送ると、`can_approve_pull_request_reviews` は変わらない。`can_approve_pull_request_reviews` だけを送ると、`default_workflow_permissions` は変わらない。いずれも、省略した側を `true`、`write` にしてから確かめた（公開・非公開リポジトリで確認）
> - 値の反映：`read`・`write`・`true`・`false` のいずれを送っても、送った値が 5.1 の取得に反映された（公開・非公開リポジトリで確認）
> - 更新直後の取得：公開リポジトリで `can_approve_pull_request_reviews` を `true` から `false` にする更新では、`204` を受け取った直後の 5.1 の取得が更新前の `true` を返すことがあった。6回中5回で起きた。時刻を測った2回では、`204` の約 0.5 秒後の取得が `true`、約 1.5 秒後の取得が `false` を返した。別の機会に同じ更新を2回行ったときは、`204` の約 0.3 秒後の取得が `false` を返した。公開リポジトリで `default_workflow_permissions` を `write` から `read` にする更新でも、直後の取得が更新前の `write` を返したことが1回あった。同じ更新のほかの回では、直後の取得が送った値を返した。`can_approve_pull_request_reviews` を `false` から `true` にする更新、`default_workflow_permissions` を `read` から `write` にする更新、非公開リポジトリで `can_approve_pull_request_reviews` を `false` にする更新では、直後の取得が送った値を返した
> - Actions が無効のときの更新：片方だけを送る更新は `204` を返して反映され、もう一方は変わらない。Actions を有効に戻したあとも、無効のあいだに送った値が返る（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set default workflow permissions for a repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-default-workflow-permissions-for-a-repository) | External Document |
| `grounding` | [Workflow syntax for GitHub Actions: permissions](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Configuring the default GITHUB_TOKEN permissions](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#configuring-the-default-github_token-permissions) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Preventing GitHub Actions from creating or approving pull requests](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#preventing-github-actions-from-creating-or-approving-pull-requests) | External Document |

## 6. リポジトリ外のワークフローからのアクセス

非公開リポジトリの Actions と再利用可能なワークフローは、同じユーザーまたは Organization が所有するほかの非公開リポジトリと共有できます。本章のエンドポイントは、リポジトリ外のワークフローがそれらにアクセスできる範囲を取得・設定します。取得と更新のどちらのドキュメントも、 **非公開リポジトリにだけ適用される** と記載しています。公開リポジトリに送ったときの応答は記載がありません。

リポジトリ設定のドキュメントは、この範囲を設定画面の「Actions」の「General」ページにある「Access」の欄で選ぶと記載しています。ユーザー所有のリポジトリの選択肢は「Not accessible」と「Accessible from repositories owned by '*USER NAME*' user」、Organization 所有のリポジトリの選択肢は「Not accessible」と「Accessible from repositories in the '*ORGANIZATION NAME*' organization」です。どちらも、アクセスは非公開リポジトリからだけ許可されると記載しています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（`repo` scope を持つ personal access token (classic) で確認）。
>
> - 公開リポジトリへの取得：トークン所有者が Admin ロールを持つ公開リポジトリでは `422` を返し、値は返らない。本文の `message` は `Access policy only applies to internal and private repositories.` だった。Actions が無効のときも同じだった
> - 権限のないリポジトリへの取得：トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリ（`cli/cli`）では `403` を返す。本文の `message` は 5.1 の注釈と同じ `You must have repository read permissions or have the repository Actions policies fine-grained permission.` で、公開リポジトリへの `422` とは異なる
> - Actions が無効のとき：取得は `200` で値を返し、更新も `204` を返して反映される。Actions を有効に戻したあとも、無効のあいだに送った値が返る。3.2 の無効化と再有効化で、値は変わらない（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10) | External Document |
| `grounding` | [Managing GitHub Actions settings for a repository: Allowing access to components in a private repository](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#allowing-access-to-components-in-a-private-repository) | External Document |

### 6.1 `GET /repos/{owner}/{repo}/actions/permissions/access`

- 用途：リポジトリ外のワークフローが、このリポジトリの Actions と再利用可能なワークフローにアクセスできる範囲の取得
- 応答フィールド：`access_level`（`none` / `user` / `organization`、必須）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get the level of access for workflows outside of the repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#get-the-level-of-access-for-workflows-outside-of-the-repository) | External Document |

### 6.2 `PUT /repos/{owner}/{repo}/actions/permissions/access`

- 用途：6.1 で取得する範囲の設定

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `access_level` | `none` / `user` / `organization` | 必須 | `none` はこのリポジトリのワークフローからだけアクセスできる。`user` はユーザー所有の非公開リポジトリのあいだで共有する。`organization` は Organization の中で共有する |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（ユーザー所有の非公開リポジトリで確認）。
>
> - 値の反映：`none` と `user` のどちらを送っても `204` を返し、直後の 6.1 の取得が送った値を返した。`204` を受け取ってから1秒以内、約 1.5 秒後、約 3 秒後の取得は、どれも更新後の値を返した（`user` にする更新で1回、`none` にする更新で2回）
> - ユーザー所有のリポジトリへの `organization`：`422` を返し、値は変わらない。本文の `errors` は `Only 'none' and 'user' access levels are allowed for this repository.` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set the level of access for workflows outside of the repository](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10#set-the-level-of-access-for-workflows-outside-of-the-repository) | External Document |
