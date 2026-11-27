# GitHub Rules API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for rules」のうち、reco が利用するリポジトリ単位の Ruleset のエンドポイントの外部契約を整理します。対象は、Ruleset の一覧・取得・作成・更新に必要な権限、HTTP 応答、フィールドと、Ruleset が持つ対象の条件、Bypass、ルールの形です。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

GitHub App の権限の一覧では、一覧と取得が Metadata の read、作成・更新・削除が Administration の write に分類されています。OAuth app のトークンと personal access token (classic) に要る scope は、エンドポイントのドキュメントに記載がありません。

| 用途 | メソッド・パス | GitHub App の権限 |
|---|---|---|
| Ruleset の一覧 | `GET /repos/{owner}/{repo}/rulesets` | Metadata: read |
| Ruleset の取得 | `GET /repos/{owner}/{repo}/rulesets/{ruleset_id}` | Metadata: read |
| Ruleset の作成 | `POST /repos/{owner}/{repo}/rulesets` | Administration: write |
| Ruleset の更新 | `PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}` | Administration: write |
| Ruleset の削除 | `DELETE /repos/{owner}/{repo}/rulesets/{ruleset_id}` | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET .../rulesets` | `200` | `404`、`500` |
| `GET .../rulesets/{ruleset_id}` | `200` | `404`、`500` |
| `POST .../rulesets` | `201` | `404`、`422` 検証の失敗、`500` |
| `PUT .../rulesets/{ruleset_id}` | `200` | `404`、`422` 検証の失敗、`500` |
| `DELETE .../rulesets/{ruleset_id}` | `204` | `404`、`500` |

Ruleset を利用できる料金プランとリポジトリの公開範囲は、エンドポイントのドキュメントに記載がありません。Ruleset の概要のドキュメントは、一つのリポジトリに Ruleset を75個まで置けると記載しています。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つ personal access token (classic) で実際の動作を確認した事項です（個人アカウント所有のリポジトリで、所有者のトークンで確認）。
>
> - 非公開リポジトリ：GitHub Free の個人アカウントが所有する非公開リポジトリでは、一覧、取得、作成（`enforcement` が `disabled` でも `active` でも）、ブランチに適用されるルールの取得（`GET .../rules/branches/{branch}`）が、どれも `403` を返した。本文の `message` は `Upgrade to GitHub Pro or make this repository public to enable this feature.` だった
> - 公開リポジトリ：一覧、取得、作成、更新、削除ができた。削除は `204` を返した
> - 存在しない `ruleset_id`：取得は `404`（`message` は `Not Found`）を返した。削除した Ruleset の取得も `404` だった。削除した Ruleset の `id` への削除も `404`（`message` は `Not Found`）を返した
> - 削除の応答と影響：本文を送らない削除は `204` を返し、応答に本文はなかった。削除の前後で、同じリポジトリのほかの Ruleset の取得の本文、`GET /repos/{owner}/{repo}` のマージ方式、マージメッセージ、リポジトリ機能のフィールド、デフォルトブランチの Branch protection の取得の本文は変わらなかった
> - `500` の応答：`bypass_actors` に `actor_type: RepositoryRole`、`actor_id: 999` を含む更新は `500` を返し、本文はなかった。同じ要求で送ったほかの変更（`rules`）も反映されず、直後の取得は要求の前と一致した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for rules](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2026-03-10) | External Document |
| `grounding` | [About rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets) | External Document |

## 2. Ruleset の名前と ID

Ruleset は、GitHub が付ける整数の `id` と、作成で必須の `name` を持ちます。名前がリポジトリの中で一意か、大文字小文字を区別するか、更新で `id` が変わるかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 同じ名前：すでにある名前で作成すると `422` を返し、本文の `errors` は `["Name must be unique"]` だった。Ruleset は作られなかった
> - 大文字小文字：大文字小文字だけが違う名前（`reco-check-001-a` に対する `RECO-CHECK-001-A`、`reco-check-001-ÄÖ` に対する `reco-check-001-äö`）の作成も、同じ `422` だった。ほかの Ruleset と大文字小文字だけが違う名前への名前の変更も `422` だった。自分自身の名前の大文字小文字を変える名前の変更は `200` で反映された
> - `target` の違い：`target` が `branch` の Ruleset と同じ名前と、大文字だけが違う名前で、`target` が `tag` の Ruleset を作ると、どちらも同じ `422` を返した
> - 前後の空白：先頭か末尾に半角空白を付けた名前は、空白を除いた名前で作られた。空白を除くとすでにある名前と同じになる名前の作成は `422`（`Name must be unique`）だった。途中の連続した半角空白は、送ったとおりに保存された
> - 空の名前：`422` を返し、`errors` は `["Name cannot be empty"]` だった
> - `id`：作成の応答の `id` は、直後の取得、一覧と一致した。`enforcement` だけの更新と、名前の変更の前後で、`id` は変わらなかった。Ruleset を削除して同じ名前で作り直すと、別の `id` になり、元の `id` の取得は `404` を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |

## 3. Ruleset の一覧と取得

### 3.1 `GET .../rulesets`

- 用途：リポジトリの Ruleset の一覧の取得
- クエリパラメータ：`per_page`（既定 `30`、最大 `100`）、`page`（既定 `1`）、`includes_parents`（既定 `true`。リポジトリより上位で構成され、リポジトリに適用される Ruleset を含める）、`targets`（`branch`、`tag`、`push` をカンマで区切って絞る）
- 応答：3.2 の Ruleset の配列

要素の並び順、ページの終わりの判定方法、一覧の要素が 3.2 のどのフィールドを含むかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 要素のフィールド：一覧の要素は `id`、`name`、`target`、`source_type`、`source`、`enforcement`、`node_id`、`_links`、`created_at`、`updated_at` だけを含み、`conditions`、`rules`、`bypass_actors`、`current_user_can_bypass` を含まなかった
> - ページング：`per_page=2` で Ruleset が11個のとき、各ページが2件（最後のページは1件）を返した。応答ヘッダー `Link` は `/repositories/{repository_id}/rulesets?...` の形の URL を持ち、最後のページには `next` がなかった。全ページの要素を合わせると、`per_page=100` の一覧と同じ要素が同じ順に並び、重複はなかった
> - 並び順：名前の順だった。大文字で始まる名前が、小文字で始まる名前より先に並んだ（11件の1例）
> - `includes_parents`：個人アカウント所有のリポジトリでは、指定しない一覧と `includes_parents=false` の一覧が一致し、どの要素も `source_type` が `Repository`、`source` が `{owner}/{repo}` だった。Organization 所有の第三者の公開リポジトリでは、指定しない一覧に `source_type` が `Organization`、`source` が Organization の名前の要素が加わり、`includes_parents=false` の一覧はそれを含まなかった
> - `targets`：`targets=tag` は、`target` が `branch` の Ruleset だけを持つリポジトリで空の配列を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get all repository rulesets](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#get-all-repository-rulesets) | External Document |

### 3.2 `GET .../rulesets/{ruleset_id}`

- 用途：一つの Ruleset の取得
- クエリパラメータ：`includes_parents`（既定 `true`）

| 応答フィールド | 型・値 | 必須 |
|---|---|---|
| `id` | integer | 必須 |
| `name` | 文字列 | 必須 |
| `target` | `branch` / `tag` / `push` / `repository` | 任意 |
| `source_type` | `Repository` / `Organization` / `Enterprise` | 任意 |
| `source` | 文字列（所有元の名前） | 必須 |
| `enforcement` | `disabled` / `active` / `evaluate` | 必須 |
| `bypass_actors` | 6章の Bypass の配列 | 任意 |
| `current_user_can_bypass` | `always` / `pull_requests_only` / `never` / `exempt` | 任意 |
| `conditions` | 5章の対象の条件。`null` を取り得る | 任意 |
| `rules` | 7章のルールの配列 | 任意 |
| `node_id`、`_links`、`created_at`、`updated_at` | 文字列ほか | 任意 |

ドキュメントは、機密の漏洩を防ぐため、`bypass_actors` は要求した利用者が Ruleset への書き込みのアクセスを持つ場合に限り返すと記載しています。`current_user_can_bypass` は、要求した利用者がその Ruleset を Bypass できる種類を表し、リポジトリ単位のエンドポイントでだけ返ります。`enforcement` の `evaluate` は GitHub Enterprise でだけ利用できると記載されています。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 所有者のトークン：Admin ロールの所有者のトークンでは、取得の本文は `bypass_actors` を含んだ。Bypass がないときは、フィールドが欠けず `[]` を返した
> - 書き込みのアクセスがないとき：第三者の公開リポジトリ（`cli/cli`、`microsoft/vscode`、`github/docs`）の Ruleset の取得は `200` を返し、本文は `bypass_actors` のフィールドを含まなかった。`current_user_can_bypass` は `never` だった。`conditions` と `rules` は含んだ
> - 上位の Ruleset：Organization が所有する Ruleset を、それが適用されるリポジトリのこのエンドポイントで取得すると、`includes_parents` を指定しなければ `200`、`includes_parents=false` では `404` を返した
> - `conditions` の形：`conditions` を送らずに作った Ruleset の取得は、`conditions` に `{}` を返した（5章）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#get-a-repository-ruleset) | External Document |

## 4. Ruleset の作成と更新

### 4.1 `POST .../rulesets`

- 用途：リポジトリへの Ruleset の追加

| 本文フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `name` | 文字列 | 必須 | Ruleset の名前 |
| `target` | `branch` / `tag` / `push` | 任意（既定 `branch`） | Ruleset の対象 |
| `enforcement` | `disabled` / `active` / `evaluate` | 必須 | 強制の段階 |
| `bypass_actors` | 6章の Bypass の配列 | 任意 | ルールを Bypass できる主体 |
| `conditions` | 5章の対象の条件 | 任意 | Ruleset を適用する ref |
| `rules` | 7章のルールの配列 | 任意 | Ruleset のルール |

応答の本文は 3.2 と同じ形です。省略したフィールドの扱いは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 作成の応答：`201` を返し、本文は Ruleset の全体（`bypass_actors` と `current_user_can_bypass` を含む）だった。本文は、直後の取得の本文と `updated_at` を含めて一致した
> - 省略したフィールド：`name` と `enforcement` だけで作成すると、`target` は `branch`、`bypass_actors` は `[]`、`conditions` は `{}`、`rules` は `[]` で作られた。`bypass_actors` だけを省略した作成でも `bypass_actors` は `[]` だった
> - 空の配列の明示：`include: []`、`exclude: []`、`bypass_actors: []`、`rules: []` を送った作成は `201` を返し、どれも `[]` で作られた
> - ルールの順序：作成の応答と取得は、送った `rules` の順を保った（2例）
> - 作成直後の取得：作成（5回）の直後の取得と一覧は、どれも作成した Ruleset を含み、取得の本文は作成の応答と一致した。reco の `apply` による作成（3回）は、どれも更新後の観測で作成した Ruleset を確かめて完了した。要求を数えた1回では、`201` の直後の一覧と取得が作成した Ruleset を返し、観測し直しは起きなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |

### 4.2 `PUT .../rulesets/{ruleset_id}`

- 用途：Ruleset の更新
- 本文フィールド：4.1 と同じフィールドを持ち、どれも任意である。`target` に既定値はない

応答の本文は 3.2 と同じ形です。省略したフィールドを保持するか、送った配列が既存の配列を置き換えるかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 更新の応答：`200` を返し、本文は Ruleset の全体だった。本文は、直後の取得の本文と一致した。更新（10回）の直後の取得と一覧は、どれも更新後の値（一覧は `updated_at`）を返した。reco の `apply` による `name` を除いた全体の更新（6回）は、どれも更新後の観測で宣言した値を確かめて完了した。要求を数えた1回では、`200` の直後の取得が更新後の値を返し、観測し直しは起きなかった
> - 省略したフィールド：`rules`、`bypass_actors`、`conditions` のいずれか一つを省略した更新は、省略したフィールドの既存の値を保持した。`rules` を省略した更新では、`pull_request` の任意のパラメータ（7.1）の既定でない値も保持された。`enforcement` だけ、`name` だけの更新も、ほかのフィールドを保持した
> - 送った配列：`rules` は既存の配列を置き換えた。`pull_request` と `deletion` を持つ Ruleset に `deletion` だけを送ると、`pull_request` は消えた。`rules: []` と `bypass_actors: []` は、どちらも空の配列を保存した
> - ルールの順序：取得が返す `rules` の順は、送った順と違う場合があった。`[deletion, pull_request]` を送って `[pull_request, deletion]` を返した例と、`[pull_request, deletion]` を送って `[deletion, pull_request]` を返した例がある。`[non_fast_forward, deletion, creation]` を送ると、`[deletion, non_fast_forward, creation]` を返した例と `[deletion, creation, non_fast_forward]` を返した例がある。更新の応答と直後の取得の順は一致した
> - `evaluate`：個人アカウント所有のリポジトリで `enforcement: evaluate` を送ると `422` を返し、`errors` は `["Enforcement evaluate option is not supported on this plan. Please upgrade to Enterprise to enable it."]` だった
> - 同じ種類のルール：`deletion` を二つ送ると `200` を返し、一つにまとめて保存した。`pull_request` を二つ送ると `422` を返し、`errors` は `["Invalid rule 'pull_request': "]` だった
> - 未知のルールの種類：`type: no_such_rule` のルールは `422` を返し、`message` は `Invalid request.\n\nInvalid property /rules/0: data matches no possible input. See \`documentation_url\`.` だった
> - リポジトリのほかの設定：`pull_request` の `allowed_merge_methods` を `["squash"]` にする更新と、`enforcement` を `active` にして `deletion` だけを送る更新の前後で、`GET /repos/{owner}/{repo}` のマージ方式（`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge`）、`allow_auto_merge`、`delete_branch_on_merge`、`allow_update_branch`、マージメッセージの4つのフィールドと、デフォルトブランチの Branch protection の取得の本文は変わらなかった
> - `name` を除いた全体：`name` を送らず、`target`、`enforcement`、`conditions`、`bypass_actors`（`DeployKey` の `actor_id: null` を含む）、`rules`（必須の5項目だけの `pull_request` と `parameters` のない `deletion`）をすべて送った更新は `200` を返し、`name` と `id` を保ったまま、送った値で置き換えた
> - 存在しない `ruleset_id`：削除した Ruleset の `id` への更新は `404`（`message` は `Not Found`）を返した
> - `target` の変更：`target` を `branch` から `tag`（`conditions` を `refs/tags/` の名前に変えて）へ、`tag` から `branch` へ変える更新は、どちらも `200` で反映され、`id` は変わらなかった。`refs/heads/` の名前の `conditions` のまま `tag` に変える更新と、`deletion` と `pull_request` を持つまま `push` に変える更新は `422` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#update-a-repository-ruleset) | External Document |

