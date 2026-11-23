# YAML 設定仕様

## 1. 目的と基本構造

本ドキュメントは、reco が受け付ける YAML 設定宣言の構造、型、許容値、組み合わせ、管理範囲の解釈、入力検証を定めます。利用者はリポジトリごとに管理する設定と期待状態を明示します。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
      allow_auto_merge: true
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
      variables:
        - name: GO_VERSION
          value: "1.27"
    environments:
      - name: production
        deployment_branch_policy:
          mode: protected_branches
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
| `general`、`actions`、`permissions`、`selected_actions`、`workflow`、`access`、`security` | マッピング | 使用する場合は、その階層で定義された設定または子のマッピングを一つ以上含む。`actions` は `permissions`、`workflow`、`access`、`variables` のうち一つ以上を含めばよい |
| `variables`（`actions` 配下） | シーケンス | 使用する場合は、1件以上の変数の宣言を含む（7章） |
| `environments` | シーケンス | 使用する場合は、1件以上の Environment の宣言を含む（8章） |
| `variables`（`environments[]` 配下） | シーケンス | 使用する場合は、1件以上の変数の宣言を含む（8.3節） |
| `rulesets` | シーケンス | 使用する場合は、1件以上の Ruleset の宣言を含む（10章） |

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

設定の一部は、意味を確定するために複数の値を一体として宣言します。この場合、個々の値を独立した管理の単位にはせず、一つの設定グループとして扱います。具体的な組み合わせは第4章で定めます。

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

これらはリポジトリ単位の設定であり、変更しても個々の Pull Request のマージ、ブランチの更新、ブランチの直接削除は行いません。`allow_update_branch` は更新を提案するボタンを表示する設定で、reco も GitHub もブランチを自動では更新しません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` の3つを **マージ方式** と呼びます。3つは独立した管理の単位で、一部だけを宣言できます。`allow_update_branch` はマージ方式に数えず、次の組み合わせの規則に関わりません。

| 宣言したマージ方式 | 判定 |
|---|---|
| 3つとも `false` | エラー |
| 1つ以上が `true` | 有効 |
| 一部を省略し、宣言したものがすべて `false` | 有効 |

3つとも `false` と宣言できないのは、GitHub が、3つのマージ方式をすべて無効にする更新を拒否し（[GitHub リポジトリ API 仕様](../research/repositories.md) 4章）、この状態を実現できないためです。この誤りは、`general` の6つの設定のいずれかの値が真偽値でない誤りより後に示します。一部を省略した宣言では、宣言していないマージ方式の値は宣言に含まれず、reco はそれを有効にしません。

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

## 7. Repository Variables

`actions.variables` は、リポジトリの Repository Variable を名前ごとに宣言するシーケンスです。Repository Variable は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.variables` | シーケンス | 変数の宣言を1件以上 | 管理する Repository Variable の一覧 | なし（要素ごとに定まる） |
| `actions.variables[].name` | 文字列 | 7.1節の名前 | 管理する Repository Variable の名前 | Repository Variable（関連リソースそのもの） |
| `actions.variables[].value` | 文字列 | 7.2節の値。`state` を書かない要素では必須 | その名前の Repository Variable が存在し、その値であること | Repository Variable（関連リソースそのもの） |
| `actions.variables[].state` | 文字列 | `absent` | その名前の Repository Variable が存在しないこと | Repository Variable（関連リソースそのもの） |

- **`name` は必須で、`value` か `state` のどちらか一方を書く**：要素が持てるキーはこの3つだけである。組み合わせは 7.3節が定める
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

### 7.1 変数の名前

`name` は、ASCII の英大文字、数字、`_` だけからなる、空でない文字列です。数字で始まらず、大文字小文字を問わず `GITHUB_` で始まりません。GitHub が受け付ける名前の規則（[GitHub Actions 変数 API 仕様](../research/actions-variables.md) 2章）に、英小文字を含まないという制限を加えたものです。

