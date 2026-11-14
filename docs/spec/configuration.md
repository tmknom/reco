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

  - name: example/project-b
    general:
      delete_branch_on_merge: true
```

| 要素 | 型 | 条件 |
|---|---|---|
| ルート | マッピング | `repositories` だけを持つ |
| `repositories` | シーケンス | 1件以上のリポジトリごとの宣言を含む |
| `repositories[]` | マッピング | `name` と、少なくとも一つの管理設定を含む `general` または `actions` を持つ |
| `repositories[].name` | 文字列 | 必須。空でない `OWNER/REPO` 形式 |
| `general`、`actions`、`permissions`、`selected_actions`、`workflow` | マッピング | 使用する場合は、その階層で定義された設定または子のマッピングを一つ以上含む。`actions` は `permissions` と `workflow` の一方だけでもよい |

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
| `actions.permissions.allowed_actions` | 文字列 | `all` / `local_only` / `selected` | Actions と再利用可能なワークフローの実行許可範囲 | Actions 許可ポリシー（独立） |
| `actions.permissions.selected_actions.github_owned_allowed` | 真偽値 | `true` / `false` | GitHub 所有 Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.verified_allowed` | 真偽値 | `true` / `false` | Marketplace の認証済み作成者が提供する Actions の許可 | 選択方式の Actions 許可条件（設定グループ） |
| `actions.permissions.selected_actions.patterns_allowed` | シーケンス | 文字列の要素を0件以上（`[]` 可） | 許可する Actions と再利用可能なワークフローのパターン | 選択方式の Actions 許可条件（設定グループ） |

`selected_actions` の3項目は選択方式の Actions 許可条件を表し、 **一つの設定グループ** として扱います。3項目を一体として宣言するのは reco の入力契約であり、GitHub への更新の単位を定めるものではありません。

GitHub API が提供する `enabled` や `sha_pinning_required` は、reco の YAML で宣言できません。ただし `allowed_actions` を宣言した場合は、そのポリシーで Actions が実行できること、つまり Actions が有効であることも期待状態に含めます。Actions が無効のリポジトリでは、reco は宣言したポリシーで Actions を有効にします。reco が Actions を無効にすることはありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Internal Document |

### 4.1 入力の組み合わせ

| `allowed_actions` | `selected_actions` | 判定 |
|---|---|---|
| `all` または `local_only` | 省略 | 有効 |
| `selected` | 3項目をすべて指定 | 有効 |
| `selected` | 省略、または3項目の一部だけを指定 | エラー |
| `all` または `local_only` | 指定 | エラー |
| 省略 | 指定 | エラー |
| 省略 | 省略（`permissions` 自体を宣言） | エラー（空のマッピング） |

これは **reco 固有の入力契約** です。GitHub REST API では Selected actions の3項目は任意パラメータですが、reco は選択方式の Actions 許可条件の期待状態を一意に定めるため、3項目を必須とします。入力として有効でも、現在の Actions 許可ポリシーや GitHub の制約によって、選択方式の Actions 許可条件を取得・適用できるとは限りません。

### 4.2 `patterns_allowed`

`patterns_allowed` は文字列の要素からなるシーケンスで、`[]` は空集合を明示します。空文字列、空白だけの文字列、前後に空白を含む文字列、`null`、文字列以外の要素、および同一文字列の重複は受け付けません。空白は、Unicode の空白文字（改行やノーブレークスペースを含む）を指します。重複は YAML のエスケープ表現を解釈した後の文字列で判定し、大文字小文字だけが異なる文字列は別の要素として扱います。値を自動補完・トリミング・正規化しません。

```yaml
patterns_allowed:
  - "actions/checkout@v4"
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
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Internal Document |

## 6. 入力検証と対象名

入力検証では、YAML の構造・値の形式と、宣言されたリポジトリ名・重複の条件を確認します。不正な入力は、問題の箇所と理由を示して拒否します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-002` | Functional Requirement |

