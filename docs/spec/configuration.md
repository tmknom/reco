# YAML 設定仕様

## 1. 目的と基本構造

本ドキュメントは、reco が受け付ける YAML 設定宣言の構造、型、許容値、組み合わせ、管理範囲の解釈、入力検証を定めます。利用者はリポジトリごとに管理する設定と期待状態を明示します。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
      allow_auto_merge: true
      squash_merge_commit:
        title: PR_TITLE
        message: PR_BODY
    actions:
      permissions:
        allowed_actions: selected
        selected_actions:
          github_owned_allowed: true
          verified_allowed: true
          patterns_allowed: []
      workflow:
        default_workflow_permissions: read
        can_approve_pull_request_reviews: false
      access:
        access_level: none
      fork_pull_requests:
        approval_policy: first_time_contributors
      retention:
        days: 30
      oidc_subject:
        use_default: false
        include_claim_keys:
          - repo
          - context
      variables:
        - name: GO_VERSION
          value: "1.27"
    environments:
      - name: production
        deployment_branch_policy:
          mode: protected_branches
        required_reviewers:
          - type: User
            id: 1234567
        prevent_self_review: true
        wait_timer: 30
        variables:
          - name: REGION
            value: ap-northeast-1
      - name: staging
    security:
      dependabot_alerts: true
      dependabot_security_updates: true
      secret_scanning: true
      secret_scanning_push_protection: true
      private_vulnerability_reporting: true
    rulesets:
      - name: protect-default-branch
        target: branch
        enforcement: active
        conditions:
          ref_name:
            include: ["~DEFAULT_BRANCH"]
            exclude: []
        bypass_actors:
          - actor_type: RepositoryRole
            actor_id: 5
            bypass_mode: always
        rules:
          - type: pull_request
            parameters:
              required_approving_review_count: 0
              dismiss_stale_reviews_on_push: false
              require_code_owner_review: false
              require_last_push_approval: false
              required_review_thread_resolution: false
          - type: deletion

  - name: example/project-b
    general:
      delete_branch_on_merge: true
```

| 要素 | 型 | 条件 |
|---|---|---|
| ルート | マッピング | `repositories` だけを持つ |
| `repositories` | シーケンス | 1件以上のリポジトリごとの宣言を含む |
| `repositories[]` | マッピング | `name` と、少なくとも一つの管理設定を含む `general`、`actions`、`environments`、`security`、`rulesets` のいずれかを持つ |
| `repositories[].name` | 文字列 | 必須。空でない `OWNER/REPO` 形式 |
| `general`、`squash_merge_commit`、`merge_commit`、`actions`、`permissions`、`selected_actions`、`workflow`、`access`、`fork_pull_requests`、`private_repositories`、`retention`、`oidc_subject`、`security` | マッピング | 使用する場合は、その階層で定義された設定または子のマッピングを一つ以上含む。`actions` は `permissions`、`workflow`、`access`、`fork_pull_requests`、`retention`、`oidc_subject`、`variables` のうち一つ以上を含めばよい |
| `variables`（`actions` 配下） | シーケンス | 使用する場合は、1件以上の変数の宣言を含む（10章） |
| `environments` | シーケンス | 使用する場合は、1件以上の Environment の宣言を含む（11章） |
| `required_reviewers`（`environments[]` 配下） | シーケンス | 0件以上6件以下の担当者の宣言を含む（11.3節） |
| `variables`（`environments[]` 配下） | シーケンス | 使用する場合は、1件以上の変数の宣言を含む（11.4節） |
| `rulesets` | シーケンス | 使用する場合は、1件以上の Ruleset の宣言を含む（13章） |

定義していないフィールドは、どの階層でも受け付けません。どの位置でも、`null` を指定して省略を表現することはできません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |

## 2. 管理範囲と期待状態

**設定の省略は管理対象外を意味します。** 省略した値を `false`、空配列、GitHub の既定値、設定の無効化などへ暗黙に補完しません。管理範囲はリポジトリごとに、期待状態が明示された管理設定から導出します。ただし、設定間の組み合わせが必須となる場合は、省略したままでは入力として成立しません。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: false
```

この例で管理するのは `general.allow_auto_merge` だけです。`false` も明示された期待値です。宣言していない `general.delete_branch_on_merge` は管理範囲外です。宣言した設定の現在値が期待値と一致していても管理範囲には含まれますが、更新は不要です。

設定の一部は、意味を確定するために複数の値を一体として宣言します。この場合、個々の値を独立した管理の単位にはせず、一つの設定グループとして扱います。どの値を組み合わせるかは、設定ごとの章で定めます。設定グループの項目のうち、省略できると章が定めた項目を省略した場合、その項目は管理範囲外です。

入力検証は、宣言だけから決まる誤りを拒否します。入力として有効な宣言でも、リポジトリの状態（公開範囲や、ほかの設定の現在値など）によっては適用できないことがあります。これは観測のあとに適用可能性として判断し、どの宣言がどの状態で適用不能になるかは [設定ごとの設計](../design/settings/README.md) の各文書の「適用可能性と副作用」が定めます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

### 2.1 関連リソースの宣言

Environment、Variable、Ruleset のように、一つのリポジトリに名前の違うものが複数あり得る関連リソースは、名前ごとに宣言します。規則は次のとおりです。

- **書いた名前だけを管理する**：宣言に書かなかった名前の関連リソースは管理範囲外であり、reco は作成も変更も削除もしない
- **書いた関連リソースが存在しなければ作成する**：宣言は、その名前の関連リソースが存在することを期待状態に含む。含まない宣言の形がある場合は、その章で定める
- **名前を消しても削除しない**：設定宣言から名前を消すことは省略であり、削除の宣言ではない。削除するのは、その名前について存在しないことを明示的に宣言した場合に限る。存在しないことを宣言できる関連リソースと書き方は、その章で定める
- **同じ名前を二度書けない**：同じリポジトリの中で、同一とみなされる名前の宣言が複数あれば、入力不正として拒否する。名前が同じかどうかの判定は、関連リソースの種類ごとにその章で定める

要素の一覧を値とする設定（`patterns_allowed`、`deployment_branch_policy.patterns` など）は、関連リソースと読み方が違います。宣言した一覧が期待値の全体であり、GitHub 上の一覧にあって宣言にない要素は、適用するとなくなります。たとえば GitHub 上の一覧が `main` と `feature/*` で、宣言が `main` と `release/*` なら、適用後の一覧は `main` と `release/*` です。どちらの読み方をするかは、設定ごとにその章で定めます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 3. General

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `general.delete_branch_on_merge` | 真偽値 | `true` / `false` | Pull Request のマージ後に head branch を自動削除する設定 | 独立 |
| `general.allow_auto_merge` | 真偽値 | `true` / `false` | Pull Request の Auto-merge を許可する設定 | 独立 |
| `general.allow_squash_merge` | 真偽値 | `true` / `false` | Pull Request の squash merge を許可する設定 | 独立 |
| `general.allow_merge_commit` | 真偽値 | `true` / `false` | Pull Request を merge commit でマージすることを許可する設定 | 独立 |
| `general.allow_rebase_merge` | 真偽値 | `true` / `false` | Pull Request の rebase merge を許可する設定 | 独立 |
| `general.allow_update_branch` | 真偽値 | `true` / `false` | base branch より遅れた head branch の更新を、Pull Request で常に提案する設定 | 独立 |
| `general.squash_merge_commit.title` | 文字列 | `PR_TITLE` / `COMMIT_OR_PR_TITLE`。必須 | squash merge のコミットメッセージの既定のタイトル | squash merge のマージメッセージ（設定グループ） |
| `general.squash_merge_commit.message` | 文字列 | `PR_BODY` / `COMMIT_MESSAGES` / `BLANK`。必須 | squash merge のコミットメッセージの既定の本文 | squash merge のマージメッセージ（設定グループ） |
| `general.merge_commit.title` | 文字列 | `PR_TITLE` / `MERGE_MESSAGE`。必須 | merge commit のコミットメッセージの既定のタイトル | merge commit のマージメッセージ（設定グループ） |
| `general.merge_commit.message` | 文字列 | `PR_BODY` / `PR_TITLE` / `BLANK`。必須 | merge commit のコミットメッセージの既定の本文 | merge commit のマージメッセージ（設定グループ） |
| `general.has_issues` | 真偽値 | `true` / `false` | リポジトリの Issues を有効にする設定 | 独立 |
| `general.has_projects` | 真偽値 | `true` / `false` | リポジトリの Projects を有効にする設定 | 独立 |
| `general.has_wiki` | 真偽値 | `true` / `false` | リポジトリの Wiki を有効にする設定 | 独立 |
| `general.web_commit_signoff_required` | 真偽値 | `true` / `false` | GitHub の Web 画面からのコミットに、作成者のサインオフを求める設定 | 独立 |