英小文字を受け付けないのは、GitHub が名前を大文字にして保存するためです（同 2章）。小文字を含む宣言は GitHub 上の綴りと必ず食い違い、`show` と `plan` に二つの綴りが現れます。小文字を含む名前は、大文字にした綴りを示して拒否します。reco が大文字に読み替えて受け付けないのは、利用者が書いた名前を黙って変えないためです。大文字小文字を問わず `GITHUB_` で始まる名前は、小文字の誤りより先に `GITHUB_` の誤りとして示します。大文字にした綴りも受け付けない名前だからです。

名前の長さは入力検証で確かめません。公式資料が上限を記載しておらず、GitHub の上限が変わり得るためです。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。GitHub が名前を大文字小文字を区別せずに扱うためです（同 2章）。同じリポジトリの `actions.variables` に同じ名前が複数あれば、入力不正として拒否します。

### 7.2 変数の値

`value` は、空でない文字列です。値は YAML のエスケープ表現を解釈した後の文字列として扱い、トリミング、改行の正規化、Unicode の正規化をしません。GitHub 上の値とは文字列の完全一致で比べ、前後の空白や末尾の改行だけが違う値も異なる値とします。GitHub は空文字列のほかの値を送ったとおりに保存するためです（[GitHub Actions 変数 API 仕様](../research/actions-variables.md) 3章）。

- **引用符のない数値、真偽値、日付は拒否する**：`value: 1.27`、`value: 1`、`value: true`、`value: 2026-11-22` などは、YAML が文字列でない値と解釈するため、文字列へ暗黙に変換しない（11.1節）。拒否の理由に、引用符で囲んだ表記（`"1.27"`）を直し方として添える
- **空文字列は拒否する**：`value: ""`、`value: ''`、中身のないブロックスカラーを受け付けない。GitHub が空文字列の値の作成も更新も拒否するため（同 3章）
- **`null` と値なしは、空文字列とは別の誤りとして拒否する**：`value: ~`、`value: null`、値を書かない `value:` は、11.1節の `null` と値なしの誤りである
- **ブロックスカラーの末尾の改行は値に含まれる**：`value: |` で書いた値は末尾に改行を持つ。GitHub 上の値が末尾に改行を持たなければ、異なる値になる

値の長さは入力検証で確かめません。名前の長さと同じく、GitHub の上限が変わり得るためです。

### 7.3 存在しないことの宣言

`state` に書ける値は `absent` だけです。`value` と `state` の組み合わせは次のとおりです。

| `value` | `state` | 判定 |
|---|---|---|
| 7.2節の値 | 省略 | 有効。存在し、値を伴うことの宣言 |
| 省略 | `absent` | 有効。存在しないことの宣言 |
| 省略 | 省略 | エラー |
| 指定（中身を問わない） | `absent` | エラー |
| 省略または指定 | `absent` 以外 | エラー |

`state` は大文字小文字を区別し、`Absent`、`present`、空文字列、文字列でない値、`null` と値なしを受け付けません。`state` の誤りは、`value` との組み合わせの誤りより先に示します。組み合わせの誤りは、`value` の中身の誤りより先に示します。

`value: ~`、`value: ""`、`value` の省略は、どれも存在しないことの宣言になりません（7.2節）。値の書き忘れが削除にならないようにするためです。`value` も `state` もない要素は、`value` がないことを示して拒否し、`state: absent` を案内しません。値を書き忘れた利用者を削除へ誘導しないためです。

```yaml
actions:
  variables:
    - name: LEGACY_FLAG
      state: absent
```

同じ名前の要素を、値の宣言と存在しないことの宣言に分けて書くことはできません。7.1節の名前の重複として拒否します。

## 8. Environments

