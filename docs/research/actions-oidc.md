# GitHub Actions OIDC API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for GitHub Actions OIDC」のうち、リポジトリ単位の OIDC トークンの subject クレーム（`sub`）のカスタマイズのエンドポイントの外部契約を整理します。対象は、カスタマイズの取得・設定に必要な権限、HTTP 応答、フィールドと、subject の形の決まり方です。Organization と Enterprise の単位のエンドポイントと、リポジトリのカスタムプロパティを OIDC のクレームに含めるエンドポイントは対象外です。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

エンドポイントのドキュメントは、OAuth のトークンと personal access token (classic) に `repo` scope が必要だと記載しています。fine-grained personal access token に必要な repository permissions は次のとおりです。

| 用途 | メソッド・パス | Fine-grained token permissions |
|---|---|---|
| subject のカスタマイズの取得 | `GET /repos/{owner}/{repo}/actions/oidc/customization/sub` | Actions: read |
| subject のカスタマイズの設定 | `PUT /repos/{owner}/{repo}/actions/oidc/customization/sub` | Actions: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET .../actions/oidc/customization/sub` | `200` | `400` 不正な要求、`404` リソースが見つからない |
| `PUT .../actions/oidc/customization/sub` | `201` 本文が空の応答 | `400` 不正な要求、`404` リソースが見つからない、`422` 検証の失敗 |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（`repo` scope を持つ personal access token (classic) で確認）。
>
> - 権限のないリポジトリへの取得：トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリ（`cli/cli`）でも、取得は `200` で、カスタマイズの内容（`include_claim_keys` を含む）を返した
> - 存在しないリポジトリへの取得：`404` だった
> - 設定の正常応答：`201` で、本文は `{}` だった
> - Actions が無効のとき：個人アカウントが所有する非公開リポジトリで Actions を無効にしても、取得は `200` で無効にする前の値を返し、設定も `201` を返して反映された

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [REST API endpoints for GitHub Actions OIDC](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Actions](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-actions) | External Document |

## 2. subject クレームの形

GitHub Actions のワークフローは、OIDC トークンを外部のクラウドなどへ示して認証を受けられます。受け取る側は、トークンの `sub` などのクレームを信頼の条件と照らし合わせます。OIDC のリファレンスのドキュメントは、`sub` が既定ではワークフローの主なメタデータを決まった形でつないだ値だと記載し、`repo:octo-org/octo-repo:environment:Production` や `repo:octo-org/octo-repo:ref:refs/heads/demo-branch` を例に挙げています。リポジトリの後ろに続く部分（`environment:Production` など）を **コンテキスト** と呼びます。

カスタマイズでは、`sub` を組み立てるクレームの key を並べて指定します。カスタマイズすると `sub` 全体が新しい形になります。ドキュメントは、リポジトリの key を `repository` ではなく `repo` と書くこと、コンテキストを `context` で指定できること、値の中のコロンを `%3A` に置き換えることを記載しています。`sub` の中の並びが key の指定の順になると明記した記載はありませんが、例はどれも指定の順に並んでいます（`["repo", "context", "job_workflow_ref"]` が `repo:octo-org/octo-repo:environment:prod:job_workflow_ref:...` になる）。指定できる key の一覧はなく、OIDC の設定の `claims_supported` を参照するよう案内しています。カスタムプロパティのクレームは `repo_property_` の接頭辞で指定します。ドキュメントは、カスタマイズの設定の手段として REST API だけを挙げ、リポジトリでカスタマイズをやめて既定の形に戻す手段も REST API だと記載しています。

OIDC のリファレンスのドキュメントは、リポジトリの名前ではなく ID にもとづく **immutable subject** の形も記載しています（`repo:OWNER@OWNER-ID/REPO@REPO-ID:ref:refs/heads/BRANCH`）。2026年7月15日より後に作られたリポジトリは、既定でこの形を使います。それより前のリポジトリは、Organization かリポジトリの単位で選んだときに限りこの形になり（選ぶ手段は OIDC の設定画面か REST API と記載され、設定画面のページ名は記載がない）、2026年7月15日より後に名前を変えたか移転したリポジトリもこの形に移ります。changelog は、2026年7月15日から新しいリポジトリと名前の変更にこの形を自動で強制すると記載しています。強制されたリポジトリで `use_immutable_subject` を `false` にできるかは、どちらの資料にも記載がありません。immutable subject とカスタマイズを併用すると、`repo` の部分に所有者とリポジトリの ID が常に含まれます。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - 受け付ける key：公開リポジトリで、`claims_supported` の key と `repo`、`context` を一つずつ送ると、次の28個は受け付けられた。`actor`、`actor_id`、`aud`、`base_ref`、`check_run_id`、`context`、`environment`、`environment_node_id`、`event_name`、`head_ref`、`iss`、`job_workflow_ref`、`job_workflow_sha`、`ref`、`ref_type`、`repo`、`repository`、`repository_id`、`repository_owner`、`repository_owner_id`、`repository_visibility`、`run_attempt`、`run_id`、`run_number`、`runner_environment`、`workflow`、`workflow_ref`、`workflow_sha`
> - 受け付けない key：`claims_supported` にあっても、`sub`、`exp`、`iat`、`jti`、`nbf`、`sha`、`enterprise`、`enterprise_id`、`ref_protected`、`issuer_scope` の10個は `400`（`The template has one or more unsupported claim keys.`）だった。`enterprise` と `enterprise_id` は、個人アカウントが所有するリポジトリで確かめた結果である。`repo_property_` で始まる key は `400`（`Repository property claims require an organization context for validation.`）だった
> - 2026年7月15日より前に作られ、そのあとに名前を変えたリポジトリの immutable subject：検証に使った非公開リポジトリは2026年7月15日より前に作られ、その日より後に一度名前を変えて元の名前に戻した。取得の `use_immutable_subject` は `true` で、`sub_claim_prefix` は ID を含む形だった。名前の変更の前の値は記録していない。このリポジトリでは `false` にできなかった（4章）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc) | External Document |
| `grounding` | [GitHub Actions OIDC configuration (claims_supported)](https://token.actions.githubusercontent.com/.well-known/openid-configuration) | External Document |
| `grounding` | [Immutable subject claims for GitHub Actions OIDC tokens](https://github.blog/changelog/2026-04-23-immutable-subject-claims-for-github-actions-oidc-tokens/) | External Document |

## 3. `GET /repos/{owner}/{repo}/actions/oidc/customization/sub`

- 用途：リポジトリの subject のカスタマイズの取得

| 応答フィールド | 型 | 必須 | 意味 |
|---|---|---|---|
| `use_default` | boolean | 必須 | 既定の形を使うか。`true` なら `include_claim_keys` は無視される |
| `include_claim_keys` | array of string | 任意 | `sub` を組み立てるクレームの key の並び |
| `use_immutable_subject` | boolean | 任意 | immutable subject の形を選んでいるか。リポジトリで設定していなければ、Organization の設定に従う |
| `sub_claim_prefix` | string | 任意 | このリポジトリの現在の `sub` の接頭辞 |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（個人アカウントが所有する公開・非公開リポジトリで確認）。
>
> - `use_default` が `true` のとき：`include_claim_keys` は応答に含まれなかった。`use_immutable_subject` と `sub_claim_prefix` は含まれた
> - `use_default` が `false` のとき：`include_claim_keys` は、保存された key が1つ以上あれば含まれ、なければ含まれなかった（4章の空の並び）。key は設定で送った順に返った
> - `use_immutable_subject` は、確かめたどの取得にも含まれた。`sub_claim_prefix` は、`false` なら `repo:<所有者>/<リポジトリ>`、`true` なら `repo:<所有者>@<所有者の ID>/<リポジトリ>@<リポジトリの ID>` で、`use_default` と `include_claim_keys` によらなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get the customization template for an OIDC subject claim for a repository](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10#get-the-customization-template-for-an-oidc-subject-claim-for-a-repository) | External Document |

## 4. `PUT /repos/{owner}/{repo}/actions/oidc/customization/sub`

- 用途：既定の形を使うかと、カスタマイズの key の並びと、immutable subject を選ぶかの設定

| 本文フィールド | 型 | 指定条件 | 意味 |
|---|---|---|---|
| `use_default` | boolean | 必須 | 既定の形を使うか。`true` なら `include_claim_keys` は無視される |
| `include_claim_keys` | array of string | 任意 | `sub` を組み立てるクレームの key の並び。重複のない文字列の並びで、各 key は英数字と `_` だけからなる |
| `use_immutable_subject` | boolean | 任意 | このリポジトリで immutable subject の形を選ぶか |

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です（個人アカウントが所有する公開・非公開リポジトリで確認）。
>
> - key の順序：`["repo", "context"]` と `["context", "repo"]` は、それぞれ送った順のまま保存された。3つの key でも同じだった
> - `use_default: true` を送ったとき：`include_claim_keys` を一緒に送っても `201` で、送った key は保存されなかった。カスタマイズしていた key も消え、あとで `use_default: false` だけを送っても元の key は返らなかった
> - `include_claim_keys` を省略したとき：カスタマイズした状態で `use_default: false` だけ（または `use_immutable_subject` を足して）を送ると、`201` を返し、保存されていた key は消えた。省略は保持ではなく、空の並びと同じだった
> - 空の並び：`use_default: false` と、`include_claim_keys` の省略または `[]` は、どちらも `201` で受け付けられ、key のない状態で保存された。この状態で発行される `sub` の形は確かめていない
> - `use_immutable_subject` を省略したとき：保存されていた値が保持された（`true` と `false` のどちらでも、`use_default` が `true` でも `false` でも確かめた）
> - `use_immutable_subject` の切り替え：名前を変えたことのない公開リポジトリでは、`false` から `true` にして、`false` に戻すことができた。2026年7月15日より後に名前を変えた非公開リポジトリ（確認の前から `true`）では、`false` を送ると `use_default` が `true` でも `false` でも `201` を返したが、取得は `true` のままで、約 31 秒後まで変わらなかった。2章の名前の変更による強制と合うが、公開範囲の違いでないことは、名前を変えていない非公開リポジトリで確かめていない
> - `use_default` のない本文：`use_immutable_subject` だけを送ると `422`（`object is missing required key: use_default`）で、値は変わらなかった
> - 受け付けない値：定義にない key、大文字を含む `Repo`、`-` や空白を含む key、空文字列は `400`（`The template has one or more unsupported claim keys.`）、重複した key は `400`（`The template has one or more duplicate claim keys`）だった。文字列の `"false"`、配列でない `include_claim_keys`、文字列でない要素は `422` だった。直後に取得した拒否（定義にない key、大文字、重複、`claims_supported` の受け付けない key）では、値は変わらなかった
> - 定義にないフィールド：本文に加えても `201` で、無視された。取得専用の `sub_claim_prefix` を送っても無視された
> - 更新直後の取得：時刻を測った更新24回（公開19回、非公開5回。値を変えない更新を含む）のどれでも、`201` を受け取ってから約 0.6〜0.8 秒後の 3章の取得から更新後の値を返し、約 3.5 秒後まで更新前の値へ戻ることはなかった。`201` を受け取った直後に取得した更新6回（公開5回、非公開1回）でも、最初の取得から更新後の値を返した

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Set the customization template for an OIDC subject claim for a repository](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10#set-the-customization-template-for-an-oidc-subject-claim-for-a-repository) | External Document |