これらはリポジトリ単位の設定であり、変更しても個々の Pull Request のマージ、ブランチの更新、ブランチの直接削除は行いません。`allow_update_branch` は更新を提案するボタンを表示する設定で、reco も GitHub もブランチを自動では更新しません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` の3つを **マージ方式** と呼びます。3つは独立した管理の単位で、一部だけを宣言できます。`allow_update_branch` はマージ方式に数えず、次の組み合わせの規則に関わりません。

| 宣言したマージ方式 | 判定 |
|---|---|
| 3つとも `false` | エラー |
| 1つ以上が `true` | 有効 |
| 一部を省略し、宣言したものがすべて `false` | 有効 |

3つとも `false` と宣言できないのは、GitHub が、3つのマージ方式をすべて無効にする更新を拒否し（[GitHub リポジトリ API 仕様](../research/repositories.md) 4章）、この状態を実現できないためです。この誤りは、`general` のほかのどの誤りよりも後に示します。一部を省略した宣言では、宣言していないマージ方式の値は宣言に含まれず、reco はそれを有効にしません。

`squash_merge_commit` と `merge_commit` は、squash merge と merge commit のそれぞれについて、コミットメッセージの既定のタイトルと本文を表します。この組を **マージメッセージ** と呼びます。どちらも `title` と `message` を一体で宣言する **一つの設定グループ** で、省略できる項目はありません。二つの設定グループは互いに独立で、片方だけを宣言できます。各値の意味は [GitHub リポジトリ API 仕様](../research/repositories.md) 4章のとおりです。rebase merge にはマージメッセージの設定がありません。

GitHub が更新で受け付けるタイトルと本文の組み合わせは、次の7つだけです（同 4章）。reco は、これ以外の組み合わせを入力不正として拒否します。

| 設定グループ | `title` | `message` |
|---|---|---|
| `squash_merge_commit` | `PR_TITLE` | `PR_BODY`、`BLANK`、`COMMIT_MESSAGES` |
| `squash_merge_commit` | `COMMIT_OR_PR_TITLE` | `COMMIT_MESSAGES` |
| `merge_commit` | `PR_TITLE` | `PR_BODY`、`BLANK` |
| `merge_commit` | `MERGE_MESSAGE` | `PR_TITLE` |

片方だけの宣言を受け付けないのは、GitHub が片方だけを送る更新を、保存されている値と同じ値でも拒否することがあるためです（同 4章）。受け付けない組み合わせを拒否するのは、承認のあとで GitHub が拒否する宣言を、入力の時点で止めるためです。設定グループの中の誤りは、キーの集合（欠けたキーと未知のキー）、`title` の値、`message` の値、組み合わせの順に、最初の一つを示します。`general` の中の誤りは、上の表の並びで最初の一つを示します。

マージメッセージは、対応するマージ方式（`squash_merge_commit` には `allow_squash_merge`、`merge_commit` には `allow_merge_commit`）の宣言によらず宣言でき、`allow_squash_merge: false` と `squash_merge_commit` も同時に宣言できます。

`has_issues`、`has_projects`、`has_wiki`、`web_commit_signoff_required` は、一部だけを宣言でき、マージ方式の組み合わせの規則にもマージメッセージにも関わりません。`web_commit_signoff_required` は、コミットの署名を求める設定とは別の設定です。`has_issues`、`has_projects`、`has_wiki` の `false` は機能を無効にする宣言で、reco は Issue、Project、Wiki のページを作成も削除もしません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 4. Actions permissions

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.permissions.enabled` | 真偽値 | `true` / `false` | GitHub Actions を有効にするか | Actions の有効状態（独立） |
| `actions.permissions.allowed_actions` | 文字列 | `all` / `local_only` / `selected` | Actions と再利用可能なワークフローの実行許可範囲 | Actions 許可ポリシー（独立） |
| `actions.permissions.selected_actions.github_owned_allowed` | 真偽値 | `true` / `false` | GitHub 所有 Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.verified_allowed` | 真偽値 | `true` / `false` | Marketplace の認証済み作成者が提供する Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.patterns_allowed` | シーケンス | 文字列の要素を0件以上（`[]` 可） | 許可する Actions と再利用可能なワークフローのパターン | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.sha_pinning_required` | 真偽値 | `true` / `false` | Actions を完全な長さのコミット SHA に固定することを必須にするか | 独立 |

`selected_actions` の3項目は選択方式の Actions 許可条件を表し、 **一つの設定グループ** として扱います。

`enabled: false` は Actions を無効にする宣言です。reco が Actions を無効にするのは、`enabled: false` を宣言したリポジトリに限ります。

`sha_pinning_required` は独立した管理の単位で、`allowed_actions` を宣言しなくても宣言できます。`sha_pinning_required` だけの宣言は、Actions が有効であることを期待状態に含みません。

`allowed_actions` を宣言した場合は、そのポリシーで Actions が実行できること、つまり Actions が有効であることも期待状態に含めます。Actions が無効のリポジトリでは、reco は宣言したポリシーで Actions を有効にします。そのため、`allowed_actions` とともに宣言した `enabled: true` の意味は、`enabled` を省略した場合と同じです。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

### 4.1 入力の組み合わせ

| `enabled` | `allowed_actions` | `selected_actions` | `sha_pinning_required` | 判定 |
|---|---|---|---|---|
| 省略または `true` | `all` または `local_only` | 省略 | 省略または指定 | 有効 |
| 省略または `true` | `selected` | 3項目をすべて指定 | 省略または指定 | 有効 |
| 省略または `true` | `selected` | 省略、または3項目の一部だけを指定 | 省略または指定 | エラー |
| 省略または `true` | `all` または `local_only` | 指定 | 省略または指定 | エラー |
| 省略または `true` | 省略 | 指定 | 省略または指定 | エラー |
| `true` | 省略 | 省略 | 省略または指定 | 有効 |
| `false` | 省略 | 省略 | 省略 | 有効 |
| 省略 | 省略 | 省略 | 指定 | 有効 |
| `false` | 指定 | 省略または指定 | 省略または指定 | エラー |
| `false` | 省略 | 指定 | 省略または指定 | エラー |
| `false` | 省略 | 省略 | 指定 | エラー |
| 省略 | 省略 | 省略 | 省略（`permissions` 自体を宣言） | エラー（空のマッピング） |

`enabled: false` と `allowed_actions`・`selected_actions`・`sha_pinning_required` を同時に宣言できないのは、Actions が無効のあいだ GitHub がこれらの保存された値を返さず（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 3.1・4章）、期待状態を確かめられないためです。この組み合わせの誤りは、`allowed_actions` の値や `selected_actions` の中身の誤りより先に示します。`actions.workflow` の設定は、`enabled` の値によらず同時に宣言できます。

3項目を必須とする点は **reco 固有の入力契約** です。GitHub REST API では Selected actions の3項目は任意パラメータですが、reco は選択方式の Actions 許可条件の期待状態を一意に定めるため、3項目を必須とします。

### 4.2 `patterns_allowed`

`patterns_allowed` は文字列の要素からなるシーケンスで、`[]` は空集合を明示します。空文字列、空白だけの文字列、前後に空白を含む文字列、`null`、文字列以外の要素、および同一文字列の重複は受け付けません。空白は、Unicode の空白文字（改行やノーブレークスペースを含む）を指します。重複は YAML のエスケープ表現を解釈した後の文字列で判定し、大文字小文字だけが異なる文字列は別の要素として扱います。値を自動補完・トリミング・正規化しません。

```yaml
patterns_allowed:
  - "actions/checkout@v7"
  - "actions/setup-go@v6"
