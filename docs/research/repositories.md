# GitHub リポジトリ API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for repositories」のうち、reco が利用するリポジトリの取得・更新エンドポイントの外部契約を整理します。対象は、リポジトリの名前と ID、必要な権限、HTTP 応答、リポジトリの一般設定のフィールドです。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

| メソッド・パス | 用途 | OAuth app・classic の scope | Fine-grained token permissions |
|---|---|---|---|
| `GET /repos/{owner}/{repo}` | リポジトリと一般設定の取得 | 記載なし | Metadata: read（マージ関連設定には追加の権限が必要。1.1 を参照） |
| `PATCH /repos/{owner}/{repo}` | 一般設定の更新 | 記載なし | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}` | `200` | `301` Moved permanently、`403` Forbidden、`404` Resource not found |
| `PATCH /repos/{owner}/{repo}` | `200` | `307` Temporary Redirect、`403` Forbidden、`404` Resource not found、`422` Validation failed, or the endpoint has been spammed |

`301` と `307` がどのような条件で返るかは、エンドポイントのドキュメントに記載がありません。名前変更後に観測した応答は 2章に示します。

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Metadata](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-metadata) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-administration) | External Document |

### 1.1 マージ関連設定の取得条件

`GET /repos/{owner}/{repo}` のドキュメントは、「マージ関連設定を見るには `contents:read` と `contents:write` の権限が必要」と記載しています。この記載は fine-grained personal access token の permissions の表記で書かれており、1章の表が Metadata: read だけを挙げることとも一致しません。OAuth app のトークンと personal access token (classic) で、scope とトークン所有者のロールのどの組み合わせがこの条件にあたるかは記載がありません。また、どのフィールドがマージ関連設定にあたるかと、条件を満たさないときにそのフィールドがどう返るかも記載がありません。

> [!NOTE]
> 次は、reco の開発者が `gh auth token` のトークン（scope は `admin:public_key`、`gist`、`read:org`、`repo`）で実際の動作を確認した事項です。
>
> - トークン所有者が Admin ロールを持つ個人アカウント所有のリポジトリでは、`allow_auto_merge` と `delete_branch_on_merge` が値とともに返る
> - トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリ（応答の `permissions` は `pull` だけが `true`）では、`allow_auto_merge`、`delete_branch_on_merge`、`allow_merge_commit`、`allow_rebase_merge`、`allow_squash_merge`、`allow_update_branch`、`merge_commit_message`、`merge_commit_title`、`squash_merge_commit_message`、`squash_merge_commit_title` の各フィールドが応答から欠け、`false` などの値では返らない。応答は `200` で、`id` は返る
> - Write・Maintain ロールでの応答は確認していない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |

## 2. リポジトリの名前と ID

パスの `{owner}` と `{repo}` は名前による指定であり、名前はリポジトリの名前変更や移転で変わります。GitHub は名前変更後も、旧名に対する Web 上の情報と Git の操作を新しい名前へリダイレクトします。ただし、旧名で新しいリポジトリを作るとこのリダイレクトは機能しなくなり、API へのリクエストがリダイレクトされるかは名前変更のドキュメントに記載がありません。`GET /repos/{owner}/{repo}` の応答の `id` について、スキーマには型（int64）以外の説明がありません。

リポジトリ ID の性質は、GitHub Actions の OpenID Connect (OIDC) のドキュメントに記載があります。OIDC トークンの `repository_id` クレームは、リポジトリの名前変更などのあいだで変わらない、システムが生成する識別子とされています。また、名前だけで作る subject クレームは、名前が再利用されると別の所有者が同じ値を作れるため、所有者 ID とリポジトリ ID を含む subject クレームを導入したと記載されています。REST API の `id` と OIDC の `repository_id` が同じ値であることは、記載がありません。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - ID の安定性：リポジトリの名前変更の前後で `GET /repos/{owner}/{repo}` の `id` は変わらず、`GET /repositories/{id}` は同じリポジトリを返す。移転の前後と、旧名で新しいリポジトリを作った場合の `id` は確認していない
> - 旧名へのリダイレクト：名前変更後の旧名で要求すると、`GET /repos/{owner}/{repo}` と `GET /repos/{owner}/{repo}/actions/permissions` に `301`、`PATCH /repos/{owner}/{repo}` に `307` が返る。いずれも `Location` は `https://api.github.com/repositories/{id}` にエンドポイントの残りのパスを続けた URL で、名前ではなくリポジトリ ID を指す。`307` が返った `PATCH` の本文の値は、リポジトリに反映されていなかった。移転や名前の再利用のあとの応答は確認していない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Renaming a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository) | External Document |
| `grounding` | [OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc) | External Document |

## 3. `GET /repos/{owner}/{repo}`

- 用途：リポジトリの情報と一般設定の取得
- 主な応答フィールド：`id`（整数、必須）、`name`（文字列、必須）、`full_name`（文字列、必須）、`owner`、`delete_branch_on_merge`（boolean、必須ではない）、`allow_auto_merge`（boolean、必須ではない）

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |

## 4. `PATCH /repos/{owner}/{repo}`

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

Pull Request の自動マージは、GitHub Free と GitHub Free for organizations では公開リポジトリだけで、GitHub Pro・GitHub Team・GitHub Enterprise Cloud・GitHub Enterprise Server では公開・非公開の両方のリポジトリで利用できると記載されています。この記載は GitHub Enterprise Server 版のドキュメントにあり、GitHub.com 版の同じページには見当たりません。利用できないリポジトリで `allow_auto_merge` を送ったときの応答は記載がありません。

本文フィールドはいずれも任意ですが、省略したフィールドの既存値が保持されるかは記載がありません。上の表の既定値 `false` が省略時に適用される値なのかも記載がありません。`200` の応答はリポジトリの表現を返しますが、それが更新後の状態を反映しているかは記載がありません。条件付きリクエストに対応する旨の記載もありません。

> [!NOTE]
> 次は、reco の開発者が `Content-Type: application/json` の本文で送り、実際の動作で確認した事項です。
>
> - 省略したフィールドの保持：`allow_auto_merge` だけを送ると、`allow_auto_merge` だけが変わり、`delete_branch_on_merge` を含む他のトップレベルのフィールドは変わらない（公開リポジトリで確認）。`delete_branch_on_merge` だけを送った場合も同様である（非公開リポジトリで確認）
> - `200` の応答本文：`id`、`allow_auto_merge`、`delete_branch_on_merge` を含み、これらの値は直後の `GET` と一致する。ただし本文全体は `GET` の応答と同じではなく、ネストしたオブジェクトの有無（`security_and_analysis`、`template_repository`）が異なる
> - 反映されない更新：GitHub Free のアカウントが所有する非公開リポジトリでは、`allow_auto_merge` だけを送ると `200` が返るが、値は変わらない。応答本文と直後の `GET` は更新前の値を示す。同じリポジトリで `delete_branch_on_merge` は変更できる。自動マージを利用できない条件（上記）に当たるリポジトリで起きたが、ほかの条件で起きないかは確認していない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Managing auto-merge for pull requests in your repository (GitHub Enterprise Server 3.18)](https://docs.github.com/en/enterprise-server@3.18/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-auto-merge-for-pull-requests-in-your-repository) | External Document |