`environments` は、リポジトリの Environment を名前ごとに宣言するシーケンスです。Environment は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `environments[].name` | 文字列 | 8.1節の名前 | その名前の Environment が存在すること | Environment（関連リソースそのもの） |
| `environments[].deployment_branch_policy.mode` | 文字列 | `all` / `protected_branches` / `selected` | Environment へデプロイできるブランチとタグの制限の方式 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns` | シーケンス | 個別パターンの要素を0件以上（`[]` 可）。`mode` が `selected` のときだけ必須で、ほかの方式では指定できない（8.2節） | デプロイを許可するブランチとタグのパターンの一覧の全体 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].type` | 文字列 | `branch` / `tag` | パターンがブランチとタグのどちらに一致するか | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].name` | 文字列 | 8.2節の名前 | ブランチ名またはタグ名のパターン | デプロイ元制限（設定グループ） |
| `environments[].variables` | シーケンス | 変数の宣言を1件以上（8.3節） | その Environment で管理する Environment Variable の一覧 | なし（要素ごとに定まる） |
| `environments[].variables[].name` | 文字列 | 7.1節の名前 | その Environment で管理する Environment Variable の名前 | Environment Variable（関連リソースそのもの） |
| `environments[].variables[].value` | 文字列 | 7.2節の値。`state` を書かない要素では必須 | その Environment に、その名前の Environment Variable が存在し、その値であること | Environment Variable（関連リソースそのもの） |
| `environments[].variables[].state` | 文字列 | `absent` | その Environment に、その名前の Environment Variable が存在しないこと | Environment Variable（関連リソースそのもの） |

- **`name` は必須である**：`name` のない要素は、`deployment_branch_policy` や `variables` を書いていても入力不正として拒否する。どの Environment の宣言かを補わない
- **要素は存在の宣言を兼ねる**：`environments[]` に書いた名前の Environment が存在しなければ、作成を計画する。`variables` だけを書いた Environment も同じである。`deployment_branch_policy` と `variables` を省略した Environment は、存在だけを管理する
- **存在しないことの宣言は受け付けない**：reco は Environment を削除しない
- **`deployment_branch_policy` は `mode` を必須とする**：`all` はすべてのブランチとタグからのデプロイを許可する。`protected_branches` は、保護ルールのあるブランチからのデプロイだけを許可する。`protected_branches` の宣言は、Branch protection を管理することを意味しない。`selected` は、`patterns` に書いたパターンに一致するブランチとタグからのデプロイだけを許可する

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 8.1 Environment の名前

`name` は、空でなく、255 文字以下で、印字可能な ASCII 文字（U+0020 から U+007E）だけからなる文字列です。前後に空白を持たず、`.` と `..` ではありません。`/` は位置を問わず含められ、空白は前後でなければ含められます。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。同じリポジトリの宣言に同じとみなされる名前が複数あれば、入力不正として拒否します。たとえば `Production` と `production` を同じリポジトリに並べることはできません。

ASCII 以外の文字と制御文字を受け付けないのは、GitHub が大文字小文字のほかに、アクセントの有無、全角と半角、末尾の空白、制御文字の違いも同じ名前とみなすためです（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 2章）。reco はこの判定を再現できません。受け付けると、GitHub 上で綴りの違う同じ Environment を存在しないと判断し、作成の要求で既存の Environment を変え得ます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 8.2 デプロイ元制限の入力の組み合わせと `patterns`

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

### 8.3 Environment Variables

`environments[].variables` は、その Environment に属する Environment Variable を名前ごとに宣言するシーケンスです。Environment Variable は関連リソースであり、2.1節の規則に従います。2.1節の2つ目の規則（書いた関連リソースが存在しなければ作成する）は、変数にも、属する先の Environment にも当てはまります。変数を書くことは、属する先の Environment が存在することの宣言を兼ねるためです。

```yaml
environments:
  - name: production
    variables:
      - name: REGION
        value: ap-northeast-1
