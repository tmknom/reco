# Configuration

reco は、リポジトリごとの設定値を書いた YAML の設定ファイルを読み、GitHub 上の設定と比べて、書いた値になるように設定を変えます。

## 設定ファイルの場所と構造

reco は、既定ではカレントディレクトリの `reco.yaml` を読みます。別の場所のファイルを使うときは、`reco plan --config path/to/file.yaml` のように指定します。

設定ファイルのルートには `repositories` だけを置き、その下にリポジトリごとの設定を並べます。リポジトリごとに、次のキーを書きます。

| キー | 書くもの |
|---|---|
| `name` | 対象のリポジトリの名前。`OWNER/REPO` の形で書く（必須） |
| `general` | [リポジトリの一般設定](#リポジトリの一般設定) |
| `actions` | [GitHub Actions の設定](#github-actions-の設定) |
| `environments` | [Environment](#environment) |
| `security` | [セキュリティの設定](#セキュリティの設定) |

次は、一つのリポジトリでマージ後のブランチの自動削除だけを管理する、最小の例です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

reco が扱える主な設定を書くと、次のようになります。

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
          patterns_allowed:
            - "actions/checkout@v7"
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
    security:
      dependabot_alerts: true
```

## リポジトリの一般設定

`general` には、Pull Request のマージに関する設定を書きます。

| キー | 値 | 意味 |
|---|---|---|
| `delete_branch_on_merge` | `true` / `false` | Pull Request をマージしたあと、head ブランチを自動で削除するか |
| `allow_auto_merge` | `true` / `false` | Pull Request の自動マージを許可するか |
| `allow_squash_merge` | `true` / `false` | Pull Request を squash merge でマージできるようにするか |
| `allow_merge_commit` | `true` / `false` | Pull Request を merge commit でマージできるようにするか |
| `allow_rebase_merge` | `true` / `false` | Pull Request を rebase merge でマージできるようにするか |
| `allow_update_branch` | `true` / `false` | base ブランチより遅れた Pull Request に、ブランチの更新を常に提案するか |

> [!NOTE]
> GitHub の仕様上、GitHub Free のアカウントが所有するプライベートリポジトリでは、自動マージは使えません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` の3つは、Pull Request をマージする方法を許可する設定で、まとめて **マージ方式** と呼びます。GitHub は3つをすべて無効にすることを受け付けないため、3つとも `false` と書いた設定ファイルはエラーになります。一部を省き、書いたものをすべて `false` にした場合も、書いていないマージ方式が GitHub 上ですべて無効なら、`plan` が `Plan cannot be applied.` を表示します。reco は、書いていないマージ方式を代わりに有効にしません。有効にしておくマージ方式を `true` と書き足します。

`allow_update_branch` を `true` にしても、reco も GitHub もブランチを自動では更新しません。

> [!NOTE]
> 3つのマージ方式と `allow_update_branch` は、まだ `apply` で変えられません。変更が含まれると、`apply` は承認を求めずにエラーで終わります。`plan` と `show` では確かめられます。

## GitHub Actions の設定

`actions` には、`permissions`（Actions の実行許可）、`workflow`（ワークフローの権限）、`access`（ほかのリポジトリのワークフローからのアクセス）、`variables`（Repository Variables）を書きます。使わないものは省けます。

### Actions の実行許可

#### `actions.permissions`

`actions.permissions` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `enabled` | `true` / `false` | このリポジトリで GitHub Actions を有効にするか |
| `allowed_actions` | `all` / `local_only` / `selected` | 実行を許可する Actions と再利用可能なワークフローの範囲（Actions 許可ポリシー） |
| `selected_actions` | 下の `selected_actions` の表のキー | `allowed_actions: selected` のときに許可するもの |
| `sha_pinning_required` | `true` / `false` | Actions を完全な長さのコミット SHA に固定することを必須にするか |

#### `actions.permissions.allowed_actions`

`allowed_actions` の値の意味は次のとおりです。

| 値 | 許可するもの |
|---|---|
| `all` | すべての Actions と再利用可能なワークフロー |
| `local_only` | ローカルの Actions と再利用可能なワークフローだけ |
| `selected` | `selected_actions` に書いた条件に合うものだけ |

#### `actions.permissions.selected_actions`

`selected_actions` には、次のキーをすべて書きます。

| キー | 値 | 意味 |
|---|---|---|
| `github_owned_allowed` | `true` / `false` | GitHub が作った Actions を許可するか |
| `verified_allowed` | `true` / `false` | Marketplace で認証済みの作成者が提供する Actions を許可するか |
| `patterns_allowed` | 文字列のリスト | 許可する Actions と再利用可能なワークフローのパターン。何も許可しないときは `[]` と書く |

#### GitHub API の仕様

設定可能な値の組み合わせは、GitHub API の仕様の影響を受けます。

- `enabled: false` を書くときは、`allowed_actions`、`selected_actions`、`sha_pinning_required` のどれも書けない
- `selected_actions` は、`allowed_actions: selected` のときだけ書ける
- `enabled: false` で Actions を無効にしても、GitHub は `allowed_actions`、`selected_actions`、`sha_pinning_required` の値を残す。あとで `enabled: true` に戻すと、無効にする前の設定で有効になる。再開するときの Actions 許可ポリシーを自分で決めたいときは、`enabled: true` の代わりに `allowed_actions` を書く。`allowed_actions` を書いても、`sha_pinning_required` は無効にする前の値で再開する。変えたいときは `sha_pinning_required` も書く
- `allowed_actions` を書くと、Actions も有効になる
- `allowed_actions` を `selected` から `all` や `local_only` に変えると、GitHub は `selected_actions` の値を消す
- Actions が無効のリポジトリで `sha_pinning_required` を書くときは、`enabled: true` か `allowed_actions` も書く。Actions が無効のあいだに送った値を GitHub は保存しないため、どちらも書かないと `apply` できない

### ワークフローの権限

`actions.workflow` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `default_workflow_permissions` | `read` / `write` | ワークフローの `GITHUB_TOKEN` に与えるデフォルト権限 |
| `can_approve_pull_request_reviews` | `true` / `false` | GitHub Actions が Pull Request を作成し、承認することを許可するか |

### ほかのリポジトリのワークフローからのアクセス

`actions.access` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `access_level` | `none` / `user` | ほかのリポジトリのワークフローが、このリポジトリの Actions と再利用可能なワークフローを使える範囲 |

`none` はほかのリポジトリからのアクセスを許可しません。`user` は、同じユーザーが所有するほかのプライベートリポジトリのワークフローからのアクセスを許可します。

- この設定はプライベートリポジトリにだけ適用される。パブリックリポジトリに書くと `apply` できない
- `organization` は設定ファイルに書けない。Organization が所有するリポジトリのための値で、reco が扱うユーザー所有のリポジトリでは GitHub が受け付けないため。Organization から移転したリポジトリなどで `show` が `organization` を示したときは、`none` か `user` を書けば、書いた値に変わる
- Actions が無効のリポジトリでも書ける

### Repository Variables

`actions.variables` には、ワークフローから `vars.<名前>` で読む Repository Variables を、名前ごとに並べます。

```yaml
repositories:
  - name: example/project-a
    actions:
      variables:
        - name: GO_VERSION
          value: "1.27"
        - name: DEPLOY_REGION
          value: ap-northeast-1
```

| キー | 値 | 意味 |
|---|---|---|
| `name` | 文字列（必須） | 変数の名前。半角の英大文字、数字、`_` で書く |
| `value` | 文字列（必須） | 変数の値 |

- 書いた名前の変数がなければ、`plan` は作成を示し、`apply` が作る。値が違えば、`plan` は書いた値への変更を示し、`apply` が値を変える
- 書かなかった変数は、変更も削除もしない。設定ファイルから行を消しても、GitHub 上の変数は残る
- GitHub が名前を大文字にして保存するため、reco は小文字を含む名前（`go_version`）を設定ファイルの誤りとして拒否し、`GO_VERSION` と書くよう示す。数字で始まる名前と、`GITHUB_` で始まる名前は書けない
- 数値、真偽値、日付に見える値は、引用符で囲む。`value: 1.27`、`value: true`、`value: 2026-11-22` は文字列ではないため拒否し、`"1.27"` のように引用符で囲んで書くよう示す
- 空の値（`value: ""`）は書けない。GitHub が空の値を受け付けないため
- 値は、書いたとおりの文字列で GitHub 上の値と比べる。前後の空白や末尾の改行だけが違っても、違う値になる。`value: |` で複数行の値を書くと、末尾に改行が付く
- 値は `show` の表示と、`plan` と `apply` が示す変更計画にそのまま出る。トークンやパスワードのような秘密情報は、Variables ではなく Secrets に入れる

## Environment

`environments` には、デプロイ先として使う Environment を、名前ごとに並べます。

```yaml
repositories:
  - name: example/project-a
    environments:
      - name: production
        deployment_branch_policy:
          mode: protected_branches
      - name: staging
        deployment_branch_policy:
          mode: selected
          patterns:
            - type: branch
              name: "main"
            - type: tag
              name: "v*"
```

| キー | 値 | 意味 |
|---|---|---|
| `name` | 文字列（必須） | Environment の名前 |
| `deployment_branch_policy.mode` | `all` / `protected_branches` / `selected` | Environment へデプロイできるブランチとタグの制限。`all` は制限なし、`protected_branches` は保護ルールのあるブランチだけ、`selected` は `patterns` に書いたパターンに一致するブランチとタグだけ |
| `deployment_branch_policy.patterns` | 下の2つのキーを持つ要素の一覧（`mode: selected` のときだけ必須） | デプロイを許可するブランチとタグのパターンの一覧の全体 |
| `patterns[].type` | `branch` / `tag`（必須） | パターンがブランチとタグのどちらに一致するか |
| `patterns[].name` | 文字列（必須） | ブランチ名またはタグ名のパターン（`release/*` など） |

- 書いた名前の Environment がなければ、reco は作成する。`deployment_branch_policy` を省いた Environment は、あるかどうかだけを管理する
- 書かなかった Environment は、変更も削除もしない。設定ファイルから名前を消しても、GitHub 上の Environment は残る
- 名前は、半角の英数字、空白、記号だけで書く。大文字と小文字だけが違う名前は同じ Environment とみなすため、同じリポジトリに並べられない
- 保護ルールのあるブランチが一つもなければ、`protected_branches` でもすべてのブランチからデプロイできる（GitHub の仕様）。ブランチの保護ルールは reco では作れない
- `patterns` には、デプロイを許可するパターンをすべて、順不同で書く。一つも許可しないときは `patterns: []` と書く。`all` と `protected_branches` では `patterns` を書けない
- `apply` は、GitHub 上にあって `patterns` にないパターンを取り除き、`patterns` にあって GitHub 上にないパターンを加える
- `type` が違えば、名前が同じでも別のパターンになる。大文字と小文字だけが違う名前（`main` と `MAIN`）も別のパターンになる
- `*` で始まる名前は、引用符で囲まないと設定ファイルを読めない。パターンの名前は引用符で囲んで書くとよい
- `selected` の Environment を `all` などに変えると、GitHub は並んでいたパターンを残さない

### Environment Variables

`environments` の各 Environment の `variables` には、その Environment を使うジョブから `vars.<名前>` で読む Environment Variables を、名前ごとに並べます。

```yaml
repositories:
  - name: example/project-a
    environments:
      - name: production
        variables:
          - name: REGION
            value: ap-northeast-1
      - name: staging
        variables:
          - name: REGION
            value: us-west-2
```

`name` と `value` の書き方と、書かなかった変数を変えないことは、[Repository Variables](#repository-variables) と同じです。

- `variables` だけを書いた Environment も、GitHub 上になければ reco が作成し、そのあとに変数を作る
- 同じ Environment の `variables` には、同じ名前を二度書けない。別の Environment と `actions.variables` には、同じ名前を書ける
- `actions.variables` と同じ名前の変数を書いても、別の変数として扱う。片方の値に合わせてもう一方を変えることはない

## セキュリティの設定

`security` には、Dependabot、Secret scanning、Private vulnerability reporting の設定を書きます。

```yaml
repositories:
  - name: example/project-a
    security:
      dependabot_alerts: true
      dependabot_security_updates: true
      secret_scanning: true
      secret_scanning_push_protection: true
      private_vulnerability_reporting: true
```

| キー | 値 | 意味 |
|---|---|---|
| `dependabot_alerts` | `true` / `false` | 依存関係の脆弱性を Dependabot alerts で知らせるか |
| `dependabot_security_updates` | `true` / `false` | 脆弱性を直す Pull Request を Dependabot security updates に作らせるか |
| `secret_scanning` | `true` / `false` | リポジトリに push されたシークレットを Secret scanning で検出するか |
| `secret_scanning_push_protection` | `true` / `false` | シークレットを含む push を、リポジトリの push protection で拒否するか。利用者が自分のアカウントに設定する push protection とは別の設定 |
| `private_vulnerability_reporting` | `true` / `false` | 見つけた脆弱性を、公開せずにメンテナーへ報告できるようにするか |

どのキーも、一つだけを書いてもよいです。Dependabot の設定で気をつけることは次のとおりです。

- `dependabot_alerts: false` と `dependabot_security_updates: true` は一緒に書けない。GitHub は、Dependabot alerts が無効のリポジトリで Dependabot security updates を有効にできないため
- Dependabot alerts が無効のリポジトリで `dependabot_security_updates` だけを書いて値を変えようとすると、`plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る。`dependabot_alerts: true` も書く
- `dependabot_alerts: false` にすると、GitHub は Dependency graph と、有効だった Dependabot security updates も無効にする。`plan` は、この変化を `Side effect:` の行で示す
- あとで `dependabot_alerts: true` に戻すと、Dependency graph は有効に戻るが、Dependabot security updates は戻らない。戻すには `dependabot_security_updates: true` も書く
- Dependabot alerts を無効にしたまま Dependency graph だけを有効にすることは、reco ではできない。GitHub の設定画面で行う
- Admin ロールが要る（[Authentication](authentication.md)）
- GitHub が Dependabot security updates を一時停止していても、設定は有効のままで、`dependabot_security_updates: true` と一致する。reco は一時停止を解除しない

Secret scanning の設定で気をつけることは次のとおりです。

- 非公開リポジトリには書けない。書くと `plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る。公開リポジトリにだけ書く
- `secret_scanning: false` と `secret_scanning_push_protection: true` は一緒に書けない。GitHub は、Secret scanning が無効のリポジトリでリポジトリの push protection を有効にできないため
- Secret scanning が無効のリポジトリで `secret_scanning_push_protection: true` だけを書くと、`plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る。`secret_scanning: true` も書く
- `secret_scanning: false` にすると、GitHub は有効だったリポジトリの push protection も無効にする。`plan` は、この変化を `Side effect:` の行で示す
- あとで `secret_scanning: true` に戻しても、リポジトリの push protection は戻らない。戻すには `secret_scanning_push_protection: true` も書く
- Admin ロールが要る（[Authentication](authentication.md)）

Private vulnerability reporting の設定で気をつけることは次のとおりです。

- 非公開リポジトリには書けない。書くと `plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る。公開リポジトリにだけ書く
- 変えるには Admin ロールが要る（[Authentication](authentication.md)）

## 未記載の設定の扱い

reco が変えるのは、設定ファイルに値を書いた設定だけです。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: false
```

この例で reco が操作するのは `allow_auto_merge` だけです。未記載の `delete_branch_on_merge` の設定は変更しません。

> [!NOTE]
> **設定ファイルから設定を消しても、GitHub 上の値は元に戻りません**。値を変えたいときは、消すのではなく、設定したい値を書いて `apply` します。

## 複数のリポジトリを並べる

複数のリポジトリを管理するときは、`repositories` の下にリポジトリごとの設定を並べます。リポジトリごとに、管理する設定を別々に選べます。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
      allow_auto_merge: true

  - name: example/project-b
    general:
      delete_branch_on_merge: false
```

なお、reco では YAML のアンカー（`&`）、エイリアス（`*`）、マージキー（`<<`）はサポートしていません。

## よくある書き方の誤り

設定ファイルに誤りがあると、reco は GitHub にアクセスする前に、誤りの箇所（行と列）と理由を示して終了します。出会いやすい誤りは次のとおりです。

### 真偽値

真偽値を引用符で囲むと、文字列として扱われ、拒否されます。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: "true" # 引用符なしで true と書くべき
```

### 必須キーの欠如

たとえば `allowed_actions: selected` のときに `selected_actions` のキーの一部を書き忘れると、足りないキーが示されます。

```yaml
repositories:
  - name: example/project-a
    actions:
      permissions:
        allowed_actions: selected
        selected_actions:
          github_owned_allowed: true
          verified_allowed: true
```
