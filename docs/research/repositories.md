# GitHub リポジトリ API 仕様

本ドキュメントは、GitHub REST API の「REST API endpoints for repositories」のうち、reco が利用するリポジトリの取得・更新エンドポイントの外部契約を整理します。対象は、リポジトリの名前と ID、必要な権限、HTTP 応答、リポジトリの一般設定のフィールドと、同じページにあるセキュリティ設定（`security_and_analysis`、Dependabot alerts、Dependabot security updates、Private vulnerability reporting）の取得・更新です。すべてのエンドポイントに共通する仕様は [GitHub REST API 共通仕様](rest-api.md) が扱います。記述の前提（根拠とする API バージョン、注釈の意味）は [Research](README.md) に従います。

## 1. 必要な権限と HTTP 応答

| メソッド・パス | 用途 | OAuth app・classic の scope | Fine-grained token permissions |
|---|---|---|---|
| `GET /repos/{owner}/{repo}` | リポジトリと一般設定の取得 | 記載なし | Metadata: read（マージ関連設定には追加の権限が必要。1.1 を参照） |
| `PATCH /repos/{owner}/{repo}` | 一般設定とセキュリティ設定（`security_and_analysis`）の更新 | 記載なし | Administration: write |
| `GET /repos/{owner}/{repo}/vulnerability-alerts` | Dependabot alerts の状態の取得 | 記載なし | Administration: read |
| `PUT` / `DELETE /repos/{owner}/{repo}/vulnerability-alerts` | Dependabot alerts の有効化・無効化 | 記載なし | Administration: write |
| `GET /repos/{owner}/{repo}/automated-security-fixes` | Dependabot security updates の状態の取得 | 記載なし | Administration: read |
| `PUT` / `DELETE /repos/{owner}/{repo}/automated-security-fixes` | Dependabot security updates の有効化・無効化 | 記載なし | Administration: write |
| `GET /repos/{owner}/{repo}/private-vulnerability-reporting` | Private vulnerability reporting の状態の取得 | 記載なし | Metadata: read |
| `PUT` / `DELETE /repos/{owner}/{repo}/private-vulnerability-reporting` | Private vulnerability reporting の有効化・無効化 | 記載なし | Administration: write |

| メソッド・パス | 正常応答 | ドキュメントに記載されたその他の応答 |
|---|---|---|
| `GET /repos/{owner}/{repo}` | `200` | `301` Moved permanently、`403` Forbidden、`404` Resource not found |
| `PATCH /repos/{owner}/{repo}` | `200` | `307` Temporary Redirect、`403` Forbidden、`404` Resource not found、`422` Validation failed, or the endpoint has been spammed |
| `GET .../vulnerability-alerts` | `204`（有効） | `404`（無効） |
| `PUT` / `DELETE .../vulnerability-alerts` | `204` | なし |
| `GET .../automated-security-fixes` | `200`（有効） | `404`（無効） |
| `PUT` / `DELETE .../automated-security-fixes` | `204` | なし |
| `GET .../private-vulnerability-reporting` | `200` | `422` Bad Request |
| `PUT` / `DELETE .../private-vulnerability-reporting` | `204` | `422` Bad Request |

セキュリティ設定のエンドポイントの説明は、Dependabot alerts と Dependabot security updates の取得・更新、Private vulnerability reporting の更新について、認証したユーザーがリポジトリの admin（取得では admin read）のアクセスを持つ必要があると記載しています。Private vulnerability reporting の取得には、この記載がありません。

`301` と `307` がどのような条件で返るかは、エンドポイントのドキュメントに記載がありません。どちらの応答本文も、スキーマは `message`、`documentation_url`、`url`、`status`（いずれも文字列）を持つ Basic Error です。名前変更後に観測した応答は 2章に示します。