```

- **要素の形は `actions.variables` と同じである**：`name` は必須で、`value` か `state: absent` のどちらか一方を書く。名前は 7.1節、値は 7.2節、存在しないことの宣言は 7.3節の規則に従う
- **`value` を書いた要素は存在と値の宣言を兼ねる**：書いた名前の Environment Variable が存在しなければ作成を計画し、存在して値が違えば値の変更を計画する
- **`state: absent` を書いた要素は、存在しないことの宣言である**：その名前の Environment Variable が存在すれば削除を計画する。`state: absent` の変数だけを書いた Environment も、Environment の存在の宣言を兼ね、存在しなければ作成を計画する
- **重複は一つの Environment の中で判定する**：同じ Environment の `variables` に同じ名前（7.1節の判定）が複数あれば、入力不正として拒否する。別の Environment の同じ名前と、`actions.variables` の同じ名前は、重複ではない
- **同じ名前の Repository Variable とは別の管理設定である**：`actions.variables` と `environments[].variables` に同じ名前を書いても、値を同期しない

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 変数 API 仕様](../research/actions-variables.md) | Repository File |

## 9. Security

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `security.dependabot_alerts` | 真偽値 | `true` / `false` | 依存関係の脆弱性を Dependabot alerts で知らせるか | 独立 |
| `security.dependabot_security_updates` | 真偽値 | `true` / `false` | 脆弱性を直す Pull Request を Dependabot security updates が作るか | 独立 |
| `security.secret_scanning` | 真偽値 | `true` / `false` | リポジトリに push されたシークレットを Secret scanning で検出するか | 独立 |
| `security.secret_scanning_push_protection` | 真偽値 | `true` / `false` | シークレットを含む push を、リポジトリの push protection で拒否するか。利用者が自分のアカウントに設定する push protection とは別の設定 | 独立 |
| `security.private_vulnerability_reporting` | 真偽値 | `true` / `false` | 見つけた脆弱性を、公開せずにメンテナーへ報告できる Private vulnerability reporting を有効にするか | 独立 |

5つの設定は独立した管理の単位で、どれか一つだけを宣言できます。5つとも省略した `security` は、空のマッピングとして拒否します（11.1節）。Dependabot の2設定のどちらの期待値も、もう一方の設定の状態を含みません。Secret scanning の2設定についても同じです。

5つの設定のいずれかの値が真偽値でない誤りは、次に挙げる組み合わせの誤りより先に示します。Dependabot の2設定の組み合わせの誤りと、Secret scanning の2設定の組み合わせの誤りが両方あるときは、Dependabot の誤りを示します。

### 9.1 Dependabot

GitHub が Dependabot security updates を一時停止していても、設定としては有効のままです（[GitHub リポジトリ API 仕様](../research/repositories.md) 7章）。そのため、一時停止中の Dependabot security updates は `dependabot_security_updates: true` と一致します。一時停止は宣言できず、reco は一時停止を解除しません。

2つの設定の組み合わせは次のとおりです。

| `dependabot_alerts` | `dependabot_security_updates` | 判定 |
|---|---|---|
| `true` | 省略、`true`、`false` | 有効 |
| `false` | 省略、`false` | 有効 |
| `false` | `true` | エラー |
| 省略 | `true`、`false` | 有効 |

`dependabot_alerts: false` と `dependabot_security_updates: true` を同時に宣言できないのは、GitHub が、Dependabot alerts が無効のリポジトリで Dependabot security updates の有効化を拒否し（GitHub リポジトリ API 仕様 7章）、この状態を実現できないためです。値の誤りを先に示すのは、組み合わせが2つの値で決まるためです。Dependency graph は宣言を受け付けない設定です。

### 9.2 Secret scanning

2つの設定の組み合わせは次のとおりです。

| `secret_scanning` | `secret_scanning_push_protection` | 判定 |
|---|---|---|
| `true` | 省略、`true`、`false` | 有効 |
| `false` | 省略、`false` | 有効 |
| `false` | `true` | エラー |
| 省略 | `true`、`false` | 有効 |

`secret_scanning: false` と `secret_scanning_push_protection: true` を同時に宣言できないのは、GitHub が、Secret scanning を無効にするとリポジトリの push protection も無効にし、Secret scanning が無効のあいだはリポジトリの push protection を有効にする更新を反映しないため（GitHub リポジトリ API 仕様 5章）、この状態を実現できないためです。

2つの設定は、reco が公開リポジトリでだけ管理する設定です。

### 9.3 Private vulnerability reporting

`private_vulnerability_reporting` は、ほかの設定との組み合わせの規則を持ちません。reco が公開リポジトリでだけ管理する設定です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 10. Rulesets

`rulesets` は、リポジトリの Ruleset を名前ごとに宣言するシーケンスです。Ruleset は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `rulesets` | シーケンス | Ruleset の宣言を1件以上 | 管理する Ruleset の一覧 | なし（要素ごとに定まる） |
| `rulesets[].name` | 文字列 | 10.1節の名前 | 管理する Ruleset の名前 | Ruleset（関連リソースそのもの） |
| `rulesets[].target` | 文字列 | `branch` | Ruleset が対象にする ref の種類 | Ruleset（関連リソースそのもの） |
| `rulesets[].enforcement` | 文字列 | `active` / `disabled` | ルールを強制するか | Ruleset（関連リソースそのもの） |
| `rulesets[].conditions.ref_name.include` | シーケンス | 10.3節のパターンを0件以上（`[]` 可） | 対象にするブランチのパターン | Ruleset（関連リソースそのもの） |
| `rulesets[].conditions.ref_name.exclude` | シーケンス | 10.3節のパターンを0件以上（`[]` 可） | 対象から外すブランチのパターン | Ruleset（関連リソースそのもの） |
| `rulesets[].bypass_actors` | シーケンス | 10.4節の要素を0件以上（`[]` 可） | ルールを Bypass できる主体の全体 | Ruleset（関連リソースそのもの） |
| `rulesets[].rules` | シーケンス | 10.5節の要素を0件以上（`[]` 可） | Ruleset が強制するルールの全体 | Ruleset（関連リソースそのもの） |

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

### 10.1 Ruleset の名前

`name` は、空でなく、印字可能な ASCII 文字（U+0020 から U+007E）だけからなり、前後に空白を持たない文字列です。空白は前後でなければ含められ、`/` や `.` も含められます。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。同じリポジトリの `rulesets` に同じとみなされる名前が複数あれば、入力不正として拒否します。名前は `target` をまたいで一意です。

ASCII 以外の文字と制御文字を受け付けないのは、GitHub が大文字小文字のほかにアクセントの有無も同じ名前とみなし、ほかの文字の規則が分かっていないためです（[GitHub Rules API 仕様](../research/rules.md) 2章）。前後の空白を受け付けないのは、GitHub が黙って除いて保存し、宣言と保存される名前が食い違うためです（同 2章）。名前の長さは入力検証で確かめません。公式資料が上限を記載していないためです。

### 10.2 `target` と `enforcement`

`target` に書ける値は `branch` だけです。タグと Push の Ruleset は扱いません。

`enforcement` に書ける値は `active` と `disabled` です。GitHub の値には `evaluate` もありますが、reco は宣言として受け付けません。reco がサポートする個人アカウント所有のリポジトリでは、GitHub が `evaluate` を拒否するためです（[GitHub Rules API 仕様](../research/rules.md) 4.2）。

### 10.3 対象の条件

`conditions` は `ref_name` だけを持ち、`ref_name` は `include` と `exclude` の両方を持ちます。どちらも省略できず、`[]` は空集合を明示します。

パターンは、`~DEFAULT_BRANCH`（デフォルトブランチ）、`~ALL`（すべてのブランチ）、`refs/heads/` で始まる文字列のどれかです。大文字小文字を区別し、`~default_branch` や `refs/heads/` のない `main` を受け付けません。GitHub がこれらを拒否し、正規化もしないためです（[GitHub Rules API 仕様](../research/rules.md) 5章）。

空文字列、空白だけの文字列、前後に空白を含む文字列、`null`、文字列以外の要素、同じ一覧の中の同じ文字列の重複は、4.2節の `patterns_allowed` と同じく受け付けません。これらの誤りは、パターンの形の誤りより先に示し、重複はパターンの形の誤りより後に示します。`include` と `exclude` に同じ文字列を書くことはできます。値を自動補完・トリミング・正規化しません。

```yaml
conditions:
  ref_name:
    include: ["~DEFAULT_BRANCH", "refs/heads/release/*"]
    exclude: []