## 5. 対象の条件

`conditions.ref_name` は、`include` と `exclude` の文字列の配列を持ちます。`include` のパターンのどれかに一致し、`exclude` のパターンのどれにも一致しない ref が対象です。`include` は、デフォルトブランチを表す `~DEFAULT_BRANCH` と、すべてのブランチを表す `~ALL` も受け付けます。Ruleset の作成のドキュメントは、パターンに `fnmatch` の構文を使え、`*` はディレクトリの区切り（`/`）に一致しないと記載しています。`include` と `exclude` が必須か、`refs/heads/` のない名前を正規化するかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで、`target: branch` で確認）。
>
> - 特別な値：`~DEFAULT_BRANCH` と `~ALL` は、送ったとおりに保存され、取得で返った。`~default_branch` は `422` を返し、`errors` は `["Invalid target patterns: '~default_branch'"]` だった
> - `refs/heads/` のない名前：`include` の `reco-check-001`、`exclude` の `reco-check-x` は、どちらも `422`（`Invalid target patterns: '...'`）を返し、正規化されなかった
> - 空の配列：`include: []` と `exclude: []` は `200` を返し、どちらも `[]` で保存された
> - `exclude` の欠落：`ref_name` に `include` だけを送ると `422` を返し、`errors` は ``["Missing required parameter `exclude`"]`` だった
> - `conditions: {}`：`200` を返し、`conditions` は `{}` で保存された
> - 順序と重複：`include` と `exclude` は送った順のまま返った。`include` に同じ名前を二つ送ると、二つのまま保存された

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |
| `grounding` | [Creating rulesets for a repository](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository) | External Document |

