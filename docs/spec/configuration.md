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
    environments:
      - name: production
        deployment_branch_policy:
          mode: protected_branches
      - name: staging

  - name: example/project-b
    general:
      delete_branch_on_merge: true
```

| 要素 | 型 | 条件 |
|---|---|---|
| ルート | マッピング | `repositories` だけを持つ |
| `repositories` | シーケンス | 1件以上のリポジトリごとの宣言を含む |
| `repositories[]` | マッピング | `name` と、少なくとも一つの管理設定を含む `general`、`actions`、`environments` のいずれかを持つ |
| `repositories[].name` | 文字列 | 必須。空でない `OWNER/REPO` 形式 |
| `general`、`actions`、`permissions`、`selected_actions`、`workflow`、`access` | マッピング | 使用する場合は、その階層で定義された設定または子のマッピングを一つ以上含む。`actions` は `permissions`、`workflow`、`access` のうち一つ以上を含めばよい |
| `environments` | シーケンス | 使用する場合は、1件以上の Environment の宣言を含む（7章） |

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

設定の一部は、意味を確定するために複数の値を一体として宣言します。この場合、個々の値を独立した管理の単位にはせず、一つの設定グループとして扱います。具体的な組み合わせは第4章で定めます。なお、観測しただけの設定や、別の設定の更新時に保全のため既存値を送信する設定は、管理範囲に追加しません。

入力が正しいことと、GitHub 上で安全に適用できることは別の判断です。リポジトリの実在性、権限、現在状態、API の取得条件、更新途中の安全性、未管理設定の保全は、入力検証では確定しません。これらは選択されたリポジトリの観測後に適用可能性として判断します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

### 2.1 関連リソースの宣言

Environment や Variable のように、一つのリポジトリに名前の違うものが複数あり得る関連リソースは、名前ごとに宣言します。規則は次のとおりです。

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

これらはリポジトリ単位の許可設定であり、変更しても個々の Pull Request のマージやブランチの直接削除は行いません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 4. Actions permissions

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `actions.permissions.enabled` | 真偽値 | `true` / `false` | GitHub Actions を有効にするか | Actions の有効状態（独立） |
| `actions.permissions.allowed_actions` | 文字列 | `all` / `local_only` / `selected` | Actions と再利用可能なワークフローの実行許可範囲 | Actions 許可ポリシー（独立） |
| `actions.permissions.selected_actions.github_owned_allowed` | 真偽値 | `true` / `false` | GitHub 所有 Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.verified_allowed` | 真偽値 | `true` / `false` | Marketplace の認証済み作成者が提供する Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.patterns_allowed` | シーケンス | 文字列の要素を0件以上（`[]` 可） | 許可する Actions と再利用可能なワークフローのパターン | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.sha_pinning_required` | 真偽値 | `true` / `false` | Actions を完全な長さのコミット SHA に固定することを必須にするか | 独立 |

`selected_actions` の3項目は選択方式の Actions 許可条件を表し、 **一つの設定グループ** として扱います。3項目を一体として宣言するのは reco の入力契約であり、GitHub への更新の単位を定めるものではありません。

`enabled: false` は Actions を無効にする宣言です。reco が Actions を無効にするのは、`enabled: false` を宣言したリポジトリに限ります。

`sha_pinning_required` は独立した管理の単位で、`allowed_actions` を宣言しなくても宣言できます。Actions が無効のリポジトリで、`enabled: true` も `allowed_actions` も宣言せずに `sha_pinning_required` を宣言すると、その変更予定は適用不能になります（[アーキテクチャ設計](../design/architecture.md) 5.4）。この判断は入力の検証ではなく、観測のあとに行う適用可能性の判断です（2章）。

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

3項目を必須とする点は **reco 固有の入力契約** です。GitHub REST API では Selected actions の3項目は任意パラメータですが、reco は選択方式の Actions 許可条件の期待状態を一意に定めるため、3項目を必須とします。入力として有効でも、現在の Actions 許可ポリシーや GitHub の制約によって、選択方式の Actions 許可条件を取得・適用できるとは限りません。

### 4.2 `patterns_allowed`

`patterns_allowed` は文字列の要素からなるシーケンスで、`[]` は空集合を明示します。空文字列、空白だけの文字列、前後に空白を含む文字列、`null`、文字列以外の要素、および同一文字列の重複は受け付けません。空白は、Unicode の空白文字（改行やノーブレークスペースを含む）を指します。重複は YAML のエスケープ表現を解釈した後の文字列で判定し、大文字小文字だけが異なる文字列は別の要素として扱います。値を自動補完・トリミング・正規化しません。