```

`patterns_allowed` の値は **文字列の集合** です。要素の順序に意味はなく、順序だけが違う宣言は同じ期待値です。ワイルドカードや異なるパターンの意味的な包含関係は解釈せず、文字列の一致で扱います。

## 5. Workflow permissions

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.workflow.default_workflow_permissions` | 文字列 | `read` / `write` | ワークフローの `GITHUB_TOKEN` の既定権限 | 独立 |
| `actions.workflow.can_approve_pull_request_reviews` | 真偽値 | `true` / `false` | GitHub Actions による Pull Request の作成と承認の許可 | 独立 |

`default_workflow_permissions: read` は個々のワークフローで明示された権限まで含め、すべての書き込み権限を禁止するものではありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

## 6. Actions access

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.access.access_level` | 文字列 | `none` / `user` | リポジトリの外のワークフローが、このリポジトリの Actions と再利用可能なワークフローを使える範囲 | 独立 |

`none` はリポジトリの外のワークフローからのアクセスを許可しません。`user` は、同じユーザーが所有するほかの非公開リポジトリのワークフローからのアクセスを許可します。

GitHub の値には `organization` もありますが、reco は宣言として受け付けません。`organization` は Organization が所有するリポジトリのための値で、reco がサポートするユーザー所有のリポジトリでは、GitHub がこの値への更新を拒否するためです（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 6章）。

`access_level` は非公開リポジトリにだけ適用される設定です。`actions.permissions` の宣言によらず宣言でき、`actions.permissions.enabled: false` と同時にも宣言できます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

## 7. Fork pull requests

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.fork_pull_requests.approval_policy` | 文字列 | `first_time_contributors_new_to_github` / `first_time_contributors` / `all_external_contributors` | Fork からの Pull Request のワークフローの実行に承認が要るユーザーの範囲 | 独立 |
| `actions.fork_pull_requests.private_repositories.run_workflows_from_fork_pull_requests` | 真偽値 | `true` / `false` | Fork からの Pull Request で起動したワークフローを実行できるか | 非公開リポジトリの Fork Pull Request ワークフローの実行条件（設定グループ） |
| `actions.fork_pull_requests.private_repositories.send_write_tokens_to_workflows` | 真偽値 | `true` / `false` | そのワークフローに、書き込み権限を持つ `GITHUB_TOKEN` を渡すか | 非公開リポジトリの Fork Pull Request ワークフローの実行条件（設定グループ） |
| `actions.fork_pull_requests.private_repositories.send_secrets_and_variables` | 真偽値 | `true` / `false` | そのワークフローで、Secrets と Variables を使えるようにするか | 非公開リポジトリの Fork Pull Request ワークフローの実行条件（設定グループ） |
| `actions.fork_pull_requests.private_repositories.require_approval_for_fork_pr_workflows` | 真偽値 | `true` / `false` | そのワークフローの実行に、書き込み権限を持つユーザーの承認を求めるか | 非公開リポジトリの Fork Pull Request ワークフローの実行条件（設定グループ） |

初めてのコントリビューターは、このリポジトリにコミットか Pull Request がマージされたことのないユーザーです。`first_time_contributors_new_to_github` では、初めてのコントリビューターのうち、GitHub に登録して間もないユーザーに承認が要ります。`first_time_contributors` では、初めてのコントリビューターすべてに承認が要ります。`all_external_contributors` では、リポジトリのメンバーか所有者でも Organization のメンバーでもない、すべてのユーザーに承認が要ります（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 7章）。

値は大文字小文字を区別して比べ、`FIRST_TIME_CONTRIBUTORS` のような大文字の表記を受け付けません。GitHub が、大文字の値も定義にない値も拒否するためです（同 7.2）。

`approval_policy` は公開リポジトリにだけ適用される設定です。`actions.permissions` の宣言によらず宣言でき、`actions.permissions.enabled: false` と同時にも宣言できます。

`private_repositories` の4項目は、非公開リポジトリの Fork Pull Request ワークフローの実行条件を表し、 **一つの設定グループ** として扱います（同 8章）。非公開リポジトリにだけ適用される設定で、`approval_policy` と同じく、`actions.permissions` の宣言によらず、`actions.permissions.enabled: false` と同時にも宣言できます。

`fork_pull_requests` は `approval_policy` と `private_repositories` のどちらか一つ以上を持ちます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

### 7.1 `private_repositories` の入力の組み合わせ

| `run_workflows_from_fork_pull_requests` | ほかの3項目 | 判定 |
|---|---|---|
| `true` | すべて指定（値は任意） | 有効 |
| `false` | すべて `false` で指定 | 有効 |
| `false` | すべて指定し、一つ以上が `true` | エラー |
| 4項目のどれかを省略 | 4項目のどれかを省略 | エラー（足りないキーをすべて示す） |

4項目を必須とする点は **reco 固有の入力契約** です。GitHub REST API では `run_workflows_from_fork_pull_requests` 以外の3項目は任意パラメータですが、reco は実行条件の期待状態を一意に定めるため、4項目を必須とします。

`run_workflows_from_fork_pull_requests: false` とほかの3項目の `true` を同時に宣言できないのは、GitHub がこの組み合わせの更新を受け付けたうえで、3項目を `false` で保存するためです（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 8.2）。この誤りは、4項目の値の誤り（真偽値でない表記など）より後に示します。組み合わせは4項目の真偽値から決まり、値が真偽値でなければ判定できないためです。

## 8. Retention

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.retention.days` | 整数 | `1` 以上 | チェック、ワークフローの実行、コミットのステータス、Artifact、Log を保持する日数 | 独立 |

`0` を受け付けないのは、GitHub が `1` 未満の値を拒否するためです（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 9.2）。

設定できる日数の上限は、入力検証では確かめません。上限はリポジトリごとに GitHub が決め（同 9章）、観測してはじめて分かるためです。

`actions.permissions` の宣言によらず宣言でき、`actions.permissions.enabled: false` と同時にも宣言できます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

## 9. OIDC subject

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.oidc_subject.use_default` | 真偽値 | `true` / `false`。必須 | OIDC トークンの subject クレームに既定の形を使うか | subject のカスタマイズ（設定グループ） |
| `actions.oidc_subject.include_claim_keys` | シーケンス | 9.1節の claim key を1件以上。`use_default` が `false` のときだけ必須で、`true` では指定できない | subject クレームを組み立てるクレームの key の並び | subject のカスタマイズ（設定グループ） |
| `actions.oidc_subject.use_immutable_subject` | 真偽値 | `true` / `false`。省略できる | subject クレームのリポジトリの部分に、所有者とリポジトリの ID を含める形を使うか | subject のカスタマイズ（設定グループ） |

3項目は subject のカスタマイズを表し、 **一つの設定グループ** として扱います。`use_immutable_subject` は省略できる項目で、省略した場合は管理範囲外です。省略しても、GitHub に保存された値を reco は変えません（[GitHub Actions OIDC API 仕様](../research/actions-oidc.md) 4章）。`use_immutable_subject` は、`use_default` の値によらず書けます。

| `use_default` | `include_claim_keys` | 判定 |
|---|---|---|
| `true` | 省略 | 有効。既定の形を使う |
| `false` | 1件以上を指定 | 有効 |
| `false` | 省略、または `[]` | エラー |
| `true` | 指定（`[]` を含む） | エラー |