> [!NOTE]
> 次は、reco の開発者が `repo` scope を持つトークンで実際の動作を確認した事項です。
>
> - 応答ヘッダー `X-Accepted-OAuth-Scopes` は、`GET /repos/{owner}/{repo}` では `repo`、`PATCH /repos/{owner}/{repo}` では空の値だった。`PATCH` に必要な scope は、このヘッダーからは特定できない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Metadata](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-metadata) | External Document |
| `grounding` | [Permissions required for fine-grained personal access tokens: Repository permissions for Administration](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens?apiVersion=2026-03-10#repository-permissions-for-administration) | External Document |
| `grounding` | [Check if vulnerability alerts are enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-vulnerability-alerts-are-enabled-for-a-repository) | External Document |
| `grounding` | [Check if Dependabot security updates are enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-dependabot-security-updates-are-enabled-for-a-repository) | External Document |
| `grounding` | [Check if private vulnerability reporting is enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-private-vulnerability-reporting-is-enabled-for-a-repository) | External Document |

### 1.1 マージ関連設定の取得条件

`GET /repos/{owner}/{repo}` のドキュメントは、「マージ関連設定を見るには `contents:read` と `contents:write` の権限が必要」と記載しています。この記載は fine-grained personal access token の permissions の表記で書かれており、1章の表が Metadata: read だけを挙げることとも一致しません。OAuth app のトークンと personal access token (classic) で、scope とトークン所有者のロールのどの組み合わせがこの条件にあたるかは記載がありません。また、どのフィールドがマージ関連設定にあたるかと、条件を満たさないときにそのフィールドがどう返るかも記載がありません。

> [!NOTE]
> 次は、reco の開発者が `gh auth token` のトークン（scope は `admin:public_key`、`gist`、`read:org`、`repo`）で実際の動作を確認した事項です。
>
> - トークン所有者が Admin ロールを持つ個人アカウント所有のリポジトリでは、`allow_auto_merge`、`delete_branch_on_merge`、`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge`、`allow_update_branch` が値とともに返る（後の4つは公開・非公開の両方で確認）
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
> - リダイレクト応答の本文：`301` と `307` のどちらも、`message` は `Moved Permanently`、`url` は `Location` と同じ URL、`documentation_url` はリダイレクトへの追従を説明するページだった
> - 名前を元に戻した場合：名前変更のあと元の名前へ戻すと、元の名前への `GET /repos/{owner}/{repo}` はリダイレクトされず `200` を返し、`id` も名前変更前と同じだった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Renaming a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/renaming-a-repository) | External Document |
| `grounding` | [OpenID Connect reference](https://docs.github.com/en/actions/reference/security/oidc) | External Document |

## 3. `GET /repos/{owner}/{repo}`

- 用途：リポジトリの情報と一般設定の取得
- 主な応答フィールド：`id`（整数、必須）、`name`（文字列、必須）、`full_name`（文字列、必須）、`owner`（必須）、`updated_at`（日時の文字列、必須）、`delete_branch_on_merge`（boolean、必須ではない）、`allow_auto_merge`（boolean、必須ではない）、`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge`、`allow_update_branch`（いずれも boolean、必須ではない）、`private`（boolean、必須）、`visibility`（文字列、必須ではない）、`permissions`（オブジェクト、必須ではない。中の `admin`、`push`、`pull` は boolean で必須）

`permissions` の各値が、認証したユーザーのリポジトリでのロールを表すのか、トークンに与えた権限を表すのかは、スキーマに記載がありません。

`updated_at` がどの変更で更新されるかは、スキーマに記載がありません。

リポジトリの公開範囲は、作成時に公開（public）か非公開（private）から選びます。内部（internal）の公開範囲を持てるのは、GitHub Enterprise Cloud を使い、Enterprise アカウントが所有する Organization のリポジトリだけです。個人アカウント所有のリポジトリは公開か非公開のどちらかで、`private` の真偽で区別できます。

> [!NOTE]
> 次は、reco の開発者が実際の動作で確認した事項です。
>
> - `updated_at` と一般設定の変更：`PATCH /repos/{owner}/{repo}` で `allow_auto_merge` や `delete_branch_on_merge` を変更しても `updated_at` は変わらず、名前を変更すると変わった
> - `private` と `visibility`：トークン所有者が Admin ロールを持つ個人アカウント所有の非公開リポジトリでは `private: true` と `visibility: "private"`、公開リポジトリでは `private: false` と `visibility: "public"` が返った。トークン所有者が Write 以上のロールを持たない第三者の公開リポジトリ（1.1節でマージ関連のフィールドが欠けたもの）でも、`private: false` は欠けずに返った
> - `permissions.admin`：トークン所有者が Admin ロールを持つ公開・非公開のリポジトリでは `true`、第三者の公開リポジトリでは `false` が返った
> - internal のリポジトリの応答は確認していない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [About repositories: About repository visibility](https://docs.github.com/en/repositories/creating-and-managing-repositories/about-repositories#about-repository-visibility) | External Document |

## 4. `PATCH /repos/{owner}/{repo}`

- 用途：リポジトリの一般設定の更新

| 本文フィールド | 型 | 指定条件 | ドキュメント上の既定値 |
|---|---|---|---|
| `delete_branch_on_merge` | boolean | 任意 | `false` |
| `allow_auto_merge` | boolean | 任意 | `false` |
| `allow_squash_merge` | boolean | 任意 | `true` |
| `allow_merge_commit` | boolean | 任意 | `true` |
| `allow_rebase_merge` | boolean | 任意 | `true` |
| `allow_update_branch` | boolean | 任意 | `false` |

`delete_branch_on_merge` は Pull Request のマージ時に head ブランチを自動で削除するか、`allow_auto_merge` は Pull Request の自動マージを許可するかの設定です。リポジトリ設定のドキュメントは、同じ二つの機能を、設定画面の「General」ページの「Pull Requests」の欄にある「Automatically delete head branches」と「Allow auto-merge」という項目で設定できると記載しています。たとえば、`allow_auto_merge` だけを指定するリクエスト本文は次のとおりです。

```json
{
  "allow_auto_merge": true
}
```

Pull Request の自動マージは、GitHub Free と GitHub Free for organizations では公開リポジトリだけで、GitHub Pro・GitHub Team・GitHub Enterprise Cloud・GitHub Enterprise Server では公開・非公開の両方のリポジトリで利用できると記載されています。この記載は GitHub Enterprise Server 版のドキュメントにあり、GitHub.com 版の同じページには見当たりません。利用できないリポジトリで `allow_auto_merge` を送ったときの応答は記載がありません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` は、Pull Request を squash merge、merge commit、rebase merge でマージすることを許可するかの設定です。リポジトリ設定のドキュメントは、同じ機能を「Pull Requests」の欄の「Allow squash merging」「Allow merge commits」「Allow rebase merging」という項目で設定できると記載しています。`allow_update_branch` は、マージの前に最新であることを求められていなくても、base ブランチより遅れた head ブランチを常に更新できるようにするかの設定です。ドキュメントは、同じ機能を「Pull Requests」の欄の「Always suggest updating pull request branches」という項目で設定できると記載しています。3つのマージ方式をすべて `false` にする要求の応答と、マージ方式の無効化がマージメッセージの設定（`squash_merge_commit_title` など）に及ぼす影響は、記載がありません。

本文フィールドはいずれも任意ですが、省略したフィールドの既存値が保持されるかは記載がありません。上の表の既定値が省略時に適用される値なのかも記載がありません。`200` の応答はリポジトリの表現を返しますが、それが更新後の状態を反映しているかは記載がありません。条件付きリクエストに対応する旨の記載もありません。

> [!NOTE]
> 次は、reco の開発者が `Content-Type: application/json` の本文で送り、実際の動作で確認した事項です。
>
> - 省略したフィールドの保持：`allow_auto_merge` だけを送ると、`allow_auto_merge` だけが変わり、`delete_branch_on_merge` を含む他のトップレベルのフィールドは変わらない（公開リポジトリで確認）。`delete_branch_on_merge` だけを送った場合も同様である（非公開リポジトリで確認）
> - `200` の応答本文：`id`、`allow_auto_merge`、`delete_branch_on_merge` を含み、これらの値は直後の `GET` と一致する。ただし本文全体は `GET` の応答と同じではなく、ネストしたオブジェクトの有無（`security_and_analysis`、`template_repository`）が異なる。`security_and_analysis` は、`GET` が返さない非公開リポジトリでも `PATCH` の応答本文には含まれた（5章）
> - 反映されない更新：GitHub Free のアカウントが所有する非公開リポジトリでは、`allow_auto_merge` だけを送ると `200` が返るが、値は変わらない。応答本文と直後の `GET` は更新前の値を示す。同じリポジトリで `delete_branch_on_merge` は変更できる。自動マージを利用できない条件（上記）に当たるリポジトリで起きたが、ほかの条件で起きないかは確認していない

> [!NOTE]
> 次は、reco の開発者が、トークン所有者が Admin ロールを持つ個人アカウント所有の公開・非公開リポジトリ（GitHub Free）で、実際の動作を確認した事項です。
>
> - 4項目の保持と反映：`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge`、`allow_update_branch` のどれか一つだけを `true` か `false` で送ると、`200` が返り、直後の `GET` は送った値を返す。ほかのマージ関連の9フィールド（1.1節）は変わらない（公開・非公開）
> - `200` の応答本文：マージ関連の10フィールドを含み、値は直後の `GET` と一致する（公開・非公開）
> - 3つのマージ方式をすべて無効にする要求：2方式を無効にしたあと、残る1方式に `false` を送ると `422` が返り、値は変わらない（公開・非公開）。3方式に `false` を一つの要求で送っても同じ `422` が返り、3つとも変わらない（公開）。本文は次のとおりで、`field` は送ったフィールドにかかわらず `merge_commit_allowed` だった
>
>     ```json
>     {"message":"Validation Failed","errors":[{"message":"Sorry, you need to allow at least one merge strategy. (no_merge_method)","resource":"Repository","field":"merge_commit_allowed","code":"invalid"}],"documentation_url":"https://docs.github.com/rest/repos/repos#update-a-repository","status":"422"}
>     ```
>
> - 有効な方式の入れ替え：`allow_merge_commit` だけが有効のとき、`allow_merge_commit: false` と `allow_squash_merge: true` を一つの要求で送ると、`200` が返り、両方が反映される（公開）
> - マージメッセージの設定への影響：squash merge と merge commit をそれぞれ無効にして有効に戻しても、`squash_merge_commit_title`、`squash_merge_commit_message`、`merge_commit_title`、`merge_commit_message` は変わらない。無効のあいだも `GET` は値を返す（公開）
> - `allow_auto_merge` と `delete_branch_on_merge`：上のどの要求でも変わらない（公開・非公開）
> - `use_squash_pr_title_as_default`：`GET` の応答にも `PATCH` の応答本文にも含まれない
> - Organization 所有のリポジトリと、Rulesets や Branch protection がマージ方式を求めるリポジトリでの応答は確認していない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Managing auto-merge for pull requests in your repository (GitHub Enterprise Server 3.18)](https://docs.github.com/en/enterprise-server@3.18/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-auto-merge-for-pull-requests-in-your-repository) | External Document |
| `grounding` | [Managing auto-merge for pull requests in your repository](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-auto-merge-for-pull-requests-in-your-repository) | External Document |
| `grounding` | [Managing the automatic deletion of branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-the-automatic-deletion-of-branches) | External Document |
| `grounding` | [Configuring commit squashing for pull requests](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/configuring-commit-squashing-for-pull-requests) | External Document |
| `grounding` | [Configuring commit merging for pull requests](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/configuring-commit-merging-for-pull-requests) | External Document |
| `grounding` | [Configuring commit rebasing for pull requests](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/configuring-commit-rebasing-for-pull-requests) | External Document |
| `grounding` | [Managing suggestions to update pull request branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-suggestions-to-update-pull-request-branches) | External Document |

## 5. `security_and_analysis`

`GET /repos/{owner}/{repo}` の応答の `security_and_analysis` は、オブジェクトか `null` です。中の項目は、`advanced_security`、`code_security`、`dependabot_security_updates`、`secret_scanning`、`secret_scanning_push_protection`、`secret_scanning_non_provider_patterns`、`secret_scanning_ai_detection`、`secret_scanning_delegated_alert_dismissal`、`secret_scanning_delegated_bypass` が `status`（`enabled` / `disabled`）を持つオブジェクトで、`secret_scanning_delegated_bypass_options` は `reviewers` の配列を持ちます。どの項目も必須ではありません。Dependency graph と Dependabot alerts の項目はありません。

`security_and_analysis` を見るには、リポジトリの admin の権限を持つか、リポジトリを所有する Organization のオーナーかセキュリティマネージャーである必要があると記載されています。条件を満たさないときに `null` を返すのか、フィールドを欠くのかは記載がありません。

`PATCH /repos/{owner}/{repo}` の本文の `security_and_analysis` は、有効・無効にするセキュリティ機能を指定するオブジェクトで、使うには同じ権限が必要と記載されています。中の各項目は `status`（`enabled` か `disabled`）を持ちます。たとえば、Secret scanning だけを有効にする本文は次のとおりです。

```json
{
  "security_and_analysis": {
    "secret_scanning": {
      "status": "enabled"
    }
  }
}
```

省略した中の項目の既存値が保持されるか、項目どうしの依存（ある項目を有効にするのに別の項目が必要か、ある項目を無効にすると別の項目も無効になるか）は記載がありません。`secret_scanning_delegated_bypass_options` についてだけ、`reviewers` を省略すると既存の値を変えないと記載されています。

Secret scanning の利用条件は、Secret scanning の概説に記載があります。公開リポジトリでは無料で自動的に動きます。Organization が所有する非公開・internal のリポジトリでは、GitHub Team か GitHub Enterprise Cloud で GitHub Secret Protection を有効にすると利用できます。ユーザーが所有するリポジトリでは、Enterprise Managed Users の GitHub Enterprise Cloud と、Secret Protection を有効にした GitHub Enterprise Server で利用できます。push protection の有効化の手順は、Secret Protection を有効にしてから push protection を有効にする順序で書かれています。

> [!NOTE]
> 次は、reco の開発者が `gh auth token` のトークン（scope は `admin:public_key`、`gist`、`read:org`、`repo`）で実際の動作を確認した事項です。対象は、トークン所有者が Admin ロールを持つ個人アカウント所有の公開リポジトリと非公開リポジトリ（GitHub Free のアカウント）、Write 以上のロールを持たない第三者の公開リポジトリです。
>
> - `GET` の応答：公開リポジトリでは、`security_and_analysis` が `secret_scanning`、`secret_scanning_push_protection`、`dependabot_security_updates`、`secret_scanning_non_provider_patterns`、`secret_scanning_validity_checks`（スキーマにない項目）を含んだ。非公開リポジトリでは、応答の `permissions.admin` が `true` でもフィールドそのものが欠けた。第三者の公開リポジトリ（`permissions.admin` が `false`）でも欠けた。どちらも `null` では返らなかった
> - 中の項目の保持：公開リポジトリで `secret_scanning` だけ、または `secret_scanning_push_protection` だけを送る `PATCH` は、送った項目だけを変え、もう一方の項目、`dependabot_security_updates`（`enabled` の状態で確認）、ほかの中の項目、トップレベルの一般設定を変えなかった。トップレベルの `delete_branch_on_merge` だけを送る `PATCH` も、`security_and_analysis` の中の項目を変えなかった
> - 項目どうしの依存：公開リポジトリで、`secret_scanning` が `disabled` のときに `secret_scanning_push_protection` を `enabled` にする `PATCH` は、`200` を返したが値を変えなかった（応答本文と直後の `GET` は `disabled`）。`secret_scanning_push_protection` が `enabled` のときに `secret_scanning` を `disabled` にする `PATCH` は、`secret_scanning_push_protection` も `disabled` にした。そのあと `secret_scanning` を `enabled` に戻しても、`secret_scanning_push_protection` は `disabled` のままだった
> - 非公開リポジトリでの更新：`secret_scanning` を `enabled` にする `PATCH` は `422` で、本文の `message` は `Secret scanning is not available for this repository.` だった。`secret_scanning_push_protection` を `enabled` にする `PATCH` は `200` を返したが値を変えなかった。`secret_scanning` を `disabled` にする `PATCH` は `200` を返した
> - `200` の応答本文：公開リポジトリと非公開リポジトリのどちらでも、ネストした項目を送る `PATCH` とトップレベルだけを送る `PATCH` の応答本文が `security_and_analysis` を含み、値は直後の `GET`（公開リポジトリ）と一致した。非公開リポジトリでは、`GET` が返さない `security_and_analysis` が応答本文に現れ、`secret_scanning` と `secret_scanning_push_protection` は `disabled`、`dependabot_security_updates` は 7章の取得と同じ値だった
> - `secret_scanning_non_provider_patterns` を `enabled` にする `PATCH` は、公開リポジトリで `200` を返したが値を変えなかった
> - 反映：公開リポジトリで、`secret_scanning_push_protection` を `disabled` にしてから約1秒おいて `secret_scanning` を `disabled` にする `PATCH`、`secret_scanning` を `enabled` にしてから約1秒おいて `secret_scanning_push_protection` を `enabled` にする `PATCH` は、どちらも応答本文と直後の `GET` が送った値を返した。`secret_scanning` だけを `disabled` にした直後の `GET` では、`secret_scanning_push_protection` もすでに `disabled` だった
> - トップレベルの更新の前後：公開リポジトリで `delete_branch_on_merge` だけを送る `PATCH` の直後と約2秒後の `GET` で、`secret_scanning` と `secret_scanning_push_protection` の値は更新前と変わらなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Get a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository) | External Document |
| `grounding` | [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository) | External Document |
| `grounding` | [Secret scanning](https://docs.github.com/en/code-security/concepts/secret-security/secret-scanning) | External Document |
| `grounding` | [Enabling push protection for your repository](https://docs.github.com/en/code-security/how-tos/secure-your-secrets/prevent-future-leaks/enable-push-protection) | External Document |

## 6. Dependabot alerts

`GET /repos/{owner}/{repo}/vulnerability-alerts` は、Dependabot alerts（dependency alerts）が有効なら本文のない `204` を、無効なら `404` を返すと記載されています。`PUT` は Dependabot alerts と Dependency graph を有効にし、`DELETE` は Dependabot alerts と Dependency graph を無効にすると記載されています。どちらも正常応答は本文のない `204` です。

```text
PUT    /repos/{owner}/{repo}/vulnerability-alerts  Enables dependency alerts and the dependency graph
DELETE /repos/{owner}/{repo}/vulnerability-alerts  Disables dependency alerts and the dependency graph
```

リポジトリの設定画面の解説は、Dependency graph と Dependabot alerts を別々の項目として示し、Dependabot alerts には先に Dependency graph を有効にする必要があると記載しています。Dependency graph、Dependabot alerts、push protection は、公開リポジトリでも既定では有効でないと記載されています。

Dependency graph の有効状態を返す操作と、Dependency graph だけを有効・無効にする操作は、REST API の記載にありません。Dependency graph を有効・無効にすると記載された操作は、Dependabot alerts の `PUT` と `DELETE` だけです。`/repos/{owner}/{repo}/dependency-graph/` の下の操作は、SBOM の取得、2つのコミットのあいだの依存関係の差分の取得（dependency review）、依存関係のスナップショットの送信（dependency submission）で、どれも Dependency graph の有効状態を返さず、変えもしません。SBOM を同期で返す `GET /repos/{owner}/{repo}/dependency-graph/sbom` は、2026年11月13日以降は使えなくなると記載され、生成を依頼する `.../sbom/generate-report` と、生成した SBOM を取得する `.../sbom/fetch-report/{sbom_uuid}` への移行が勧められています。`security_and_analysis` にも Dependency graph の項目はありません（5章）。Dependency graph の有効化の手順は、設定画面の操作だけを示し、REST API に触れていません。無効にしたときにほかの機能がどうなるかは記載がありません。

Dependabot security updates は、Dependency graph と Dependabot alerts を有効にしたリポジトリで利用できると記載されています（7章）。Dependabot alerts を無効にしたときに Dependabot security updates がどうなるかは記載がありません。

権限の不足と無効を区別できる応答があるかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が 5章と同じトークンと対象で、実際の動作を確認した事項です。Dependency graph の状態は、`GET /repos/{owner}/{repo}/dependency-graph/sbom` の応答（有効なら `200`、無効なら `404`）と、利用者が設定画面で見た表示で確かめた。
>
> - 取得：有効なら `204`。無効なら `404` で、本文の `message` は `Vulnerability alerts are disabled.` だった。第三者の公開リポジトリと存在しないリポジトリへの取得も `404` で、`message` は `Not Found` だった。ステータスだけでは、無効と権限の不足・存在しないことを区別できない
> - 更新の応答：`PUT` と `DELETE` は `204` で、すでに同じ状態のときも `204` だった。直後の取得は更新後の状態を示した
> - Dependency graph：Dependabot alerts が無効で Dependency graph が有効な公開リポジトリ（確認前の状態）で、`PUT` は Dependency graph を有効のままにした。`DELETE` は Dependency graph を無効にした（SBOM の取得が `404`、設定画面でも無効）。そのあとの `PUT` は Dependency graph を有効に戻した。Dependabot alerts を無効のまま Dependency graph を有効に戻すには、設定画面の操作が要った。非公開リポジトリでも、`DELETE` のあと SBOM の取得が `404`、`PUT` のあと `200` になった
> - Dependency graph を有効に戻した直後の SBOM：`PUT` の直後の SBOM の取得は `200` だったが、含まれるパッケージの数は `DELETE` の前より少なかった（公開リポジトリで 12 から 1、その十数秒後に 2）
> - SBOM の同期の取得：記載された期限を過ぎても `200` を返し、応答は `Deprecation` と `Sunset` のヘッダーを持った。非同期の操作の成否が Dependency graph の状態を示すかは確かめていない
> - 設定画面での Dependency graph の無効化：非公開リポジトリの設定画面で Dependency graph を無効にしようとすると、確認のダイアログが「Disabling the dependency graph will also disable Dependabot alerts and Dependabot security updates.」と示した。無効にしたあと、設定画面で Dependency graph だけを有効に戻せた。戻したあとの Dependabot alerts の取得は `404`（`Vulnerability alerts are disabled.`、`permissions.admin` は `true`）、Dependabot security updates は `enabled: false`、`paused: false`、SBOM の取得は `200` で、Dependabot の2設定は無効のままだった。無効にする前と、無効にしていたあいだの Dependabot の2設定の値は観測していない
> - Dependabot security updates：`DELETE` は、有効だった Dependabot security updates を無効にした（公開リポジトリでは `enabled: true` から、非公開リポジトリでは `enabled: true` と `paused: true` から、`enabled: false` と `paused: false` へ）。`security_and_analysis.dependabot_security_updates` も `disabled` になった。そのあとの `PUT` は Dependabot security updates を有効に戻さなかった。`PUT` は、無効だった Dependabot security updates を有効にしなかった
> - `PUT` と Dependabot security updates の一時停止：有効で一時停止中（`paused: true`）のリポジトリで Dependabot alerts の `PUT` を再び送っても、`paused` は `true` のままだった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Check if vulnerability alerts are enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-vulnerability-alerts-are-enabled-for-a-repository) | External Document |
| `grounding` | [Enable vulnerability alerts](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-vulnerability-alerts) | External Document |
| `grounding` | [Disable vulnerability alerts](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#disable-vulnerability-alerts) | External Document |
| `grounding` | [Managing security and analysis settings for your repository](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-security-and-analysis-settings-for-your-repository) | External Document |
| `grounding` | [Dependabot security updates](https://docs.github.com/en/code-security/concepts/supply-chain-security/dependabot-security-updates) | External Document |
| `grounding` | [Enabling the dependency graph](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/enable-dependency-graph) | External Document |
| `grounding` | [REST API endpoints for software bill of materials (SBOM)](https://docs.github.com/en/rest/dependency-graph/sboms?apiVersion=2026-03-10) | External Document |
| `grounding` | [REST API endpoints for dependency review](https://docs.github.com/en/rest/dependency-graph/dependency-review?apiVersion=2026-03-10) | External Document |
| `grounding` | [REST API endpoints for dependency submission](https://docs.github.com/en/rest/dependency-graph/dependency-submission?apiVersion=2026-03-10) | External Document |

## 7. Dependabot security updates

`GET /repos/{owner}/{repo}/automated-security-fixes` は、Dependabot security updates が有効か、無効か、一時停止中かを示すと記載されています。有効なら `200` で、本文は `enabled` と `paused`（どちらも boolean、必須）を持ちます。無効なら `404` と記載されています。`PUT` は有効にし、`DELETE` は無効にします。どちらも正常応答は本文のない `204` です。

Dependabot security updates は、Dependency graph と Dependabot alerts を有効にしたリポジトリで利用できると記載されています。メンテナーが Dependabot の Pull Request に反応しなくなると、Dependabot は更新を一時停止して知らせると記載されています。一時停止を API で解除できるかは記載がありません。`security_and_analysis.dependabot_security_updates`（5章）と、このエンドポイントの値の関係は記載がありません。

> [!NOTE]
> 次は、reco の開発者が 5章と同じトークンと対象で、実際の動作を確認した事項です。
>
> - 取得：有効なら `200` で `enabled: true`。無効のときも `404` ではなく `200` で、本文は `enabled: false`、`paused: false` だった。一時停止中の非公開リポジトリでは `enabled: true`、`paused: true` だった。`404`（`message` は `Not Found`）が返ったのは、第三者の公開リポジトリへの取得だった
> - 更新の応答：`PUT` と `DELETE` は `204` で、すでに同じ状態のときも `204` だった。一時停止中に `PUT` を送っても `paused` は `true` のままだった。いったん無効にして有効に戻すと、`paused` は `false` になった
> - Dependabot alerts が無効のとき：`PUT` も `DELETE` も `422` で、本文の `message` は `Vulnerability alerts must be enabled to configure automated security fixes.` だった。Dependabot alerts は無効のままだった
> - Dependabot alerts が有効のときの `DELETE` は、Dependabot alerts を変えなかった
> - Dependabot alerts を有効にした直後：Dependabot alerts の `PUT` の約1秒後に送った `PUT` は、公開と非公開のどちらのリポジトリでも `422` ではなく `204` で、直後の取得は `enabled: true` を示した。Dependabot alerts と Dependency graph がどちらも無効だった公開リポジトリでも同じだった
> - `security_and_analysis.dependabot_security_updates` との一致：公開リポジトリでは、すべての観測で、`enabled` が `true` なら `enabled`、`false` なら `disabled` だった。非公開リポジトリでは `GET` が `security_and_analysis` を返さず、`PATCH` の応答本文の値と一致した。`paused` に当たる項目は `security_and_analysis` にない

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Check if Dependabot security updates are enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-dependabot-security-updates-are-enabled-for-a-repository) | External Document |
| `grounding` | [Enable Dependabot security updates](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-dependabot-security-updates) | External Document |
| `grounding` | [Disable Dependabot security updates](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#disable-dependabot-security-updates) | External Document |
| `grounding` | [Dependabot security updates](https://docs.github.com/en/code-security/concepts/supply-chain-security/dependabot-security-updates) | External Document |

## 8. Private vulnerability reporting

`GET /repos/{owner}/{repo}/private-vulnerability-reporting` は、Private vulnerability reporting が有効かを `enabled`（boolean、必須）で返すと記載されています。`PUT` は有効にし、`DELETE` は無効にします。正常応答は、取得が `200`、更新が本文のない `204` です。どれも `422` Bad Request を返し得ると記載されていますが、条件は記載がありません。

設定の手順のページは、公開リポジトリのオーナーと管理者が Private vulnerability reporting を有効にできると記載しています。非公開リポジトリで利用できるかは記載がありません。

> [!NOTE]
> 次は、reco の開発者が 5章と同じトークンと対象で、実際の動作を確認した事項です。
>
> - 公開リポジトリ：取得は `200` で `enabled` を返した。`PUT` と `DELETE` は `204` で、すでに同じ状態のときも `204` だった。直後の取得は更新後の状態を示した
> - 非公開リポジトリ：取得、`PUT`、`DELETE` のどれも `422` ではなく `404` で、本文の `message` は `Not Found` だった。存在しないリポジトリへの取得も同じ `404` と本文で、区別できない
> - 第三者の公開リポジトリ（`permissions.admin` が `false`）：取得は `200` で `enabled` を返した
> - 公開リポジトリで `PUT` と `DELETE` を送った前後で、General、GitHub Actions の権限、Environments、Dependabot alerts と Dependabot security updates（どちらも無効）、Secret scanning と push protection（どちらも有効）の値は変わらなかった。Dependabot alerts と Dependabot security updates がどちらも有効のときも、この2つと Secret scanning、push protection、Dependency graph（SBOM の取得が `200`）の値は変わらなかった

| Relationship | Target | Target Type |
|---|---|---|
| `grounding` | [Check if private vulnerability reporting is enabled for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#check-if-private-vulnerability-reporting-is-enabled-for-a-repository) | External Document |
| `grounding` | [Enable private vulnerability reporting for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-private-vulnerability-reporting-for-a-repository) | External Document |
| `grounding` | [Disable private vulnerability reporting for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#disable-private-vulnerability-reporting-for-a-repository) | External Document |
| `grounding` | [Configuring private vulnerability reporting for a repository](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository) | External Document |