```

### 10.4 Bypass

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

### 10.5 ルール

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

## 11. 入力検証と対象名

入力検証では、YAML の構造・値の形式と、宣言されたリポジトリ名・重複の条件を確認します。不正な入力は、問題の箇所と理由を示して拒否します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-002` | Functional Requirement |

### 11.1 YAML の形式と値

| 条件 | 判定 |
|---|---|
| マッピングのキー | 定義された文字列のキーだけを受理。未知のキーと文字列以外のキーを拒否 |
| 真偽値 | タグを明示しない、引用符のない小文字の `true` / `false` だけを受理。引用符付き、`True` / `FALSE`、`yes` / `no`、数値などは拒否 |
| 整数 | タグを明示しない、引用符のない10進の表記（`0`、または `0` 以外の数字で始まる数字の列）だけを受理。引用符付き、小数、16進・8進、先頭の `0`、符号付き、`_` を含む表記は拒否 |
| 文字列の列挙値 | 定義された文字列に完全一致する場合だけ受理。引用符の有無は意味を変えない |
| 文字列 | 型が文字列と定められた場所では文字列を要求。数値・真偽値・`null` から暗黙に変換しない。変数の `value` では、引用符のない数値・真偽値・日付に、引用符で囲んだ表記を直し方として添える |
| `null` と値なし | どの位置でも拒否 |
| 同一マッピング内の重複キー | 拒否。`name` と `"name"` のように表記が違っても同じキーとみなし、後勝ちで上書きしない |
| 空のシーケンス・マッピング | `repositories: []`、`environments: []`、`variables: []`、空のリポジトリごとの宣言、空の `general` などを拒否。`rulesets: []` も拒否する。`patterns_allowed: []`、`deployment_branch_policy.patterns: []`、Ruleset の `include: []`、`exclude: []`、`bypass_actors: []`、`rules: []` は例外として有効 |
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
| `allowed_actions: "selected"` | 値の形式は有効。ただし `selected_actions` の3項目も必要 |
| `access_level: organization` | エラー（宣言で受け付ける値ではない） |
| `value: "1.27"`（変数の値） | 有効 |
| `value: 1.27`（変数の値） | エラー（文字列ではない。`"1.27"` を直し方として示す） |
| `value: ""`（変数の値） | エラー（空文字列） |
| `value: ~`（変数の値） | エラー（`null`） |

