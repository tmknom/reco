# GitHub REST API 共通仕様

本ドキュメントは、reco が利用するすべての GitHub REST API エンドポイントに共通する外部契約を整理します。対象は、リクエストの形式、API バージョン、リダイレクト、失敗応答、レート制限、条件付きリクエスト、認証とトークンです。エンドポイントごとの権限・応答・フィールドは、エンドポイントの文書が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. リクエストの URL・ヘッダー・パスパラメータ

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

## 2. API バージョン

サポート対象の API バージョンは、`2026-03-10`（サポート終了日は未定）と `2022-11-28`（2028-03-10 にサポート終了）です。新しいバージョンのリリース後も、以前のバージョンは少なくとも24か月サポートされます。サポートが終了したバージョンを指定すると `410 Gone` が返ります。バージョン間の破壊的変更は、GitHub の破壊的変更の一覧に記載されます。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [API Versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions) | External Document |
| `grounding` | [Breaking changes](https://docs.github.com/en/rest/about-the-rest-api/breaking-changes) | External Document |

## 3. リダイレクト

GitHub は、どのリクエストもリダイレクトになり得ると想定し、リダイレクトに追従するよう推奨しています。`301` は恒久的な移動を表し、`location` ヘッダーの URL へリクエストを繰り返したうえで、以後もその URL を使うよう求めます。`302` と `307` は一時的な移動を表し、`location` ヘッダーの URL へリクエストを繰り返すものの、以後に使う URL は変えないよう求めます。ほかのリダイレクト用ステータスコードが HTTP 仕様に従って使われる場合もあります。

一方、公開ドキュメントには、リダイレクト先へ認証情報を送ってよいか、更新系メソッド（`PATCH`・`PUT` など）をリダイレクト先へ再送したときに何が起きるかの記載がありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Best practices for using the REST API: Follow redirects](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#follow-redirects) | External Document |

## 4. 失敗応答の解釈

適切に認証されていないリクエストが非公開リソースにアクセスすると、GitHub は非公開リポジトリの存在を確認させないため、`403 Forbidden` ではなく `404 Not Found` を返します。エンドポイントが対応していない HTTP メソッドを使った場合も、`405` ではなく `404` が返ります。したがって、`404` だけでは対象の不存在を断定できません。

認証の失敗は、状況によって異なるステータスコードで報告されます。無効な認証情報を使うと、最初は `401 Unauthorized` が返ります。トークンを付けない場合や、トークンの権限が不足している場合は、`404 Not Found` または `403 Forbidden` が返ります。短時間に無効な認証情報で繰り返しリクエストすると、GitHub はそのユーザーの認証を有効な認証情報も含めて一時的にすべて拒否し、`403 Forbidden` を返します。

GitHub は、存在するはずのリソースに `404` が返るときの確認事項を、トークン種別ごとに挙げています。主な項目は、トークンの scope と権限、トークン所有者の権限、アクセスできるリポジトリの範囲、トークンの有効期限と失効です。

GitHub App や fine-grained personal access token で「Resource not accessible by integration」や「Resource not accessible by personal access token」のエラーを受けたときは、応答の `X-Accepted-GitHub-Permissions` ヘッダーで必要な権限を確認できます。

各エンドポイントのドキュメントに列挙された応答コードには、共通仕様によって返るコードが含まれていない場合があります。たとえば、レート制限の超過（5章）で返る `429` や、認証失敗で返る `401` は、エンドポイントの応答コード一覧にないことがあります。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Authenticating to the REST API: Failed login limit](https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api#failed-login-limit) | External Document |
| `grounding` | [Troubleshooting the REST API: 404 Not Found for an existing resource](https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api#404-not-found-for-an-existing-resource) | External Document |
| `grounding` | [Troubleshooting the REST API: Resource not accessible](https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api#resource-not-accessible) | External Document |

## 5. レート制限

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

## 6. 条件付きリクエスト

多くのエンドポイントは `etag` ヘッダーを返し、`last-modified` ヘッダーを返すエンドポイントも多くあります。これらを使った条件付きリクエストで対象が変わっていなければ `304 Not Modified` が返ります。`304` が返ったリクエストは、`Authorization` ヘッダーで正しく認証していれば、プライマリ制限の枠を消費しません。

ただし、`POST`・`PUT`・`PATCH`・`DELETE` などの更新系メソッドの条件付きリクエストは、エンドポイントごとのドキュメントに記載がある場合を除いて **サポートされません** 。したがって、条件付きリクエストに対応する旨の記載がない更新エンドポイントでは、取得から更新までの間に他者が設定を変更したかを検出する手段が、公開ドキュメントにありません。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Best practices for using the REST API: Use conditional requests](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#use-conditional-requests) | External Document |

## 7. 認証とトークン

REST API へのアクセスに使うトークンは、種別によって権限の表し方が異なります。OAuth app のトークンと personal access token (classic) は scope で、fine-grained personal access token は repository permissions で、アクセスできる範囲を表します。どの種別でも、実際にアクセスできるかは、トークンの scope や permissions に加えて、トークン所有者のリポジトリに対するロールによって変わり得ます。

### 7.1 GitHub CLI が発行・保存するトークン

`gh auth login` は、既定ではブラウザを使う認証フローでトークンを取得し、システムの資格情報ストアに保存します。資格情報ストアを使えない場合は、平文のファイルに保存します。`gh auth token` は、指定したホスト（省略時は既定のホスト）のアクティブなアカウントのトークンを出力し、更新可能で期限切れ間近のトークンは出力前に自動で更新します。

GitHub CLI が求めるトークンの最小の scope は `repo`、`read:org`、`gist` です。GitHub CLI のドキュメントはこの最小 scope を `--with-token` で渡すトークンの条件として示しており、ブラウザを使う認証フローで得られるトークンの種別と scope を明記していません。fine-grained personal access token を GitHub CLI で使う場合は、`--with-token` ではなく環境変数 `GH_TOKEN` で渡すよう推奨しています。

トークンが持つ scope は、そのトークンで送ったリクエストの応答ヘッダー `X-OAuth-Scopes` で確認できます。エンドポイントが確認する scope は `X-Accepted-OAuth-Scopes` ヘッダーに示されます。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [gh auth login](https://cli.github.com/manual/gh_auth_login) | External Document |
| `grounding` | [gh auth token](https://cli.github.com/manual/gh_auth_token) | External Document |
| `grounding` | [Scopes for OAuth apps](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps) | External Document |

### 7.2 OAuth app のトークンと personal access token (classic) の scope

scope は OAuth のトークンがアクセスできる範囲を制限するだけで、ユーザーがすでに持つ権限を超える権限を与えません。reco の利用するエンドポイントに関係する scope は次のとおりです。

| scope | 公開ドキュメントの記載 |
|---|---|
| （scope なし） | 公開情報（ユーザーのプロフィール、リポジトリの情報、gist を含む）への読み取り専用のアクセスを与える |
| `repo` | 公開・非公開リポジトリへの完全なアクセス（コードやコミットステータスの読み書きを含む）を与える |
| `public_repo` | アクセスを公開リポジトリに限定する |

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Scopes for OAuth apps](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps) | External Document |
| `grounding` | [Authenticating to the REST API: Authenticating with a personal access token](https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api#authenticating-with-a-personal-access-token) | External Document |

### 7.3 トークン所有者のリポジトリロール

Organization が所有するリポジトリのロールのうち、リポジトリの設定を変更できるのは Admin だけです。Pull Request の自動マージを個々の Pull Request で有効・無効にする操作は、Write・Maintain・Admin に許可されています。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Repository roles for an organization](https://docs.github.com/en/organizations/managing-user-access-to-your-organizations-repositories/managing-repository-roles/repository-roles-for-an-organization) | External Document |
