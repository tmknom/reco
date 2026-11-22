# GitHub Actions 変数 API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for GitHub Actions variables」のうち、reco が利用するリポジトリ単位のエンドポイントの外部契約を整理します。対象は、リポジトリの変数（Repository Variables）と、Environment ごとの変数（Environment Variables）の一覧・取得・作成・更新・削除に必要な権限、HTTP 応答、名前と値の規則、フィールドです。Organization の変数は扱いません。Environment そのものは [GitHub デプロイ環境 API 仕様](deployment-environments.md) が、すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

どのエンドポイントも、認証した利用者がリポジトリの collaborator であることを求め、OAuth app のトークンと personal access token (classic) には、リポジトリの公開範囲によらず `repo` scope が必要と記載されています。GitHub App の権限の一覧では、Repository Variables のエンドポイントは Variables、Environment Variables のエンドポイントは Environments に分類され、どちらも取得が read、作成・更新・削除が write です。

| 用途 | メソッド・パス | GitHub App の権限 |
|---|---|---|
| Repository Variables の一覧 | `GET /repos/{owner}/{repo}/actions/variables` | Variables: read |
| Repository Variable の作成 | `POST /repos/{owner}/{repo}/actions/variables` | Variables: write |
| Repository Variable の取得 | `GET /repos/{owner}/{repo}/actions/variables/{name}` | Variables: read |
| Repository Variable の更新 | `PATCH /repos/{owner}/{repo}/actions/variables/{name}` | Variables: write |
| Repository Variable の削除 | `DELETE /repos/{owner}/{repo}/actions/variables/{name}` | Variables: write |
| Environment Variables の一覧 | `GET /repos/{owner}/{repo}/environments/{environment_name}/variables` | Environments: read |
| Environment Variable の作成 | `POST /repos/{owner}/{repo}/environments/{environment_name}/variables` | Environments: write |
| Environment Variable の取得 | `GET /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}` | Environments: read |
| Environment Variable の更新 | `PATCH /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}` | Environments: write |
| Environment Variable の削除 | `DELETE /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}` | Environments: write |

| 操作 | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| 一覧（`GET .../variables`） | `200` | 記載なし |
| 作成（`POST .../variables`） | `201` | 記載なし |
| 取得（`GET .../variables/{name}`） | `200` | 記載なし |
| 更新（`PATCH .../variables/{name}`） | `204` | 記載なし |
| 削除（`DELETE .../variables/{name}`） | `204` | 記載なし |

パスの `environment_name` は URL エンコードして渡すと記載されています（`/` は `%2F`）。Environment が存在しないとき、変数がないとき、権限がないときの応答は記載がありません。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つ personal access token (classic) で実際の動作を確認した事項です（公開リポジトリと、非公開リポジトリで確認）。
>
> - 権限：自分の公開・非公開リポジトリでは、両方の種類の変数の一覧、取得、作成、更新、削除がどれも正常応答を返した。応答ヘッダー `X-Accepted-OAuth-Scopes` は空だった
> - 権限のない第三者の公開リポジトリ（`cli/cli`）：Repository Variables の一覧と取得は `403` を返し、本文の `message` は `You must have repository read permissions or have the repository variables fine-grained permission.` だった。Environment Variables の一覧は `403` で、`message` は `Must have admin rights to Repository.` だった
> - 存在しないリポジトリ：Repository Variables の一覧は `404`（`Not Found`）を返した
> - 存在しない変数：存在しない名前への取得、更新、削除は、どちらの種類でも `404` を返した。取得と更新の本文は `message` が `Not Found` の Basic Error で、削除の本文は `{}` だった。更新の `404` のあと、その名前の変数は作られていなかった
> - 存在しない Environment：Environment Variables の一覧と取得は `404`（`Not Found`）を返し、Environment は作られなかった。作成も `404` を返し、Environment は作られなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for GitHub Apps](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps?apiVersion=2026-03-10) | External Document |

### 1.1 利用できるリポジトリ