### 11.2 リポジトリ名と重複

`name` は、`OWNER` と `REPO` を一つの `/` で区切った `OWNER/REPO` 形式の文字列です。`OWNER` と `REPO` はともに空ではなく、ASCII の英数字と `-`、`_`、`.` だけからなり、`.` と `..` ではありません。このため、空白、制御文字、ASCII 以外の文字を含む名前は受け付けません。URL や、末尾に `.git` を付けたクローン用の表記（`.GIT` のように大文字小文字が異なるものを含む）は受け付けません。この値は名前による **リポジトリ指定** であり、GitHub のリポジトリ ID や、実体の存在確認を表すものではありません。

GitHub は所有者名・リポジトリ名を大文字小文字非依存で扱います。このため、reco はリポジトリ指定どうしを、所有者名とリポジトリ名の大文字小文字を区別せずに比較し、一致するものを同一とみなします。同一の設定宣言に同一とみなされる `name` が複数ある場合は、同一文字列か **大文字小文字だけが異なるか** を問わず入力時に拒否します。たとえば `Example/Project-A` と `example/project-a` を同一の設定宣言に並べることはできません。

表記が異なる名前がリネーム・移転などによって GitHub 上の同一リポジトリ ID を指すかどうかは、YAML の検証では判定しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 12. References

- [要求定義](requirements.md)
- [設定ごとの設計](../design/settings/README.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
- [GitHub Actions 変数 API 仕様](../research/actions-variables.md)
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)
- [GitHub Rules API 仕様](../research/rules.md)
- [YAML 1.2.2 Specification](https://yaml.org/spec/1.2.2/)