`use_default: true` と `include_claim_keys` を同時に宣言できないのは、GitHub がこの組み合わせの更新を受け付けたうえで、key を保存しないためです（同 4章）。`use_default: false` で key のない宣言を受け付けないのは、GitHub はこの状態を保存するものの、発行される subject クレームの形が資料にも確認にもなく、宣言の意味を予想できないためです（同 4章）。この組み合わせの誤りは、`use_default` の値の誤りより後に、`include_claim_keys` の要素の誤りより先に示します。組み合わせは `use_default` の真偽値と `include_claim_keys` を書いたかどうかで決まり、要素の値によらないためです。

`actions.permissions` の宣言によらず宣言でき、`actions.permissions.enabled: false` と同時にも宣言できます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions OIDC API 仕様](../research/actions-oidc.md) | Repository File |

### 9.1 claim key

`include_claim_keys` の要素は、ASCII の英数字と `_` だけからなる、空でない文字列です。同じ文字列の要素を二つ以上書くことはできません。重複は YAML のエスケープ表現を解釈した後の文字列で判定し、大文字小文字だけが異なる文字列は別の要素として扱います。`null` と文字列以外の要素は受け付けません。

```yaml
oidc_subject:
  use_default: false
  include_claim_keys:
    - repo
    - context
```

`include_claim_keys` の値は **並び** です。要素の順序に意味があり、宣言の順のまま期待値になります。順序だけが違う宣言は、異なる期待値です。GitHub は key を送った順のまま保存し、資料の例はどれも subject クレームを key の並びの順に組み立てているためです（同 2章・4章）。

reco は、GitHub が受け付ける key の一覧を持たず、形式だけを確かめます。受け付ける key は、リポジトリを所有するのが個人アカウントか Organization か Enterprise かで変わり得るためです（同 2章）。形式に合っても GitHub が受け付けない key（大文字を含む `Repo` や、定義にない key）は、入力検証を通り、`apply` の更新操作の失敗として分かります。GitHub はこの更新を拒否し、値を変えません（同 4章）。

## 10. Repository Variables

`actions.variables` は、リポジトリの Repository Variable を名前ごとに宣言するシーケンスです。Repository Variable は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.variables` | シーケンス | 変数の宣言を1件以上 | 管理する Repository Variable の一覧 | なし（要素ごとに定まる） |
| `actions.variables[].name` | 文字列 | 10.1節の名前 | 管理する Repository Variable の名前 | Repository Variable（関連リソースそのもの） |
| `actions.variables[].value` | 文字列 | 10.2節の値。`state` を書かない要素では必須 | その名前の Repository Variable が存在し、その値であること | Repository Variable（関連リソースそのもの） |
| `actions.variables[].state` | 文字列 | `absent` | その名前の Repository Variable が存在しないこと | Repository Variable（関連リソースそのもの） |

- **`name` は必須で、`value` か `state` のどちらか一方を書く**：要素が持てるキーはこの3つだけである。組み合わせは 10.3節が定める
- **`value` を書いた要素は、存在と値の宣言を兼ねる**：書いた名前の Repository Variable が存在しなければ作成を計画し、存在して値が違えば値の変更を計画する
- **`state: absent` を書いた要素は、存在しないことの宣言である**：その名前の Repository Variable が存在すれば削除を計画し、存在しなければ変更不要である
- **行を消しても削除しない**：`actions.variables` から要素を消すことは省略であり（2.1節）、その変数は管理範囲外になる
- **`actions` に `variables` だけを書いた宣言は、Actions の許可設定を宣言しない**：`permissions`、`workflow`、`access` の設定は管理範囲外のままである

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 変数 API 仕様](../research/actions-variables.md) | Repository File |

### 10.1 変数の名前

`name` は、ASCII の英大文字、数字、`_` だけからなる、空でない文字列です。数字で始まらず、大文字小文字を問わず `GITHUB_` で始まりません。GitHub が受け付ける名前の規則（[GitHub Actions 変数 API 仕様](../research/actions-variables.md) 2章）に、英小文字を含まないという制限を加えたものです。

英小文字を受け付けないのは、GitHub が名前を大文字にして保存するためです（同 2章）。小文字を含む宣言は GitHub 上の綴りと必ず食い違い、`show` と `plan` に二つの綴りが現れます。小文字を含む名前は、大文字にした綴りを示して拒否します。reco が大文字に読み替えて受け付けないのは、利用者が書いた名前を黙って変えないためです。大文字小文字を問わず `GITHUB_` で始まる名前は、小文字の誤りより先に `GITHUB_` の誤りとして示します。大文字にした綴りも受け付けない名前だからです。

名前の長さは入力検証で確かめません。公式資料が上限を記載しておらず、GitHub の上限が変わり得るためです。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。GitHub が名前を大文字小文字を区別せずに扱うためです（同 2章）。同じリポジトリの `actions.variables` に同じ名前が複数あれば、入力不正として拒否します。

### 10.2 変数の値

`value` は、空でない文字列です。値は YAML のエスケープ表現を解釈した後の文字列として扱い、トリミング、改行の正規化、Unicode の正規化をしません。GitHub 上の値とは文字列の完全一致で比べ、前後の空白や末尾の改行だけが違う値も異なる値とします。GitHub は空文字列のほかの値を送ったとおりに保存するためです（[GitHub Actions 変数 API 仕様](../research/actions-variables.md) 3章）。

- **引用符のない数値、真偽値、日付は拒否する**：`value: 1.27`、`value: 1`、`value: true`、`value: 2026-11-22` などは、YAML が文字列でない値と解釈するため、文字列へ暗黙に変換しない（14.1節）。拒否の理由に、引用符で囲んだ表記（`"1.27"`）を直し方として添える
- **空文字列は拒否する**：`value: ""`、`value: ''`、中身のないブロックスカラーを受け付けない。GitHub が空文字列の値の作成も更新も拒否するため（同 3章）
- **`null` と値なしは、空文字列とは別の誤りとして拒否する**：`value: ~`、`value: null`、値を書かない `value:` は、14.1節の `null` と値なしの誤りである
- **ブロックスカラーの末尾の改行は値に含まれる**：`value: |` で書いた値は末尾に改行を持つ。GitHub 上の値が末尾に改行を持たなければ、異なる値になる

値の長さは入力検証で確かめません。名前の長さと同じく、GitHub の上限が変わり得るためです。

### 10.3 存在しないことの宣言

`state` に書ける値は `absent` だけです。`value` と `state` の組み合わせは次のとおりです。

| `value` | `state` | 判定 |
|---|---|---|
| 10.2節の値 | 省略 | 有効。存在し、値を伴うことの宣言 |
| 省略 | `absent` | 有効。存在しないことの宣言 |
| 省略 | 省略 | エラー |
| 指定（中身を問わない） | `absent` | エラー |
| 省略または指定 | `absent` 以外 | エラー |

`state` は大文字小文字を区別し、`Absent`、`present`、空文字列、文字列でない値、`null` と値なしを受け付けません。`state` の誤りは、`value` との組み合わせの誤りより先に示します。組み合わせの誤りは、`value` の中身の誤りより先に示します。

`value: ~`、`value: ""`、`value` の省略は、どれも存在しないことの宣言になりません（10.2節）。値の書き忘れが削除にならないようにするためです。`value` も `state` もない要素は、`value` がないことを示して拒否し、`state: absent` を案内しません。値を書き忘れた利用者を削除へ誘導しないためです。

```yaml
actions:
  variables:
    - name: LEGACY_FLAG
      state: absent