### 6.1 YAML の形式と値

| 条件 | 判定 |
|---|---|
| マッピングのキー | 定義された文字列のキーだけを受理。未知のキーと文字列以外のキーを拒否 |
| 真偽値 | タグを明示しない、引用符のない小文字の `true` / `false` だけを受理。引用符付き、`True` / `FALSE`、`yes` / `no`、数値などは拒否 |
| 文字列の列挙値 | 定義された文字列に完全一致する場合だけ受理。引用符の有無は意味を変えない |
| 文字列 | 型が文字列と定められた場所では文字列を要求。数値・真偽値・`null` から暗黙に変換しない |
| `null` と値なし | どの位置でも拒否 |
| 同一マッピング内の重複キー | 拒否。`name` と `"name"` のように表記が違っても同じキーとみなし、後勝ちで上書きしない |
| 空のシーケンス・マッピング | `repositories: []`、空のリポジトリごとの宣言、空の `general` などを拒否。`patterns_allowed: []` は例外として有効 |
| 関連する設定の欠落・矛盾 | 拒否。必要な設定の組み合わせを示す |
| YAML 文書数 | 1ファイルに1文書だけを許可。文書がない場合（空のファイル、コメントだけのファイル）と複数文書を拒否 |
| YAML のアンカー（anchor）、エイリアス（alias）、マージキー（merge key） | 拒否。宣言の共有・継承を認めない |
| 明示的なタグ（`!!bool`・`!!str` などの標準タグ、`!custom` などの独自タグ、非特定タグ `!`） | 拒否。値の型や受理条件をタグで上書きできないようにする |
| 文字コード | UTF-8 だけを受理。UTF-16 などほかの符号化を拒否 |

真偽値の受理可否は、YAML パーサーが最終的に真偽値へ変換したかどうかだけでは判断しません。たとえば YAML 1.1 の解釈により `yes` が真偽値となるパーサーであっても、reco は受け付けません。次の例は、必要な親要素とほかの設定が適正に存在する場合の個々の値の判定です。

| 部分例 | 判定 |
|---|---|
| `allow_auto_merge: true` | 有効 |
| `allow_auto_merge: "true"` | エラー（文字列） |
| `allow_auto_merge: TRUE` | エラー（受理する表記ではない） |
| `allow_auto_merge: !!bool "true"` | エラー（明示タグ） |
| `allowed_actions: "selected"` | 値の形式は有効。ただし `selected_actions` の3項目も必要 |

### 6.2 リポジトリ名と重複

`name` は、`OWNER` と `REPO` を一つの `/` で区切った `OWNER/REPO` 形式の文字列です。`OWNER` と `REPO` はともに空ではなく、ASCII の英数字と `-`、`_`、`.` だけからなり、`.` と `..` ではありません。このため、空白、制御文字、ASCII 以外の文字を含む名前は受け付けません。URL や、末尾に `.git` を付けたクローン用の表記（`.GIT` のように大文字小文字が異なるものを含む）は受け付けません。この値は名前による **リポジトリ指定** であり、GitHub のリポジトリ ID や、実体の存在確認を表すものではありません。

GitHub は所有者名・リポジトリ名を大文字小文字非依存で扱います。このため、reco はリポジトリ指定どうしを、所有者名とリポジトリ名の大文字小文字を区別せずに比較し、一致するものを同一とみなします。同一の設定宣言に同一とみなされる `name` が複数ある場合は、同一文字列か **大文字小文字だけが異なるか** を問わず入力時に拒否します。たとえば `Example/Project-A` と `example/project-a` を同一の設定宣言に並べることはできません。

表記が異なる名前がリネーム・移転などによって GitHub 上の同一リポジトリ ID を指すかどうかは、YAML の検証では判定しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Internal Document |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Internal Document |

## 7. References

- [要求定義](requirements.md)
- [CLI 仕様](cli.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
- [YAML 1.2.2 Specification](https://yaml.org/spec/1.2.2/)