## 6. Bypass

`bypass_actors` の要素は、次のフィールドを持ちます。

| フィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `actor_type` | `Integration` / `OrganizationAdmin` / `RepositoryRole` / `Team` / `DeployKey` / `User` | 必須 | Bypass できる主体の種類 |
| `actor_id` | integer、`null` を取り得る | `Integration`、`RepositoryRole`、`Team`、`User` で必須 | 主体の ID。`OrganizationAdmin` では無視され、`DeployKey` では `null` にする |
| `bypass_mode` | `always` / `pull_request` / `exempt` | 任意（既定 `always`） | Bypass できる場面 |

ドキュメントは、`OrganizationAdmin` は個人アカウントのリポジトリには当てはまらないと記載しています。`bypass_mode` の `pull_request` は、Pull Request でだけ Bypass できることを表し、`DeployKey` には当てはまらず、ブランチの Ruleset にだけ当てはまります。`exempt` は、その主体にルールを実行せず、Bypass の監査記録も作らないことを表します。`RepositoryRole` の `actor_id` がどのロールを指すかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（個人アカウント所有の公開リポジトリで確認）。
>
> - `RepositoryRole`：`actor_id` が `2`、`4`、`5` は `200` で保存された。`1` と `3` は `422`（`Actor base role does not have write permissions`）、`6` は `422`（`Invalid bypass actor: ...`）、`999` は `500` を返した（1章）。`actor_id` を省略すると `422`（`actor_id is required for RepositoryRole`）だった。Admin ロールの所有者のトークンでの取得では、`actor_id: 5`、`bypass_mode: always` の Bypass を持つ Ruleset の `current_user_can_bypass` が `always`、Bypass のない Ruleset では `never` だった
> - `Integration`：同じリポジトリの既存の Ruleset に登録されていた `actor_id`（`29110`）は `200` で保存された。GitHub Actions の App の ID（`15368`）と存在しない ID は `422`（`Actor ... integration must be part of the ruleset source or owner organization`）だった
> - `DeployKey`：`bypass_mode: always` は `200` で保存され、取得は `actor_id: null` を返した。`actor_id: 1` を送っても `null` で保存された。作成で `actor_id: null` を送ると `201` を返し、`null` で保存された。`bypass_mode: pull_request` は `422`（`deploy key bypass mode must not be 'PULL_REQUEST'`）だった
> - `User`：トークンの所有者の ID は `200` で保存された
> - `Team` と `OrganizationAdmin`：`Team` は `422`（`Actor  team must be part of the ruleset source or owner organization` ほか）、`OrganizationAdmin` は `actor_id` の有無によらず `422`（`ruleset source must be in an organization`）だった
> - `bypass_mode`：`RepositoryRole` の `pull_request` と `exempt` は `200` で保存された。`bypass_mode` を省略すると `always` で保存された
> - 重複：同じ `actor_type` と `actor_id` の要素を二つ送ると、`bypass_mode` が同じでも違っても `200` を返し、先の要素だけを保存した
> - 順序：3つの要素は、送った順のまま返った（1例）。`RepositoryRole`（`5`）、`DeployKey` の順で作成した Ruleset の取得は、`DeployKey`、`RepositoryRole` の順で返した（1例）。送った順を保つとは限らない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |

## 7. ルール

`rules` の要素は、ルールの種類を表す `type` と、種類ごとの `parameters` を持ちます。種類ごとの形は OpenAPI 記述の `repository-rule` の各要素が定めます。

### 7.1 `pull_request`

マージの前に、対象でないブランチからの Pull Request を経ることを求めるルールです。

| `parameters` のフィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `required_approving_review_count` | integer（`0`〜`10`） | 必須 | マージの前に要る承認の数 |
| `dismiss_stale_reviews_on_push` | boolean | 必須 | レビュー可能な新しいコミットの Push で、以前の承認を取り消す |
| `require_code_owner_review` | boolean | 必須 | Code owner が指定されたファイルを変える Pull Request に、承認を求める |
| `require_last_push_approval` | boolean | 必須 | 最後のレビュー可能な Push を、Push した人以外が承認することを求める |
| `required_review_thread_resolution` | boolean | 必須 | コードについての会話をすべて解決してからマージさせる |
| `allowed_merge_methods` | `merge` / `squash` / `rebase` の配列 | 任意 | 許すマージの方法。少なくとも一つを有効にする |
| `dismissal_restriction` | object | 任意 | レビューを取り消せる人、Team、App を限る |
| `require_extra_approval_for_unattributed_changes` | boolean | 任意 | 人間の共同作業者のいない Copilot の Pull Request に、承認の数が0でなければ、もう一つの承認を求める |
| `required_reviewers` | 配列 | 任意 | ファイルのパターンと、そのレビューを求める Team の組 |