```

同じ名前の要素を、値の宣言と存在しないことの宣言に分けて書くことはできません。10.1節の名前の重複として拒否します。

## 11. Environments

`environments` は、リポジトリの Environment を名前ごとに宣言するシーケンスです。Environment は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `environments[].name` | 文字列 | 11.1節の名前 | その名前の Environment が存在すること | Environment（関連リソースそのもの） |
| `environments[].deployment_branch_policy.mode` | 文字列 | `all` / `protected_branches` / `selected` | Environment へデプロイできるブランチとタグの制限の方式 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns` | シーケンス | 個別パターンの要素を0件以上（`[]` 可）。`mode` が `selected` のときだけ必須で、ほかの方式では指定できない（11.2節） | デプロイを許可するブランチとタグのパターンの一覧の全体 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].type` | 文字列 | `branch` / `tag` | パターンがブランチとタグのどちらに一致するか | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].name` | 文字列 | 11.2節の名前 | ブランチ名またはタグ名のパターン | デプロイ元制限（設定グループ） |
| `environments[].required_reviewers` | シーケンス | 担当者の要素を0件以上6件以下（`[]` 可。11.3節） | その Environment を使うデプロイジョブの実行を承認できる担当者の一覧の全体 | 担当者の集合 |
| `environments[].required_reviewers[].type` | 文字列 | `User` | 担当者の種類 | 担当者の集合 |
| `environments[].required_reviewers[].id` | 整数 | 正の整数 | 担当者とする利用者の GitHub 上の ID | 担当者の集合 |
| `environments[].prevent_self_review` | 真偽値 | `true` / `false`（11.5節） | デプロイジョブを起動した利用者が、自分のジョブを承認できないこと | 自己承認の禁止 |
| `environments[].wait_timer` | 整数 | `0` 以上 `43199` 以下（11.5節） | ジョブが起動されてから実行されるまでデプロイを待たせる分数 | デプロイの待機時間 |
| `environments[].variables` | シーケンス | 変数の宣言を1件以上（11.4節） | その Environment で管理する Environment Variable の一覧 | なし（要素ごとに定まる） |
| `environments[].variables[].name` | 文字列 | 10.1節の名前 | その Environment で管理する Environment Variable の名前 | Environment Variable（関連リソースそのもの） |
| `environments[].variables[].value` | 文字列 | 10.2節の値。`state` を書かない要素では必須 | その Environment に、その名前の Environment Variable が存在し、その値であること | Environment Variable（関連リソースそのもの） |
| `environments[].variables[].state` | 文字列 | `absent` | その Environment に、その名前の Environment Variable が存在しないこと | Environment Variable（関連リソースそのもの） |

- **`name` は必須である**：`name` のない要素は、`deployment_branch_policy` や `variables` を書いていても入力不正として拒否する。どの Environment の宣言かを補わない
- **要素は存在の宣言を兼ねる**：`environments[]` に書いた名前の Environment が存在しなければ、作成を計画する。`variables` だけを書いた Environment も同じである。`deployment_branch_policy`、`required_reviewers`、`prevent_self_review`、`wait_timer`、`variables` を省略した Environment は、存在だけを管理する
- **存在しないことの宣言は受け付けない**：reco は Environment を削除しない
- **`deployment_branch_policy` は `mode` を必須とする**：`all` はすべてのブランチとタグからのデプロイを許可する。`protected_branches` は、保護ルールのあるブランチからのデプロイだけを許可する。`protected_branches` の宣言は、Branch protection を管理することを意味しない。`selected` は、`patterns` に書いたパターンに一致するブランチとタグからのデプロイだけを許可する

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 11.1 Environment の名前

`name` は、空でなく、255 文字以下で、印字可能な ASCII 文字（U+0020 から U+007E）だけからなる文字列です。前後に空白を持たず、`.` と `..` ではありません。`/` は位置を問わず含められ、空白は前後でなければ含められます。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。同じリポジトリの宣言に同じとみなされる名前が複数あれば、入力不正として拒否します。たとえば `Production` と `production` を同じリポジトリに並べることはできません。

ASCII 以外の文字と制御文字を受け付けないのは、GitHub が大文字小文字のほかに、アクセントの有無、全角と半角、末尾の空白、制御文字の違いも同じ名前とみなすためです（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 2章）。reco はこの判定を再現できません。受け付けると、GitHub 上で綴りの違う同じ Environment を存在しないと判断し、作成の要求で既存の Environment を変え得ます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 11.2 デプロイ元制限の入力の組み合わせと `patterns`

`deployment_branch_policy` は、方式と個別パターンの一覧をまとめた一つの設定グループです。`mode` と `patterns` の組み合わせは次のとおりです。

| `mode` | `patterns` | 判定 |
|---|---|---|
| `all` または `protected_branches` | 省略 | 有効 |
| `selected` | 0件以上の要素を指定（`[]` を含む） | 有効 |
| `selected` | 省略 | エラー |
| `all` または `protected_branches` | 指定（`[]` を含む） | エラー |
| 省略 | 省略または指定 | エラー |

`mode` に定義していない値を書いた場合は、`patterns` の有無より先に `mode` の誤りとして示します。組み合わせの誤りは、`patterns` の中身の誤りより先に示します。

`patterns` の要素は、`type` と `name` の両方を持つマッピングです。`type` は省略できず、省略を `branch` と読みません。`name` は文字列で、空文字列、空白だけの文字列、前後に空白を含む文字列を受け付けません。空白は 4.2節と同じく Unicode の空白文字を指します。また、位置を問わず、U+0000 から U+0020 までの文字（C0 制御文字と半角空白。DEL は含まない）を含む文字列を受け付けません。GitHub は、この範囲から確かめた文字（半角空白、タブ、改行など）を含む名前のパターンの作成をどれも拒み、範囲の外で確かめた文字（DEL と ASCII 以外の空白を含む）は受け付けたためです（[GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) 4.1）。受け付けると、承認のあとの更新で初めて失敗します。それ以外の文字（途中の ASCII 以外の空白、`/`、`*`、ASCII 以外の文字）は受け付け、値を自動補完・トリミング・正規化しません。前後の空白と U+0000 から U+0020 までの文字を拒むほかに、パターンの構文が GitHub に受け付けられるかは、入力検証では確かめません。

```yaml
environments:
  - name: production
    deployment_branch_policy:
      mode: selected
      patterns:
        - type: branch
          name: main
        - type: tag
          name: "v*"
```

同じ一覧に、`type` と `name` の組が同じ要素を二つ書けません。重複は YAML のエスケープ表現を解釈した後の文字列で判定し、大文字小文字だけが異なる名前（`main` と `MAIN`）と、名前が同じで `type` が違う要素は、別の要素として扱います。GitHub がこれらを別のパターンとして扱うためです（[GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) 2章）。

`patterns` の値は `type` と `name` の組の **集合** です。要素の順序に意味はなく、順序だけが違う宣言は同じ期待値です。ワイルドカードの意味的な包含関係は解釈せず、文字列の一致によって扱います。`patterns` は一覧の全体であり、宣言した一覧が適用後の一覧になります（2.1節）。`patterns: []` は、どのブランチとタグからもデプロイできない状態の宣言です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

### 11.3 担当者

`required_reviewers` は、その Environment を使うデプロイジョブの実行を承認できる担当者の一覧です。

```yaml
environments:
  - name: production
    required_reviewers:
      - type: User
        id: 1234567
```

要素は、`type` と `id` の両方を持つマッピングです。`type` は `User` だけを受け付け、省略できません。省略を `User` と読まないのは、省略を GitHub が `User` と補う動作に依存しないためです。`Team` は受け付けません。個人アカウント所有のリポジトリでは GitHub が `type: Team` を受け付けず、Organization が所有するリポジトリでの動作は確かめていないためです（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 4.1）。`id` は利用者の GitHub 上の ID で、正の整数です。login では宣言できません。reco が login から ID を解決すると、解決した相手が意図した相手かを確かめる手段がないためです。ID は `show` の表示から書き写せます（[CLI 仕様](cli.md) 6章）。