Environment Variables は、すべての公開リポジトリで利用でき、非公開リポジトリでは GitHub Pro か GitHub Team のプランの利用者が利用できると記載されています。利用できないリポジトリでの API の応答は記載がありません。Repository Variables の利用条件は、参照したページに記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 非公開リポジトリ：利用者によると GitHub Free のアカウントが所有する非公開リポジトリで、Environment Variables の一覧、作成、取得、更新、削除がどれも正常応答を返し、作成した変数は一覧と取得に現れた。Repository Variables も同じだった。API の応答からは料金プランを確かめられなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Deployments and environments: Environment variables](https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments#environment-variables) | External Document |

## 2. 変数の名前

変数の名前は、英数字（`[a-z]`、`[A-Z]`、`[0-9]`）と `_` だけからなり、空白を含まず、`GITHUB_` で始まらず、数字で始まらないと記載されています。参照するときは大文字と小文字を区別せず、GitHub は入力の大文字小文字によらず名前を大文字で保存すると記載されています（この記述は「secret names」と書いている）。名前は、作られたリポジトリ、Organization、Enterprise の中で一意でなければなりません。名前の長さの上限、API がパスの名前をどう比べるか、応答の名前がどの綴りかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認。大文字小文字、規則に反する名前の一部、長さの上限は Environment Variables でも確認）。
>
> - 小文字の名前：`reco_check_lower` で作成すると、一覧には `RECO_CHECK_LOWER` として現れた。小文字のパスと大文字小文字を混ぜたパスへの取得、小文字のパスへの更新は、この変数に当たった
> - 大文字小文字だけが違う名前：`RECO_CHECK_LOWER` の作成は `409` を返した（5.1）
> - 前後の空白：末尾に空白をつけた名前（`RECO_CHECK_LOWER%20`）への取得は `200` を返し、本文の `name` は末尾に空白のついた `RECO_CHECK_LOWER ` だった。値は `RECO_CHECK_LOWER` のものだった。先頭の空白、末尾のタブをつけた名前への取得は `404` だった。一覧には空白のついた名前は現れなかった
> - 規則に反する名前：数字で始まる名前、`-`、空白、`.`、`É` を含む名前の作成は `422` を返し、`message` は `Variable names can only contain alphanumeric characters ([a-z], [A-Z], [0-9]) or underscores (_). Spaces are not allowed. Must start with a letter ([a-z], [A-Z]) or underscores (_).` だった。`GITHUB_` で始まる名前は、小文字の `github_` でも `422`（`Variable names must not start with GITHUB_.`）だった。空の名前は `422`（`Variable name cannot be empty.`）だった。どれも変数は作られなかった
> - `_` で始まる名前：`_RECO_CHECK_UNDERSCORE` は作成できた
> - 長さ：311文字の名前の作成は `422`（`Variable key maximum size is 300 characters.`）を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Variables reference: Naming conventions for configuration variables](https://docs.github.com/en/actions/reference/workflows-and-actions/variables#naming-conventions-for-configuration-variables) | External Document |

## 3. 変数の値

値は一つの変数あたり 48 KB に制限されると記載されています。保存できる変数の数は、リポジトリあたり500件、Environment あたり100件です。ワークフローの実行1回で使える Organization と Repository の変数の合計は 256 KB までと記載されています。上限を数える単位（文字かバイトか）、空文字列を値にできるか、値を正規化するか（前後の空白、改行）は記載がありません。

GitHub は Variables を、機密でない設定情報を保存して再利用するためのものとしています（「Variables provide a way to store and reuse non-sensitive configuration information.」）。値はビルドの出力でマスクされずに表示され、パスワードのような秘密情報には Secrets を使うよう記載されています（「By default, variables render unmasked in your build outputs. If you need greater security for sensitive information, such as passwords, use secrets instead.」）。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認。上限と空文字列、前後の空白、CRLF、マルチバイト文字の往復は Environment Variables でも確認）。
>
> - 上限：値の UTF-8 で 48,000 バイトまでの作成は `201` を返し、48,001 バイト以上は `422`（`Value is too large.`）を返した。ASCII 48,000 文字、2バイトの文字 24,000 文字、3バイトの文字 16,000 文字、U+0001 48,000 文字が、どれもちょうど上限だった。改行は1文字を1バイトと数え、48,000 文字は受け付けられ、48,002 文字は拒まれた（48,001 文字は試していない）。更新でも、上限を超える値（49,152 バイト）は同じ `422` で、値は変わらなかった
> - 空文字列：作成も更新も `422`（`Variable value cannot be empty.`）を返し、更新の場合は値が変わらなかった
> - 値の往復：前後の空白、改行だけの値、末尾の改行、CRLF、タブ、U+00A0 と U+3000、絵文字を含むマルチバイト文字、引用符と `\`、U+0001、U+0000 を含む値は、取得と一覧のどちらでも送ったとおりに返った。ESC、BEL、U+202E を含む Environment Variable の値も、一覧で送ったとおりに返った
> - 応答の表現：応答本文の JSON では、ASCII 以外の文字は UTF-8 のまま、改行は `\n`、U+0001 は `\u0001` で表された。U+0001 だけからなる 48,000 バイトの値の取得の本文は 288,112 バイトだった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Variables reference: Limits for configuration variables](https://docs.github.com/en/actions/reference/workflows-and-actions/variables#limits-for-configuration-variables) | External Document |
| `grounding` | [Variables](https://docs.github.com/en/actions/concepts/workflows-and-actions/variables) | External Document |

## 4. 変数の一覧と取得

### 4.1 `GET .../actions/variables`、`GET .../environments/{environment_name}/variables`

- 用途：リポジトリの変数、または Environment の変数の一覧の取得
- クエリパラメータ：`per_page`（既定 `10`、最大 `30`）、`page`（既定 `1`）

| 応答フィールド | 型 | 必須 |
|---|---|---|
| `total_count` | integer | 必須 |
| `variables` | 4.2 の変数の配列 | 必須 |

要素の並び順、ページの終わりの判定方法、最大を超える `per_page` の扱いは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで、どちらの種類も変数が32件の状態で確認。0件と1件は非公開リポジトリでも確認）。
>
> - ページング：`per_page` に `31` と `100` を送ると、`30` と同じく1ページ30件を返した。`per_page=100` の2ページ目は31件目と32件目を返した。どのページの `total_count` も全件数で、ページをまたいだ名前の重複と欠けはなく、最後のページには `Link` の `next` がなく、その次のページは `200` と空の配列を返した。`Link` の URL は [GitHub デプロイ環境 API 仕様](deployment-environments.md) 3.1 の一覧と同じく `/repositories/{repository_id}/...` の形だった
> - 網羅：作成した変数はすべて一覧に現れた。作成、更新、削除の約1秒以内の一覧は、どれも更新後の状態を返した。作成と値の更新の直後に一覧を送る確認を両方の種類で10回ずつ繰り返しても、作成した変数を含まない一覧と、更新前の値を返す一覧はなかった
> - 並び順：名前の昇順（バイト順。数字、大文字、`_` の順）だった
> - 要素：一覧の要素は `name`、`value`、`created_at`、`updated_at` だけを持ち、ID を持たなかった。32件すべてで、要素は個別取得の本文と全フィールド一致した
> - 応答本文の大きさ：`curl` の既定と異なる `User-Agent` で受け取った空白のない本文では、短い値の要素は約115バイトだった。48,000 バイトの値の変数を28件含むページは 1,347,400 バイト、30件からなるページは 1,443,392 バイトだった（30件は非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [List repository variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#list-repository-variables) | External Document |
| `grounding` | [List environment variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#list-environment-variables) | External Document |

### 4.2 `GET .../variables/{name}`

- 用途：一つの変数の取得

| 応答フィールド | 型 | 必須 |
|---|---|---|
| `name` | 文字列 | 必須 |
| `value` | 文字列 | 必須 |
| `created_at` | 文字列（date-time） | 必須 |
| `updated_at` | 文字列（date-time） | 必須 |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 応答の `name`：小文字のパスへの取得は大文字の `name` を返したが、末尾に空白をつけたパスへの取得は空白のついた `name` を返した（2章）。保存されている名前と異なる `name` を返すことがある

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#get-a-repository-variable) | External Document |
| `grounding` | [Get an environment variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#get-an-environment-variable) | External Document |

### 4.3 Environment の名前

Environment Variables のエンドポイントのパスの `environment_name` を、GitHub がどう比べるかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 大文字小文字と末尾の空白：`reco-check-026` の Environment に対し、`RECO-CHECK-026` への一覧と作成は同じ Environment の変数に当たり、新しい Environment は作られなかった。`reco-check-026 a/b` の Environment に対し、`RECO-CHECK-026 A/B` への取得と、末尾に空白をつけた名前への一覧も、同じ Environment の変数に当たった。[GitHub デプロイ環境 API 仕様](deployment-environments.md) 2章の Environment のエンドポイントと同じ動作である
> - エンコード：`%20` と `%2F` にエンコードした名前で、一覧、作成、取得ができた。`/` をエンコードしない名前の一覧は `404` を返した
> - Environment の作り直し：変数を持つ Environment を削除すると、その名前の変数の一覧は `404` を返した。同じ名前で作り直した Environment は別の `id` を持ち、変数の一覧は直後も約3秒後も `total_count: 0` だった。削除前の変数の取得は `404` だった
> - 初めて作成した Environment：一度も作ったことのない名前の Environment を `PUT` で作成した直後に、変数の一覧は `200` で `{"variables":[],"total_count":0}` を返し、続く変数の作成は `201` を返した（非公開リポジトリで確認）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10) | External Document |

## 5. 変数の作成、更新、削除

### 5.1 `POST .../variables`

- 用途：変数の作成

| 本文フィールド | 型 | 指定条件 | 意味 |
|---|---|---|---|
| `name` | 文字列 | 必須 | 変数の名前 |
| `value` | 文字列 | 必須 | 変数の値 |

同じ名前の変数がすでにあるときの応答は記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで両方の種類を確認。作成の `201` は非公開リポジトリでも確認）。
>
> - 作成の応答：`201` を返し、本文は `{}` で、`Location` はなかった
> - 同じ名前の作成：すでにある名前（大文字小文字だけが違う名前を含む）の作成は `409` を返し、`message` は `Already exists - Variable already exists` だった。既存の値と `updated_at` は変わらなかった
> - 応答のない作成：作成の1件が、要求を送ってから10分以上応答を返さなかった。打ち切ったあとの一覧には、その変数が作られていた

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Create a repository variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#create-a-repository-variable) | External Document |
| `grounding` | [Create an environment variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#create-an-environment-variable) | External Document |

### 5.2 `PATCH .../variables/{name}`

- 用途：変数の更新

| 本文フィールド | 型 | 指定条件 | 意味 |
|---|---|---|---|
| `name` | 文字列 | 任意 | 変数の名前 |
| `value` | 文字列 | 任意 | 変数の値 |

省略したフィールドの扱いと、本文の `name` がパスの名前と違うときの扱いは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで両方の種類を確認。`value` だけの更新は非公開リポジトリでも確認）。
>
> - `value` だけの更新：`204` を返し、本文はなかった。直後の取得と一覧では、値が送った値になり、`name` と `created_at` は変わらず、`updated_at` が進んだ
> - 同じ値の更新：現在と同じ値を送っても `204` を返し、`updated_at` は進んだ
> - パスと同じ `name` を含む更新：`204` を返し、値だけが変わった
> - 空の本文：`{}` は `422`（`Failed to add variable. The variable key and value are empty.`）を返し、値は変わらなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#update-a-repository-variable) | External Document |
| `grounding` | [Update an environment variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#update-an-environment-variable) | External Document |

### 5.3 `DELETE .../variables/{name}`

- 用途：変数の削除

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで両方の種類を確認。非公開リポジトリでも確認）。
>
> - 削除の応答：`204` を返し、本文はなかった。直後の取得は `404` で、直後の一覧は削除した変数を含まなかった
> - 削除済みの変数：同じ名前への2回目の削除は `404` を返し、本文は `{}` だった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Delete a repository variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#delete-a-repository-variable) | External Document |
| `grounding` | [Delete an environment variable](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10#delete-an-environment-variable) | External Document |

### 5.4 ほかの変数と Environment への影響

一つの変数の作成、更新、削除が、ほかの変数や Environment の設定を変えるかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（公開リポジトリで確認）。
>
> - 同じ一覧のほかの変数：同じ名前の作成（`409`）、更新、削除の前後で一覧の全体を比べ、操作した変数以外の要素は `updated_at` を含めて変わらなかった（両方の種類）
> - 同じ名前の Repository Variable と Environment Variable：同じ名前で両方を作成でき、一方の更新と削除は、もう一方の値と `updated_at` を変えなかった
> - Environment：Environment Variables の作成、更新、削除を約40回送った前後で、その Environment の `id` と `updated_at` は変わらなかった。別の Environment（`staging`）と、その変数は、`updated_at` を含めて変わらなかった。Environment Variables の操作の前後で、Repository Variables も変わらなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10) | External Document |