```yaml
patterns_allowed:
  - "actions/checkout@v7"
  - "actions/setup-go@v6"
```

許可パターンは、reco の照合では **文字列の集合** として扱います。配列要素の順序だけが違う場合は差分とみなしません。GitHub から取得した値が同じ文字列を複数含む場合も一つの要素として扱い、重複の有無だけが違う場合は差分とみなしません。一方、ワイルドカードや異なるパターンの意味的な包含関係は解釈せず、文字列の一致によって扱います。GitHub がこれらのパターンを実際に許可条件として扱うかどうかは、GitHub の仕様と対象リポジトリの条件に従います。

GitHub の公式資料は `patterns_allowed` を公開リポジトリに適用する設定としています。非公開リポジトリでの保存・再取得・適用の具体的な挙動を、この入力契約によって保証するものではありません。

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

GitHub の値には `organization` もありますが、reco は宣言として受け付けません。`organization` は Organization が所有するリポジトリのための値で、reco がサポートするユーザー所有のリポジトリでは、GitHub がこの値への更新を拒否するためです（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 6章）。一方、Organization から移転したリポジトリなどで `organization` を観測した場合は、宣言した値と異なる値として扱い、宣言した値への変更を計画します。

`access_level` は非公開リポジトリにだけ適用される設定です。公開リポジトリで宣言すると、その変更予定は適用不能になります（[アーキテクチャ設計](../design/architecture.md) 5.4）。この判断は入力の検証ではなく、観測のあとに行う適用可能性の判断です（2章）。Actions の有効状態は、この設定の宣言にも適用可能性にも関係しません。`actions.permissions.enabled: false` と同時に宣言できます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |

## 7. Environments

`environments` は、リポジトリの Environment を名前ごとに宣言するシーケンスです。Environment は関連リソースであり、2.1節の規則に従います。

| YAML パス（リポジトリ配下） | 型 | 許容値 | 意味 | 管理の単位 |
|---|---|---|---|---|
| `environments[].name` | 文字列 | 7.1節の名前 | その名前の Environment が存在すること | Environment（関連リソースそのもの） |
| `environments[].deployment_branch_policy.mode` | 文字列 | `all` / `protected_branches` / `selected` | Environment へデプロイできるブランチとタグの制限の方式 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns` | シーケンス | 個別パターンの要素を0件以上（`[]` 可）。`mode` が `selected` のときだけ必須で、ほかの方式では指定できない（7.2節） | デプロイを許可するブランチとタグのパターンの一覧の全体 | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].type` | 文字列 | `branch` / `tag` | パターンがブランチとタグのどちらに一致するか | デプロイ元制限（設定グループ） |
| `environments[].deployment_branch_policy.patterns[].name` | 文字列 | 7.2節の名前 | ブランチ名またはタグ名のパターン | デプロイ元制限（設定グループ） |

- **`name` は必須である**：`name` のない要素は、ほかのキーを書いていても入力不正として拒否する。どの Environment の宣言かを補わない
- **要素は存在の宣言を兼ねる**：`environments[]` に書いた名前の Environment が存在しなければ、作成を計画する。`deployment_branch_policy` を省略した Environment は、存在だけを管理する
- **存在しないことの宣言は受け付けない**：reco は Environment を削除しない
- **`deployment_branch_policy` は `mode` を必須とする**：`all` はすべてのブランチとタグからのデプロイを許可する。`protected_branches` は、保護ルールのあるブランチからのデプロイだけを許可する。`protected_branches` の宣言は、Branch protection を管理することを意味しない。`selected` は、`patterns` に書いたパターンに一致するブランチとタグからのデプロイだけを許可する

同じ Environment の変更予定は、存在、`deployment_branch_policy` の順に表示し、実行します。存在しない Environment の中の設定は、Environment を作成してからでないと変えられないためです。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 7.1 Environment の名前

`name` は、空でなく、255 文字以下で、印字可能な ASCII 文字（U+0020 から U+007E）だけからなる文字列です。前後に空白を持たず、`.` と `..` ではありません。`/` は位置を問わず含められ、空白は前後でなければ含められます。