`required_reviewers` の値は `type` と `id` の組の **集合** です。要素の順序に意味はなく、順序だけが違う宣言は同じ期待値です。一覧の全体であり、宣言した一覧が適用後の一覧になります（2.1節）。

- **`required_reviewers: []` は、担当者がいないことの宣言である**：適用すると、担当者の承認を待たずにジョブが進むようになる。省略は管理対象外である
- **7件以上は入力不正である**：GitHub が受け付けるのは6件までである（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 4.1）
- **`type` と `id` の組が同じ要素を二つ書けない**
- **誤りを示す順**：件数の誤りを要素の誤りより先に示す。要素は先頭から順に、キーの過不足（`type` と `id` がそろい、ほかのキーがないこと）、`type`、`id`、先の要素との重複の順に確かめ、最初に見つかった誤りを示す
- **入力検証は、担当者にできるかを確かめない**：`id` の利用者が存在するか、リポジトリの権限を持つかは確かめない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 11.4 Environment Variables

`environments[].variables` は、その Environment に属する Environment Variable を名前ごとに宣言するシーケンスです。Environment Variable は関連リソースであり、2.1節の規則に従います。2.1節の2つ目の規則（書いた関連リソースが存在しなければ作成する）は、変数にも、属する先の Environment にも当てはまります。変数を書くことは、属する先の Environment が存在することの宣言を兼ねるためです。

```yaml
environments:
  - name: production
    variables:
      - name: REGION
        value: ap-northeast-1
```

- **要素の形は `actions.variables` と同じである**：`name` は必須で、`value` か `state: absent` のどちらか一方を書く。名前は 10.1節、値は 10.2節、存在しないことの宣言は 10.3節の規則に従う
- **`value` を書いた要素は存在と値の宣言を兼ねる**：書いた名前の Environment Variable が存在しなければ作成を計画し、存在して値が違えば値の変更を計画する
- **`state: absent` を書いた要素は、存在しないことの宣言である**：その名前の Environment Variable が存在すれば削除を計画する。`state: absent` の変数だけを書いた Environment も、Environment の存在の宣言を兼ね、存在しなければ作成を計画する
- **重複は一つの Environment の中で判定する**：同じ Environment の `variables` に同じ名前（10.1節の判定）が複数あれば、入力不正として拒否する。別の Environment の同じ名前と、`actions.variables` の同じ名前は、重複ではない
- **同じ名前の Repository Variable とは別の管理設定である**：`actions.variables` と `environments[].variables` に同じ名前を書いても、値を同期しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 変数 API 仕様](../research/actions-variables.md) | Repository File |

### 11.5 自己承認の禁止とデプロイの待機時間

`prevent_self_review` は、デプロイジョブを起動した利用者が、担当者であっても自分のジョブを承認できなくする設定です。`wait_timer` は、ジョブが起動されてから実行されるまで待たせる分数です。どちらも、省略すれば管理対象外です。

```yaml
environments:
  - name: production
    required_reviewers:
      - type: User
        id: 1234567
    prevent_self_review: true
    wait_timer: 30
```

- **`prevent_self_review` は真偽値である**：表記は 14.1節の真偽値の規則に従う
- **`required_reviewers: []` と `prevent_self_review: true` は同時に宣言できない**：GitHub は、担当者がいない Environment に `prevent_self_review` を保存しないためである（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 4.1）。`required_reviewers: []` と `prevent_self_review: false` は宣言できる
- **`required_reviewers` を省略した `prevent_self_review` も受け付ける**：GitHub 上の担当者で適用できるかが決まるため、入力検証では確かめない。適用できない場合の扱いは [Environments の設計](../design/settings/environments.md) の適用可能性と副作用に従う
- **`wait_timer` は `0` から `43199` までの整数である**：`0` は待機時間がないことの宣言である。表記は 14.1節の整数の規則に従い、小数（`2.0` を含む）と負の値は受け付けない。`43200` 以上を受け付けないのは、GitHub の公式資料は 43,200 までとするが、`43200` を送ると GitHub が成功を応答して値を変えなかったためである（同 4.1）
- **誤りを示す順**：`prevent_self_review` と `wait_timer` の値の誤りを、組み合わせの誤りより先に示す。`required_reviewers` の誤りがあれば、組み合わせより先にそれを示す

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

## 12. Security

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `security.dependabot_alerts` | 真偽値 | `true` / `false` | 依存関係の脆弱性を Dependabot alerts で知らせるか | 独立 |
| `security.dependabot_security_updates` | 真偽値 | `true` / `false` | 脆弱性を直す Pull Request を Dependabot security updates が作るか | 独立 |
| `security.secret_scanning` | 真偽値 | `true` / `false` | リポジトリに push されたシークレットを Secret scanning で検出するか | 独立 |
| `security.secret_scanning_push_protection` | 真偽値 | `true` / `false` | シークレットを含む push を、リポジトリの push protection で拒否するか。利用者が自分のアカウントに設定する push protection とは別の設定 | 独立 |
| `security.private_vulnerability_reporting` | 真偽値 | `true` / `false` | 見つけた脆弱性を、公開せずにメンテナーへ報告できる Private vulnerability reporting を有効にするか | 独立 |

5つの設定は独立した管理の単位で、どれか一つだけを宣言できます。5つとも省略した `security` は、空のマッピングとして拒否します（14.1節）。Dependabot の2設定のどちらの期待値も、もう一方の設定の状態を含みません。Secret scanning の2設定についても同じです。

5つの設定のいずれかの値が真偽値でない誤りは、次に挙げる組み合わせの誤りより先に示します。Dependabot の2設定の組み合わせの誤りと、Secret scanning の2設定の組み合わせの誤りが両方あるときは、Dependabot の誤りを示します。

### 12.1 Dependabot

GitHub が Dependabot security updates を一時停止していても、設定としては有効のままです（[GitHub リポジトリ API 仕様](../research/repositories.md) 7章）。そのため、一時停止中の Dependabot security updates は `dependabot_security_updates: true` と一致します。一時停止は宣言できず、reco は一時停止を解除しません。

2つの設定の組み合わせは次のとおりです。

| `dependabot_alerts` | `dependabot_security_updates` | 判定 |
|---|---|---|
| `true` | 省略、`true`、`false` | 有効 |
| `false` | 省略、`false` | 有効 |
| `false` | `true` | エラー |
| 省略 | `true`、`false` | 有効 |

`dependabot_alerts: false` と `dependabot_security_updates: true` を同時に宣言できないのは、GitHub が、Dependabot alerts が無効のリポジトリで Dependabot security updates の有効化を拒否し（GitHub リポジトリ API 仕様 7章）、この状態を実現できないためです。値の誤りを先に示すのは、組み合わせが2つの値で決まるためです。Dependency graph は宣言を受け付けない設定です。

### 12.2 Secret scanning

2つの設定の組み合わせは次のとおりです。

| `secret_scanning` | `secret_scanning_push_protection` | 判定 |
|---|---|---|
| `true` | 省略、`true`、`false` | 有効 |
| `false` | 省略、`false` | 有効 |
| `false` | `true` | エラー |
| 省略 | `true`、`false` | 有効 |

`secret_scanning: false` と `secret_scanning_push_protection: true` を同時に宣言できないのは、GitHub が、Secret scanning を無効にするとリポジトリの push protection も無効にし、Secret scanning が無効のあいだはリポジトリの push protection を有効にする更新を反映しないため（GitHub リポジトリ API 仕様 5章）、この状態を実現できないためです。

2つの設定は、reco が公開リポジトリでだけ管理する設定です。

### 12.3 Private vulnerability reporting

`private_vulnerability_reporting` は、ほかの設定との組み合わせの規則を持ちません。reco が公開リポジトリでだけ管理する設定です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 13. Rulesets