Ruleset で使えるルールのドキュメントは、`require_extra_approval_for_unattributed_changes` に当たる設定が、新しい Ruleset でも既存の Ruleset でも既定で有効だと記載しています。ほかの任意のフィールドを省略したときの値は、記載がありません。同じドキュメントは、リポジトリが無効にしたマージの方法と違う方法を Ruleset が求めると、マージが止まると記載しています。その組み合わせを作成・更新で拒否するかは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 必須の5項目だけ：作成と更新は `200` / `201` を返した。応答と取得は、送っていない `allowed_merge_methods: ["merge", "squash", "rebase"]`、`require_extra_approval_for_unattributed_changes: true`、`required_reviewers: []` を加えて返した。`dismissal_restriction` は返らなかった
> - 任意のフィールドの既定値への戻り：`allowed_merge_methods: ["squash"]` と `require_extra_approval_for_unattributed_changes: false` を持つ Ruleset に、必須の5項目だけの `pull_request` を含む `rules` を送ると、二つは上の値に戻った
> - 往復：取得した `parameters` をそのまま `rules` に入れて送る更新の後、Ruleset の取得の本文は `updated_at` を除いて更新の前と一致した
> - 必須の項目の欠落：5項目のうち一つを欠くと `422`（`Invalid property /rules/0: data matches no possible input.`）だった。`parameters` そのものを省略すると `200` を返し、5項目と任意のフィールドはどれも上の既定の値（`required_approving_review_count` は `0`）で保存された。`required_approving_review_count: 3` の Ruleset でも `0` に戻った
> - 未知のキーと `null`：未知のキーを加えると `200` を返し、そのキーは保存されなかった。必須の項目に `null` を送ると `422` だった
> - `required_approving_review_count` の範囲：`0`、`1`、`6`、`10` は `200` で保存された。`-1`、`11`、`100`、文字列の `"1"` は `422` だった
> - `allowed_merge_methods`：`[]` は `422`（`Invalid parameter allowed_merge_methods: Expected at least 1 elements, got 0`）だった。`["rebase", "merge"]`、`["rebase", "squash"]`、`["squash", "merge", "rebase"]`、`["rebase", "merge", "squash"]` は、送った順のまま返った。`["squash", "squash"]` は `200` を返し、二つのまま保存された。定義にない値（`fast_forward`）と `null` は `422` だった
> - `require_extra_approval_for_unattributed_changes`：`false` は保存され、取得で返った。`null` は `422` だった
> - `dismissal_restriction`：個人アカウント所有のリポジトリでは、`enabled` が `false` でも `true` でも、`allowed_actors` が欠けても空でも、`User`（トークンの所有者）、`RepositoryRole`（`2`、`5`）、`Team`（`id: 1`）、`IntegrationInstallation` を含んでも、`200` を返した。値は、どの場合も Ruleset の取得に現れなかった。`User` を送ったあとの、ブランチに適用されるルールの取得（`GET .../rules/branches/{branch}`）と Ruleset の履歴の版の取得（`GET .../rulesets/{ruleset_id}/history/{version_id}`）にも現れなかった。GraphQL API の `PullRequestParameters` の `dismissalRestriction` は、`User` と `RepositoryRole` を送った直後でも `enabled: false`、`allowedActors: []` を返した（確認の前からある Ruleset も同じ）。送った値は、REST API と GraphQL API のどちらでも観測できなかった。`enabled` を欠く値は `422` だった
> - `required_reviewers`：`Team` の `id: 1` を持つ要素は `422`（`Invalid node ID format, Reviewer 1 not found, Repository must be in an organization`）だった。`[]` は `200` だった
> - リポジトリのマージ方式との組み合わせ：`allow_squash_merge` が `false` のリポジトリで、`allowed_merge_methods: ["squash"]` の Ruleset の作成と更新は `201` / `200` を返し、送った値で保存された（確認用のブランチを対象にした `active` と `disabled`、`~DEFAULT_BRANCH` を対象にした `active` で確認）。`allow_squash_merge` と `allow_rebase_merge` が `false` のリポジトリで、`["squash", "rebase"]` にする更新も `200` だった。逆に、`allowed_merge_methods: ["squash"]` の `active` の Ruleset があるリポジトリで、`PATCH /repos/{owner}/{repo}` で `allow_squash_merge` を `false` にする更新は `200` で反映され、Ruleset は変わらなかった。`allow_rebase_merge` が `false` のリポジトリでも、`allowed_merge_methods` を送らない `PUT` は、`["merge", "squash", "rebase"]` で保存した（リポジトリで無効の方式を外さなかった）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |
| `grounding` | [Available rules for rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets) | External Document |