二つの名前が同じかどうかは、ASCII の英字の大文字小文字を区別せずに比べます。同じリポジトリの宣言に同じとみなされる名前が複数あれば、入力不正として拒否します。たとえば `Production` と `production` を同じリポジトリに並べることはできません。

ASCII 以外の文字と制御文字を受け付けないのは、GitHub が大文字小文字のほかに、アクセントの有無、全角と半角、末尾の空白、制御文字の違いも同じ名前とみなすためです（[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 2章）。reco はこの判定を再現できません。受け付けると、GitHub 上で綴りの違う同じ Environment を存在しないと判断し、作成の要求で既存の Environment を変え得ます。

GitHub 上の Environment の名前に ASCII 以外の文字などが含まれ、宣言した名前と同じかどうかを reco が判定できない場合の扱いは、入力検証ではなく、観測のあとに行う適用可能性の判断です（2章、[アーキテクチャ設計](../design/architecture.md) 5.4）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

### 7.2 デプロイ元制限の入力の組み合わせと `patterns`

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

`patterns` は、reco の照合では `type` と `name` の組の **集合** として扱い、要素の順序だけが違う場合は差分とみなしません。ワイルドカードの意味的な包含関係は解釈せず、文字列の一致によって扱います。`patterns` は一覧の全体であり、宣言した一覧が適用後の一覧になります（2.1節）。`patterns: []` は、どのブランチとタグからもデプロイできない状態の宣言です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

## 8. 入力検証と対象名

入力検証では、YAML の構造・値の形式と、宣言されたリポジトリ名・重複の条件を確認します。不正な入力は、問題の箇所と理由を示して拒否します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-002` | Functional Requirement |

### 8.1 YAML の形式と値

| 条件 | 判定 |
|---|---|
| マッピングのキー | 定義された文字列のキーだけを受理。未知のキーと文字列以外のキーを拒否 |
| 真偽値 | タグを明示しない、引用符のない小文字の `true` / `false` だけを受理。引用符付き、`True` / `FALSE`、`yes` / `no`、数値などは拒否 |
| 文字列の列挙値 | 定義された文字列に完全一致する場合だけ受理。引用符の有無は意味を変えない |
| 文字列 | 型が文字列と定められた場所では文字列を要求。数値・真偽値・`null` から暗黙に変換しない |
| `null` と値なし | どの位置でも拒否 |
| 同一マッピング内の重複キー | 拒否。`name` と `"name"` のように表記が違っても同じキーとみなし、後勝ちで上書きしない |
| 空のシーケンス・マッピング | `repositories: []`、`environments: []`、空のリポジトリごとの宣言、空の `general` などを拒否。`patterns_allowed: []` と `deployment_branch_policy.patterns: []` は例外として有効 |
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
| `allowed_actions: "selected"` | 値の形式は有効。ただし `selected_actions` の3項目も必要 |
| `access_level: organization` | エラー（宣言で受け付ける値ではない） |

### 8.2 リポジトリ名と重複

`name` は、`OWNER` と `REPO` を一つの `/` で区切った `OWNER/REPO` 形式の文字列です。`OWNER` と `REPO` はともに空ではなく、ASCII の英数字と `-`、`_`、`.` だけからなり、`.` と `..` ではありません。このため、空白、制御文字、ASCII 以外の文字を含む名前は受け付けません。URL や、末尾に `.git` を付けたクローン用の表記（`.GIT` のように大文字小文字が異なるものを含む）は受け付けません。この値は名前による **リポジトリ指定** であり、GitHub のリポジトリ ID や、実体の存在確認を表すものではありません。

GitHub は所有者名・リポジトリ名を大文字小文字非依存で扱います。このため、reco はリポジトリ指定どうしを、所有者名とリポジトリ名の大文字小文字を区別せずに比較し、一致するものを同一とみなします。同一の設定宣言に同一とみなされる `name` が複数ある場合は、同一文字列か **大文字小文字だけが異なるか** を問わず入力時に拒否します。たとえば `Example/Project-A` と `example/project-a` を同一の設定宣言に並べることはできません。

表記が異なる名前がリネーム・移転などによって GitHub 上の同一リポジトリ ID を指すかどうかは、YAML の検証では判定しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 9. References

- [要求定義](requirements.md)
- [CLI 仕様](cli.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)
- [YAML 1.2.2 Specification](https://yaml.org/spec/1.2.2/)