`rulesets` は、リポジトリの Ruleset を名前ごとに宣言するシーケンスです。Ruleset は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `rulesets` | シーケンス | Ruleset の宣言を1件以上 | 管理する Ruleset の一覧 | なし（要素ごとに定まる） |
| `rulesets[].name` | 文字列 | 13.1節の名前 | 管理する Ruleset の名前 | Ruleset（関連リソースそのもの） |
| `rulesets[].target` | 文字列 | `branch` | Ruleset が対象にする ref の種類 | Ruleset（関連リソースそのもの） |
| `rulesets[].enforcement` | 文字列 | `active` / `disabled` | ルールを強制するか | Ruleset（関連リソースそのもの） |
| `rulesets[].conditions.ref_name.include` | シーケンス | 13.3節のパターンを0件以上（`[]` 可） | 対象にするブランチのパターン | Ruleset（関連リソースそのもの） |
| `rulesets[].conditions.ref_name.exclude` | シーケンス | 13.3節のパターンを0件以上（`[]` 可） | 対象から外すブランチのパターン | Ruleset（関連リソースそのもの） |
| `rulesets[].bypass_actors` | シーケンス | 13.4節の要素を0件以上（`[]` 可） | ルールを Bypass できる主体の全体 | Ruleset（関連リソースそのもの） |
| `rulesets[].rules` | シーケンス | 13.5節の要素を0件以上（`[]` 可） | Ruleset が強制するルールの全体 | Ruleset（関連リソースそのもの） |

- **`name`、`target`、`enforcement`、`conditions`、`bypass_actors`、`rules` はどれも必須である**：省略を空のシーケンスや GitHub の既定値に補わない。要素が持てるキーはこの6つだけで、`id` と `state` も受け付けない
- **要素は、存在と値の宣言を兼ねる**：書いた名前の Ruleset が存在しなければ作成を計画し、存在して値が違えば値の変更を計画する
- **宣言は、その Ruleset の全体である**：`include`、`exclude`、`bypass_actors`、`rules` は一覧の全体として読む（2.1節の、要素の一覧を値とする設定の読み方）。GitHub 上の Ruleset にあって宣言にないルール、Bypass、パターンは、適用するとなくなる
- **存在しないことの宣言は受け付けない**：reco は Ruleset を削除しない
- **宣言に書かなかった名前の Ruleset は管理範囲外である**：reco は作成も変更も削除もせず、要求も送らない

`include`、`exclude`、`bypass_actors`、`rules` の値は、それぞれ要素の **集合** です。要素の順序に意味はなく、順序だけが違う宣言は同じ期待値です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Rules API 仕様](../research/rules.md) | Repository File |

### 13.1 Ruleset の名前

`name` は、空でなく、印字可能な ASCII 文字（U+0020 から U+007E）だけからなり、前後に空白を持たない文字列です。空白は前後でなければ含められ、`/` や `.` も含められます。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。同じリポジトリの `rulesets` に同じとみなされる名前が複数あれば、入力不正として拒否します。名前は `target` をまたいで一意です。

ASCII 以外の文字と制御文字を受け付けないのは、GitHub が大文字小文字のほかにアクセントの有無も同じ名前とみなし、ほかの文字の規則が分かっていないためです（[GitHub Rules API 仕様](../research/rules.md) 2章）。前後の空白を受け付けないのは、GitHub が黙って除いて保存し、宣言と保存される名前が食い違うためです（同 2章）。名前の長さは入力検証で確かめません。公式資料が上限を記載していないためです。

### 13.2 `target` と `enforcement`

`target` に書ける値は `branch` だけです。タグと Push の Ruleset は扱いません。

`enforcement` に書ける値は `active` と `disabled` です。GitHub の値には `evaluate` もありますが、reco は宣言として受け付けません。reco がサポートする個人アカウント所有のリポジトリでは、GitHub が `evaluate` を拒否するためです（[GitHub Rules API 仕様](../research/rules.md) 4.2）。

### 13.3 対象の条件

`conditions` は `ref_name` だけを持ち、`ref_name` は `include` と `exclude` の両方を持ちます。どちらも省略できず、`[]` は空集合を明示します。

パターンは、`~DEFAULT_BRANCH`（デフォルトブランチ）、`~ALL`（すべてのブランチ）、`refs/heads/` で始まる文字列のどれかです。大文字小文字を区別し、`~default_branch` や `refs/heads/` のない `main` を受け付けません。GitHub がこれらを拒否し、正規化もしないためです（[GitHub Rules API 仕様](../research/rules.md) 5章）。

空文字列、空白だけの文字列、前後に空白を含む文字列、`null`、文字列以外の要素、同じ一覧の中の同じ文字列の重複は、4.2節の `patterns_allowed` と同じく受け付けません。これらの誤りは、パターンの形の誤りより先に示し、重複はパターンの形の誤りより後に示します。`include` と `exclude` に同じ文字列を書くことはできます。値を自動補完・トリミング・正規化しません。

```yaml
conditions:
  ref_name:
    include: ["~DEFAULT_BRANCH", "refs/heads/release/*"]
    exclude: []
```

### 13.4 Bypass

`bypass_actors` の要素は、`actor_type`、`actor_id`、`bypass_mode` を持つマッピングです。受け付ける組は次のとおりです。

| `actor_type` | `actor_id` | `bypass_mode` |
|---|---|---|
| `RepositoryRole` | `2`、`4`、`5` のどれか | `always` / `pull_request` / `exempt` |
| `User` | 正の整数 | `always` / `pull_request` / `exempt` |
| `Integration` | 正の整数 | `always` / `pull_request` / `exempt` |
| `DeployKey` | 書かない | `always` |

- **`bypass_mode` は必須である**：GitHub の既定値（`always`）に補わない
- **`RepositoryRole` の ID は数値のまま書く**：ID とロールの対応は公開されておらず、reco は対応を持たない。`show` の表示から書き写せる。`2`・`4`・`5` に限るのは、GitHub がほかの ID を拒否するか、成否の分からない応答を返したためである（[GitHub Rules API 仕様](../research/rules.md) 6章）
- **`Team` と `OrganizationAdmin` は受け付けない**：個人アカウント所有のリポジトリでは GitHub が拒否する（同 6章）
- **同じ主体を二度書けない**：`actor_type` と `actor_id` が同じ要素を二つ書くと、`bypass_mode` が違っても拒否する。GitHub は先の要素だけを残すため（同 6章）。`DeployKey` は二つ書けない

誤りは、`actor_type`、`actor_id` の有無、`actor_id` の値、`bypass_mode`、同じ主体の重複の順に示します。

### 13.5 ルール

`rules` の要素は、`type` と、型が持つなら `parameters` を持つマッピングです。受け付ける `type` は `pull_request` と `deletion` だけです。同じ `type` の要素を二つ書けません。誤りは、`type`、`parameters` の有無、`parameters` の中身、同じ `type` の重複の順に示します。

`pull_request` は、マージの前に Pull Request を経ることを求めるルールです。`parameters` は必須で、次の5項目をすべて書きます。ほかのキーは受け付けません。GitHub は未知のキーを黙って捨てるためです（[GitHub Rules API 仕様](../research/rules.md) 7.1）。

| `parameters` のキー | 型 | 許容値 |
|---|---|---|
| `required_approving_review_count` | 整数 | `0` から `10` |
| `dismiss_stale_reviews_on_push` | 真偽値 | `true` / `false` |
| `require_code_owner_review` | 真偽値 | `true` / `false` |
| `require_last_push_approval` | 真偽値 | `true` / `false` |
| `required_review_thread_resolution` | 真偽値 | `true` / `false` |

`deletion` は、Bypass できる主体にだけ対象のブランチの削除を許すルールです。`parameters` を書けません。GitHub は送った `parameters` を黙って捨てるためです（同 7.2）。

```yaml
rules:
  - type: pull_request
    parameters:
      required_approving_review_count: 1
      dismiss_stale_reviews_on_push: true
      require_code_owner_review: false
      require_last_push_approval: false
      required_review_thread_resolution: true
  - type: deletion
```