### 7.2 `deletion`

Bypass の権限を持つ利用者にだけ、対象の ref の削除を許すルールです。`parameters` を持ちません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 取得の形：`{"type": "deletion"}` を返し、`parameters` のフィールドはなかった
> - `parameters` を送ったとき：`parameters` に `{}`、`{"x": 1}`、`null` を送ると、どれも `200` を返し、`parameters` は保存されなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |

### 7.3 `required_status_checks`

対象の ref を更新する前に、指定したステータスチェックが成功していることを求めるルールです。コミットは、先にチェックが成功する別の ref へ Push しておく必要があります。

| `parameters` のフィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `required_status_checks` | 下の要素の配列 | 必須 | 必須のステータスチェック |
| `strict_required_status_checks_policy` | boolean | 必須 | 対象のブランチへの Pull Request を最新のコードで検証することを求める。チェックが一つもなければ効かない |
| `do_not_enforce_on_create` | boolean | 任意 | チェックが禁じる場合でも、リポジトリとブランチの作成を許す |

| `required_status_checks` の要素のフィールド | 型・値 | 指定条件 | 意味 |
|---|---|---|---|
| `context` | 文字列 | 必須 | コミットにあるべきステータスチェックの名前 |
| `integration_id` | integer | 任意 | ステータスチェックの発行元に限る App の ID |