`rules: []` は、ルールを一つも持たない Ruleset の宣言です。

## 14. 入力検証と対象名

入力検証では、YAML の構造・値の形式と、宣言されたリポジトリ名・重複の条件を確認します。不正な入力は、問題の箇所と理由を示して拒否します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-002` | Functional Requirement |

### 14.1 YAML の形式と値

| 条件 | 判定 |
|---|---|
| マッピングのキー | 定義された文字列のキーだけを受理。未知のキーと文字列以外のキーを拒否 |
| 真偽値 | タグを明示しない、引用符のない小文字の `true` / `false` だけを受理。引用符付き、`True` / `FALSE`、`yes` / `no`、数値などは拒否 |
| 整数 | タグを明示しない、引用符のない10進の表記（`0`、または `0` 以外の数字で始まる数字の列）だけを受理。引用符付き、小数、16進・8進、先頭の `0`、符号付き、`_` を含む表記は拒否 |
| 文字列の列挙値 | 定義された文字列に完全一致する場合だけ受理。引用符の有無は意味を変えない |
| 文字列 | 型が文字列と定められた場所では文字列を要求。数値・真偽値・`null` から暗黙に変換しない。変数の `value` では、引用符のない数値・真偽値・日付に、引用符で囲んだ表記を直し方として添える |
| `null` と値なし | どの位置でも拒否 |
| 同一マッピング内の重複キー | 拒否。`name` と `"name"` のように表記が違っても同じキーとみなし、後勝ちで上書きしない |
| 空のシーケンス・マッピング | `repositories: []`、`environments: []`、`variables: []`、空のリポジトリごとの宣言、空の `general` などを拒否。`rulesets: []` も拒否する。`patterns_allowed: []`、`deployment_branch_policy.patterns: []`、`required_reviewers: []`、Ruleset の `include: []`、`exclude: []`、`bypass_actors: []`、`rules: []` は例外として有効 |
| 関連する設定の欠落・矛盾 | 拒否。必要な設定の組み合わせを示す |
| YAML 文書数 | 1ファイルに1文書だけを許可。文書がない場合（空のファイル、コメントだけのファイル）と複数文書を拒否 |
| YAML のアンカー（anchor）、エイリアス（alias）、マージキー（merge key） | 拒否。宣言の共有・継承を認めない |
| 明示的なタグ（`!!bool`・`!!str` などの標準タグ、`!custom` などの独自タグ、非特定タグ `!`） | 拒否。値の型や受理条件をタグで上書きできないようにする |
| 文字コード | UTF-8 だけを受理。UTF-16 などほかの符号化を拒否 |

真偽値の受理可否は、YAML パーサーが最終的に真偽値へ変換したかどうかだけでは判断しません。たとえば YAML 1.1 の解釈により `yes` が真偽値となるパーサーであっても、reco は受け付けません。

真偽値を書くつもりだったと読める表記は、拒否の理由に直し方を添えます。引用符で囲んだ `true` / `false`（大文字小文字を問わない）には引用符を外した小文字の値を、引用符のない `True` / `TRUE` / `False` / `FALSE` には小文字の値を示します。`yes` や数値などには直し方を添えません。どの真偽値を意図したかが表記から決まらないためです。

次の例は、必要な親要素とほかの設定が適正に存在する場合の個々の値の判定です。

| 部分例 | 判定 |
|---|---|
| `allow_auto_merge: true` | 有効 |
| `allow_auto_merge: "true"` | エラー（文字列） |
| `allow_auto_merge: TRUE` | エラー（受理する表記ではない） |
| `allow_auto_merge: !!bool "true"` | エラー（明示タグ） |
| `allow_squash_merge: false`、`allow_merge_commit: false`、`allow_rebase_merge: false` を同時に宣言 | エラー（3つのマージ方式をすべて無効にできない） |
| `squash_merge_commit: {title: PR_TITLE}` | エラー（`message` が必要） |
| `squash_merge_commit: {title: COMMIT_OR_PR_TITLE, message: PR_BODY}` | エラー（GitHub が受け付けない組み合わせ） |
| `squash_merge_commit: {title: pr_title, message: PR_BODY}` | エラー（定義された文字列に完全一致しない） |
| `allowed_actions: "selected"` | 値の形式は有効。ただし `selected_actions` の3項目も必要 |
| `access_level: organization` | エラー（宣言で受け付ける値ではない） |
| `approval_policy: FIRST_TIME_CONTRIBUTORS` | エラー（定義された文字列に完全一致しない） |
| `days: 30` | 有効 |
| `days: "30"` | エラー（引用符付き） |
| `days: 0` | エラー（正の整数ではない） |
| `run_workflows_from_fork_pull_requests: false` と `send_secrets_and_variables: true` を同時に宣言 | エラー（実行を許可しないまま、Secrets と Variables の送信を有効にできない） |
| `required_reviewers: []` | 有効（担当者がいないことの宣言） |
| `required_reviewers: [{type: Team, id: 42}]` | エラー（`Team` は受け付けない） |
| `required_reviewers: [{type: User, id: "1234567"}]` | エラー（引用符付き） |
| `required_reviewers` に7件の要素 | エラー（6件まで） |
| `required_reviewers: []` と `prevent_self_review: true` を同時に宣言 | エラー（担当者がいない Environment に `prevent_self_review` を設定できない） |
| `wait_timer: 0` | 有効（待機時間がないことの宣言） |
| `wait_timer: 43200` | エラー（`43199` まで） |
| `wait_timer: 2.0` | エラー（小数） |
| `value: "1.27"`（変数の値） | 有効 |
| `value: 1.27`（変数の値） | エラー（文字列ではない。`"1.27"` を直し方として示す） |
| `value: ""`（変数の値） | エラー（空文字列） |
| `value: ~`（変数の値） | エラー（`null`） |
| `use_default: false` だけ（`oidc_subject`） | エラー（key が必要） |
| `use_default: true` と `include_claim_keys: [repo]`（`oidc_subject`） | エラー（既定の形を使うときは key を指定できない） |
| `include_claim_keys: [repo, job-workflow]` | エラー（`-` は claim key の形式に合わない） |
| `include_claim_keys: [Repo]` | 値の形式は有効。GitHub が受け付けないため、`apply` の更新操作が失敗する |

### 14.2 リポジトリ名と重複

`name` は、`OWNER` と `REPO` を一つの `/` で区切った `OWNER/REPO` 形式の文字列です。`OWNER` と `REPO` はともに空ではなく、ASCII の英数字と `-`、`_`、`.` だけからなり、`.` と `..` ではありません。このため、空白、制御文字、ASCII 以外の文字を含む名前は受け付けません。URL や、末尾に `.git` を付けたクローン用の表記（`.GIT` のように大文字小文字が異なるものを含む）は受け付けません。この値は名前による **リポジトリ指定** であり、GitHub のリポジトリ ID や、実体の存在確認を表すものではありません。

GitHub は所有者名・リポジトリ名を大文字小文字非依存で扱います。このため、reco はリポジトリ指定どうしを、所有者名とリポジトリ名の大文字小文字を区別せずに比較し、一致するものを同一とみなします。同一の設定宣言に同一とみなされる `name` が複数ある場合は、同一文字列か **大文字小文字だけが異なるか** を問わず入力時に拒否します。たとえば `Example/Project-A` と `example/project-a` を同一の設定宣言に並べることはできません。

表記が異なる名前がリネーム・移転などによって GitHub 上の同一リポジトリ ID を指すかどうかは、YAML の検証では判定しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 15. References

- [要求定義](requirements.md)
- [設定ごとの設計](../design/settings/README.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
- [GitHub Actions OIDC API 仕様](../research/actions-oidc.md)
- [GitHub Actions 変数 API 仕様](../research/actions-variables.md)
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)
- [GitHub Rules API 仕様](../research/rules.md)
- [YAML 1.2.2 Specification](https://yaml.org/spec/1.2.2/)