Ruleset で使えるルールのドキュメントは、発行元に指定する App が、`statuses:write` の権限でリポジトリにインストールされ、最近チェックの実行を送っていることを求めると記載しています。発行元を指定しなければ、どの発行元のステータスも受け付けます。任意のフィールドを省略したときの値と、配列の順序と重複の扱いは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 取得の形：`parameters` は3つのフィールドをすべて返した。`integration_id` を送らなかった要素は、取得でこのフィールドを持たなかった（`null` も返さなかった）
> - `do_not_enforce_on_create`：省略した作成は `false` で保存された。`true` は保存された。`true` の Ruleset に、このフィールドを省略した `rules` を送ると `false` に戻った。`rules` を省略した更新では `true` が保持された
> - 往復：取得した `rules` をそのまま送る更新の後、Ruleset の取得の本文は `updated_at` を除いて更新の前と一致した（このルールだけの例と、`deletion`、`non_fast_forward`、既定でない任意のパラメータを持つ `pull_request` と組み合わせた例）
> - 必須の項目の欠落：`required_status_checks`、`strict_required_status_checks_policy`、要素の `context` のどれかを欠くと `422`（`Invalid property /rules/0: data matches no possible input.`）だった。`parameters` そのものを省略した作成と更新は `201` / `200` を返し、`required_status_checks: []`、`strict_required_status_checks_policy: false`、`do_not_enforce_on_create: false` で保存された
> - `null` と型違い：`strict_required_status_checks_policy`、`do_not_enforce_on_create`、`integration_id` の `null` と、文字列の `integration_id`（`"15368"`）は `422` だった
> - 空の配列：`required_status_checks: []` は、`strict_required_status_checks_policy` が `false` でも `true` でも `200` を返し、`[]` で保存された
> - `context`：空文字列は `422`（`Invalid array contents. Errors at index 0: Expected context to be present`）だった。前後に半角空白を付けた名前は、送ったとおりに保存された。大文字小文字だけが違う `CI/A` と `ci/a` は、別の要素として二つとも保存された
> - `integration_id`：GitHub Actions の App の ID（`15368`）と Dependabot の App の ID（`29110`）は `200` で保存された。リポジトリにインストールされていない App（Codecov の `254`、Renovate の `2740`）、`0`、存在しない ID は `422`（`Invalid parameter required_status_checks: Invalid integration ids`）だった
> - 未知のキー：`parameters` の未知のキーは `422`（`Unexpected parameter`）、要素の未知のキーも `422`（`Invalid array contents. Errors at index 0: Unexpected parameter`）だった。`pull_request` と違い、黙って捨てなかった
> - 要素の重複：同じ `context` の要素を二つ送ると、`integration_id` がどちらにもない場合、同じ場合、違う場合、片方だけにある場合のどれも `200` を返し、二つのまま保存された
> - 要素の順序：作成と更新の応答と取得は、送った順を保った（5例）
> - 同じ種類のルール：`required_status_checks` を二つ送ると `422` を返し、`errors` は `["Invalid rule 'required_status_checks': "]` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |
| `grounding` | [Available rules for rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets) | External Document |

### 7.4 `non_fast_forward`

Push の権限を持つ利用者に、対象の ref への Force push を禁じるルールです。`parameters` を持ちません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 取得の形：`{"type": "non_fast_forward"}` を返し、`parameters` のフィールドはなかった
> - `parameters` を送ったとき：`parameters` に `{}`、`{"x": 1}`、`null` を送ると、どれも `200` を返し、`parameters` は保存されなかった
> - 同じ種類のルール：`non_fast_forward` を二つ送ると `200` を返し、一つにまとめて保存した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset) | External Document |
