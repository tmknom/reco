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
| `rulesets` | [Ruleset](#ruleset) |

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
      workflow_permissions:
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

`general` には、Pull Request のマージに関する設定と、Issues・Projects・Wiki を使うか、Web 画面からのコミットにサインオフを求めるかを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `delete_branch_on_merge` | `true` / `false` | Pull Request をマージしたあと、head ブランチを自動で削除するか |
| `allow_auto_merge` | `true` / `false` | Pull Request の自動マージを許可するか |
| `allow_squash_merge` | `true` / `false` | Pull Request を squash merge でマージできるようにするか |
| `allow_merge_commit` | `true` / `false` | Pull Request を merge commit でマージできるようにするか |
| `allow_rebase_merge` | `true` / `false` | Pull Request を rebase merge でマージできるようにするか |
| `allow_update_branch` | `true` / `false` | base ブランチより遅れた Pull Request に、ブランチの更新を常に提案するか |
| `has_issues` | `true` / `false` | Issues を使えるようにするか |
| `has_projects` | `true` / `false` | Projects を使えるようにするか |
| `has_wiki` | `true` / `false` | Wiki を使えるようにするか |
| `web_commit_signoff_required` | `true` / `false` | GitHub の Web 画面からのコミットに、作成者のサインオフを求めるか |

> [!NOTE]
> GitHub の仕様上、GitHub Free のアカウントが所有する非公開リポジトリでは、自動マージは使えません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` の3つは、Pull Request をマージする方法を許可する設定で、まとめて **マージ方式** と呼びます。GitHub は3つをすべて無効にすることを受け付けないため、3つとも `false` と書いた設定ファイルはエラーになります。一部を省き、書いたものをすべて `false` にした場合も、書いていないマージ方式が GitHub 上ですべて無効なら、`plan` が `Plan cannot be applied.` を表示します。reco は、書いていないマージ方式を代わりに有効にしません。有効にしておくマージ方式を `true` と書き足します。

`allow_update_branch` を `true` にしても、reco も GitHub もブランチを自動では更新しません。

### マージ時のコミットメッセージ

`squash_merge_commit` と `merge_commit` には、squash merge と merge commit でマージするときに GitHub が最初に入れておくコミットメッセージの形を、タイトル（`title`）と本文（`message`）の組で書きます。`title` と `message` はどちらも必須で、片方だけは書けません。rebase merge にはこの設定はありません。

```yaml
    general:
      squash_merge_commit:
        title: PR_TITLE
        message: PR_BODY
      merge_commit:
        title: MERGE_MESSAGE
        message: PR_TITLE
```

値の意味は次のとおりです。

| 値 | 意味 |
|---|---|
| `PR_TITLE` | Pull Request のタイトル |
| `COMMIT_OR_PR_TITLE` | コミットが1つならそのコミットのタイトル、2つ以上なら Pull Request のタイトル（squash merge のタイトルだけ） |
| `MERGE_MESSAGE` | `Merge pull request #123 from branch-name` のような従来のタイトル（merge commit のタイトルだけ） |
| `PR_BODY` | Pull Request の本文 |
| `COMMIT_MESSAGES` | ブランチのコミットメッセージ（squash merge の本文だけ） |
| `BLANK` | 空の本文 |

GitHub が受け付けるタイトルと本文の組み合わせは次の7つだけで、ほかの組み合わせを書いた設定ファイルはエラーになります。

| キー | `title` | `message` |
|---|---|---|
| `squash_merge_commit` | `PR_TITLE` | `PR_BODY`、`BLANK`、`COMMIT_MESSAGES` |
| `squash_merge_commit` | `COMMIT_OR_PR_TITLE` | `COMMIT_MESSAGES` |
| `merge_commit` | `PR_TITLE` | `PR_BODY`、`BLANK` |
| `merge_commit` | `MERGE_MESSAGE` | `PR_TITLE` |

GitHub は、squash merge が無効のあいだ `squash_merge_commit` を、merge commit が無効のあいだ `merge_commit` を変えさせません。そのマージ方式が GitHub 上で無効なリポジトリで値を変えようとすると、`plan` が `Plan cannot be applied.` を表示します。reco は、マージ方式を代わりに有効にしません。`allow_squash_merge: true`（または `allow_merge_commit: true`）を書き足すか、`squash_merge_commit`（または `merge_commit`）を外します。同じ設定ファイルでマージ方式を有効にする場合は、reco がマージ方式を有効にしてからコミットメッセージを変えます。

### Issues・Projects・Wiki とコミットのサインオフ

`has_issues`、`has_projects`、`has_wiki` を `false` にすると、その機能が無効になり、既存の Issue、Project、Wiki のページを GitHub の画面から使えなくなります。reco は Issue、Project、Wiki のページを作りも消しもしません。Issues は、`true` に戻すと、無効にする前の Issue をまた使えます。Wiki も、GitHub の公式資料によると、ページは消えず、`true` に戻すと元のページが戻ります。Projects を無効にしたときに既存の Project がどうなるかは、GitHub の公式資料に記載がありません。

> [!NOTE]
> GitHub の仕様上、GitHub Free のアカウントが所有する非公開リポジトリでは、Wiki は使えません。

`web_commit_signoff_required` は、コミットの署名を求める設定（Ruleset の署名付きコミットを求めるルールなど）とは別の設定で、`true` にしてもコミットの署名は求められません。

## GitHub Actions の設定

`actions` には、`permissions`（Actions の実行許可）、`workflow_permissions`（ワークフローの権限）、`access`（ほかのリポジトリのワークフローからのアクセス）、`fork_pr_contributor_approval`（Fork からの Pull Request のワークフローの承認）、`fork_pr_workflows_private_repos`（非公開リポジトリでの、Fork からの Pull Request のワークフローの実行条件）、`artifact_and_log_retention`（Artifact や Log などの保持期間）、`oidc_subject`（OIDC トークンの subject の形）、`variables`（Repository Variables）を書きます。使わないものは省けます。

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

`actions.workflow_permissions` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `default_workflow_permissions` | `read` / `write` | ワークフローの `GITHUB_TOKEN` に与えるデフォルト権限 |
| `can_approve_pull_request_reviews` | `true` / `false` | GitHub Actions が Pull Request を作成し、承認することを許可するか |

### ほかのリポジトリのワークフローからのアクセス

`actions.access` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `access_level` | `none` / `user` | ほかのリポジトリのワークフローが、このリポジトリの Actions と再利用可能なワークフローを使える範囲 |

`none` はほかのリポジトリからのアクセスを許可しません。`user` は、同じユーザーが所有するほかの非公開リポジトリのワークフローからのアクセスを許可します。

- この設定は非公開リポジトリにだけ適用される。公開リポジトリに書くと `apply` できない
- `organization` は設定ファイルに書けない。Organization が所有するリポジトリのための値で、reco が扱うユーザー所有のリポジトリでは GitHub が受け付けないため。Organization から移転したリポジトリなどで `show` が `organization` を示したときは、`none` か `user` を書けば、書いた値に変わる
- Actions が無効のリポジトリでも書ける

### Fork からの Pull Request のワークフロー

公開リポジトリには `actions.fork_pr_contributor_approval` を、非公開リポジトリには `actions.fork_pr_workflows_private_repos` を書きます。リポジトリの公開範囲に合わない側を書くと `apply` できません。合わない側を消してください。

#### 公開リポジトリの承認

`actions.fork_pr_contributor_approval` には、次のキーを書きます。

```yaml
repositories:
  - name: example/project-a
    actions:
      fork_pr_contributor_approval:
        approval_policy: first_time_contributors
```

| キー | 値 | 意味 |
|---|---|---|
| `approval_policy` | `first_time_contributors_new_to_github` / `first_time_contributors` / `all_external_contributors` | Fork からの Pull Request のワークフローの実行に承認が要るユーザーの範囲。承認できるのは、リポジトリへの書き込み権限を持つユーザー |

`approval_policy` の値の意味は次のとおりです。

| 値 | 承認が要るユーザー |
|---|---|
| `first_time_contributors_new_to_github` | このリポジトリにコミットか Pull Request がマージされたことがなく、GitHub に登録して間もないユーザー |
| `first_time_contributors` | このリポジトリにコミットか Pull Request がマージされたことのないユーザー |
| `all_external_contributors` | リポジトリのメンバーか所有者でも、Organization のメンバーでもない、すべてのユーザー |

- この設定は公開リポジトリにだけ適用される。非公開リポジトリに書くと `apply` できない。そのときは `fork_pr_contributor_approval` ごと消す
- 値は小文字で書く。`FIRST_TIME_CONTRIBUTORS` のような大文字の値は、GitHub が受け付けないため書けない
- Actions が無効のリポジトリでも書ける

#### 非公開リポジトリの実行条件

`actions.fork_pr_workflows_private_repos` には、次の4つのキーをすべて書きます。

```yaml
repositories:
  - name: example/private-project
    actions:
      fork_pr_workflows_private_repos:
        run_workflows_from_fork_pull_requests: true
        send_write_tokens_to_workflows: false
        send_secrets_and_variables: false
        require_approval_for_fork_pr_workflows: true
```

| キー | 値 | 意味 |
|---|---|---|
| `run_workflows_from_fork_pull_requests` | `true` / `false` | Fork からの Pull Request で起動したワークフローを実行できるか。`true` だけなら、ワークフローは読み取り専用の `GITHUB_TOKEN` で、Secrets を使わずに動く |
| `send_write_tokens_to_workflows` | `true` / `false` | そのワークフローに、書き込み権限を持つ `GITHUB_TOKEN` を渡すか |
| `send_secrets_and_variables` | `true` / `false` | そのワークフローで、Secrets と Variables を使えるようにするか |
| `require_approval_for_fork_pr_workflows` | `true` / `false` | そのワークフローの実行に、書き込み権限を持つユーザーの承認を求めるか |

- 4つは一組の設定で、一部だけは書けない。足りないキーはエラーで示される
- `run_workflows_from_fork_pull_requests` が `false` なら、ほかの3つも `false` と書く。GitHub は実行を許可しないあいだ、ほかの3つを `false` にするため、`true` を書くとエラーになる
- 書き込み権限を持つ `GITHUB_TOKEN` か Secrets と Variables が、Fork からの Pull Request で起動したワークフローに渡るようになる変更には、`plan` と `apply` の変更計画に `Note:` の行が付く（[コマンドリファレンス](commands.md#plan)）
- この設定は非公開リポジトリにだけ適用される。公開リポジトリに書くと `apply` できない。そのときは `fork_pr_workflows_private_repos` ごと消す
- Actions が無効のリポジトリでも書ける

### 保持期間

`actions.artifact_and_log_retention` には、次のキーを書きます。

```yaml
repositories:
  - name: example/project-a
    actions:
      artifact_and_log_retention:
        days: 30
```

| キー | 値 | 意味 |
|---|---|---|
| `days` | `1` 以上の整数 | チェック、ワークフローの実行、コミットのステータス、Artifact、Log を保持する日数 |

- 設定できる日数には、リポジトリごとに上限がある。個人アカウントが所有するリポジトリでは、公開リポジトリで90日、非公開リポジトリで400日が目安で、実際の上限は `show` の `days` の行末に `# maximum allowed: <日数>` と示される
- 今の値と違い、かつ上限を超える日数を書くと、`plan` に `Applicability: not applicable` と、書いた日数と上限が出て、`apply` できない。reco は、書いた日数を上限の日数に切り詰めて変えることはしない
- `apply` の承認のあとに上限が下がり、書いた日数が上限を超えた場合は、その変更は `failed` になる
- 日数を短くしても、新しい日数が適用されるのは、変更のあとに作られる Artifact や Log などだけである
- `"30"` のように引用符で囲んだ値や、`30.5` のような小数は書けない
- Actions が無効のリポジトリでも書ける

### OIDC トークンの subject

ワークフローが外部のクラウドなどへ認証するときに示す OIDC トークンは、subject（`sub`）にリポジトリやブランチなどを並べた文字列を持ちます。受け取る側は、この文字列を信頼の条件と照らし合わせます。`actions.oidc_subject` には、subject の形を決める次のキーを書きます。

```yaml
repositories:
  - name: example/project-a
    actions:
      oidc_subject:
        use_default: false
        include_claim_keys:
          - repo
          - context
        use_immutable_subject: true
```

| キー | 値 | 意味 |
|---|---|---|
| `use_default` | `true` / `false` | GitHub の既定の形の subject を使うか。必ず書く |
| `include_claim_keys` | key の並び | subject を組み立てるクレームの key。`use_default: false` のときだけ、1つ以上書く |
| `use_immutable_subject` | `true` / `false` | subject のリポジトリの部分に、所有者とリポジトリの ID を含める形を使うか。書かなければ、reco はこの値を変えない |

- `use_default: true` のときは `include_claim_keys` を書けない。GitHub は、既定の形を使うあいだ key を保存しないため
- `use_default: false` のときは、key を1つ以上書く。key のない状態は、発行される subject の形が分からないため書けない
- key は書いた順が意味を持つ。`repo` と `context` を入れ替えると別の設定になる
- key は英数字と `_` だけからなる文字列で、同じ key は二度書けない。reco は使える key の一覧を持たないため、GitHub が受け付けない key（大文字を含む `Repo` など）は `plan` では分からず、`apply` で `failed` になる（[トラブルシューティング](troubleshooting.md)）。そのとき GitHub 上の値は変わらない
- key を持つ状態から `use_default: true` に変えると、GitHub は保存していた key を消す。あとで `use_default: false` に戻しても、key は戻らない
- 2026年7月15日より後に名前を変えたか移転したリポジトリなどで、`use_immutable_subject` が `true` のとき、`false` を書いても GitHub が値を変えないことがある。そのとき `apply` は `failed` になる（[トラブルシューティング](troubleshooting.md#oidc-トークンの-subject-の-use_immutable_subject-が-false-にならない)）
- subject の形を変える変更には、`plan` と `apply` の変更計画に `Note:` の行が付く（[コマンドリファレンス](commands.md#plan)）
- subject の形を変えると、今の subject に合わせて設定した外部のクラウドなどの信頼の条件と一致しなくなり、ワークフローが認証できなくなることがある。新しい形でも認証できるよう、先に外部の信頼の条件を直してから `apply` する（[Safety](safety.md)）
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
| `value` | 文字列 | 変数の値。`state` を書かない要素では必須 |
| `state` | `absent` | その名前の変数がないこと。`value` と同時には書けない |

- 書いた名前の変数がなければ、`plan` は作成を示し、`apply` が作る。値が違えば、`plan` は書いた値への変更を示し、`apply` が値を変える
- 書かなかった変数は、変更も削除もしない。設定ファイルから行を消しても、GitHub 上の変数は残る
- 変数を消すには、行を消すのではなく、`value` の代わりに `state: absent` を書く。変数があれば、`plan` は削除と消える値を `"<今の値>" -> absent` と示し、`apply` が消す。変数がなければ、変更はない。消えたあとは、その行を設定ファイルから消してよい
- `value` を書き忘れた要素や `value: ~` は、削除の宣言にならず、設定ファイルの誤りとして拒否する。`state` に書けるのは `absent` だけである

```yaml
      variables:
        - name: LEGACY_FLAG
          state: absent
```

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

- 書いた名前の Environment がなければ、reco は作成する。`deployment_branch_policy`、`reviewers`、`prevent_self_review`、`wait_timer`、`variables` のどれも省いた Environment は、あるかどうかだけを管理する
- 書かなかった Environment は、変更も削除もしない。設定ファイルから名前を消しても、GitHub 上の Environment は残る
- 名前は、半角の英数字、空白、記号だけで書く。大文字と小文字だけが違う名前は同じ Environment とみなすため、同じリポジトリに並べられない
- 保護ルールのあるブランチが一つもなければ、`protected_branches` でもすべてのブランチからデプロイできる（GitHub の仕様）。ブランチの保護ルールは reco では作れない
- `patterns` には、デプロイを許可するパターンをすべて、順不同で書く。一つも許可しないときは `patterns: []` と書く。`all` と `protected_branches` では `patterns` を書けない
- `apply` は、GitHub 上にあって `patterns` にないパターンを取り除き、`patterns` にあって GitHub 上にないパターンを加える
- `type` が違えば、名前が同じでも別のパターンになる。大文字と小文字だけが違う名前（`main` と `MAIN`）も別のパターンになる
- `*` で始まる名前は、引用符で囲まないと設定ファイルを読めない。パターンの名前は引用符で囲んで書くとよい
- `selected` の Environment を `all` などに変えると、GitHub は並んでいたパターンを残さない

### デプロイを承認できる担当者

`environments` の各 Environment の `reviewers` には、その Environment を使うデプロイジョブの実行を承認できる担当者（GitHub の画面の「Required reviewers」）を並べます。

```yaml
repositories:
  - name: example/project-a
    environments:
      - name: production
        reviewers:
          - type: User
            id: 583231
          - type: User
            id: 1234567
```

| キー | 値 | 意味 |
|---|---|---|
| `reviewers` | 下の2つのキーを持つ要素の一覧（6件まで） | デプロイを承認できる担当者の一覧の全体 |
| `reviewers[].type` | `User`（必須） | 担当者の種類 |
| `reviewers[].id` | 正の整数（必須） | 担当者の数値の ID |

- 担当者のうち1人が承認すれば、デプロイジョブは進む
- `id` は利用者の login ではなく、数値の ID を書く。`reco show` が、いまの担当者を `id` と、行末のコメントの login で示すので、そこから書き写せる。まだ担当者でない利用者の ID は、GitHub CLI の `gh api users/<login> --jq .id` で調べられる
- 担当者をすべて、順不同で書く。GitHub 上にいて書かなかった担当者は、外す差分として `plan` に出る
- 担当者がいないことは `reviewers: []` と書く。`reviewers` の行を消すと、担当者を管理しなくなる
- 担当者をすべて外すと、GitHub は画面の「Prevent self-review」（下の `prevent_self_review`）も一緒に無効にする。担当者を外す前に「Prevent self-review」が有効で、設定ファイルに `prevent_self_review` を書いていなければ、`plan` の `Side effect:` の行に、`prevent_self_review` が失われることが出る。あとで担当者を設定し直しても、「Prevent self-review」は無効のままになる
- GitHub 上にない Environment に担当者を書くと、reco は Environment を作ってから担当者を設定する。担当者の設定が失敗すると、担当者のいない Environment が残る
- `type: Team` は書けない。個人アカウントが所有するリポジトリでは GitHub が Team を担当者として受け付けず、Organization が所有するリポジトリでの動作を reco は確かめていない
- この設定が止めるのは、その Environment を使うデプロイジョブであり、Ruleset の `pull_request` のような Pull Request のレビューとは別の設定である

### 自己承認の禁止とデプロイの待機時間

`environments` の各 Environment には、`prevent_self_review`（GitHub の画面の「Prevent self-review」）と `wait_timer`（画面の「Wait timer」）を書けます。

```yaml
repositories:
  - name: example/project-a
    environments:
      - name: production
        reviewers:
          - type: User
            id: 583231
          - type: User
            id: 1234567
        prevent_self_review: true
        wait_timer: 30
```

| キー | 値 | 意味 |
|---|---|---|
| `prevent_self_review` | `true` / `false` | デプロイジョブを起動した利用者が、担当者であっても自分のデプロイジョブを承認できないようにするか |
| `wait_timer` | `0` から `43199` までの整数 | デプロイジョブが起動されてから実行されるまで待たせる時間（分）。`0` は待たせないこと |

- 担当者が1人だけのとき、`prevent_self_review: true` にすると、その人が起動したデプロイジョブを承認できる担当者がいなくなる
- GitHub が `prevent_self_review` の変更を受け付けるのは、担当者がいる Environment だけである。`reviewers: []` と `prevent_self_review: true` は一緒に書けない。`reviewers` を書かずに `prevent_self_review` を書き、GitHub 上に担当者がいないまま値を変える必要があるとき（承認の設定のない Environment に `true` を書いた場合や、担当者が1人もいない承認の設定に残る `true` を `false` にする場合）は、`plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る
- `reviewers: []` と `prevent_self_review: false` を一緒に書くと、担当者を外す変更だけで「Prevent self-review」も無効になる。`plan` には `prevent_self_review` の変更として出て、`Side effect:` の行は出ない
- 承認の設定（画面の「Required reviewers」）がない Environment の `prevent_self_review` は `false`、待機時間のない Environment の `wait_timer` は `0` と比べる。担当者が1人もいない承認の設定が GitHub 上に残っているとき（`reco show` が `# reviewers: no reviewers ...` と示す）は、その設定が持つ `prevent_self_review` の値と比べる
- 非公開リポジトリでは、料金プランによって待機時間を設定できない（公式資料は GitHub Free、Pro、Team を挙げる）。GitHub Free の非公開リポジトリでは、`wait_timer` を変える `apply` は、`0` にする変更も含めて `422` で失敗する

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

`name`、`value`、`state: absent` の書き方と、書かなかった変数を変えないことは、[Repository Variables](#repository-variables) と同じです。

- `variables` だけを書いた Environment も、GitHub 上になければ reco が作成し、そのあとに変数を作る
- GitHub 上にない Environment の中に `state: absent` の変数を書くと、reco は Environment を作成し、作成のあとにその変数がないことを確かめる。作られたばかりの Environment には変数がないため、`plan` は `(environment absent) -> absent` と示し、削除はしない
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

## Ruleset

`rulesets` には、ブランチへの操作を制限する Ruleset を、名前ごとに並べます。書いた名前の Ruleset が GitHub 上になければ、`apply` が作成します。あって内容が違えば、`apply` が、書いた内容で Ruleset の全体を置き換えます。`state: absent` を書いた名前の Ruleset は、`apply` が消します。

```yaml
repositories:
  - name: example/project-a
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
              required_approving_review_count: 1
              dismiss_stale_reviews_on_push: true
              require_code_owner_review: false
              require_last_push_approval: false
              required_review_thread_resolution: true
              allowed_merge_methods: [squash, rebase]
              require_extra_approval_for_unattributed_changes: true
          - type: deletion
          - type: non_fast_forward
          - type: required_status_checks
            parameters:
              required_status_checks:
                - context: "test"
                - context: "lint"
                  integration_id: 15368
              strict_required_status_checks_policy: true
              do_not_enforce_on_create: false
```

| キー | 値 | 意味 |
|---|---|---|
| `name` | 文字列（必須） | Ruleset の名前 |
| `target` | `branch`（必須） | Ruleset を適用する対象。ブランチだけを扱う |
| `enforcement` | `active` / `disabled`（必須） | ルールを強制するか |
| `conditions.ref_name.include` | パターンの一覧（必須。`[]` 可） | 対象にするブランチ。`~DEFAULT_BRANCH`（デフォルトブランチ）、`~ALL`（すべてのブランチ）、`refs/heads/` で始まるパターン（`refs/heads/release/*` など） |
| `conditions.ref_name.exclude` | パターンの一覧（必須。`[]` 可） | 対象から外すブランチ |
| `bypass_actors` | 要素の一覧（必須。`[]` 可） | ルールを迂回できる主体。`actor_type`、`actor_id`、`bypass_mode` を書く |
| `rules` | 要素の一覧（必須。`[]` 可） | 課すルール。`pull_request`（Pull Request を経ることを必須にする）、`deletion`（ブランチの削除の制限）、`non_fast_forward`（Force push の制限）、`required_status_checks`（ステータスチェックの成功を必須にする）を書ける。同じ `type` は一つだけ |
| `state` | `absent` | その名前の Ruleset がないこと。`name` 以外のキーと同時に書けない |

**Ruleset は、書いた内容がその Ruleset の全体になります**。GitHub 上の Ruleset にあって設定ファイルにないルール、迂回できる主体、パターンは、適用するとなくなります。`plan` は、なくなるものを `Rules to remove:` や `Bypass actors to remove:` の行で示します。

`state` を書かない要素では、表のキーはどれも省略できません。書き忘れを既定値で補わないためです。ほかに気をつけることは次のとおりです。

- 書いた名前の Ruleset がなければ、`plan` は作成を示す。書かなかった Ruleset を、reco は変更も削除もしない。設定ファイルから要素を消しても、GitHub 上の Ruleset は残る
- Ruleset を消すには、要素を消すのではなく、`name` と `state: absent` だけを書く。Ruleset があれば、`plan` は `present -> absent` と、消える Ruleset の今の内容の全体を示し、`apply` が消す。Ruleset がなければ、変更はない。消えたあとは、その要素を設定ファイルから消してよい
- キーを書き忘れた要素や `rules: []` は削除の宣言にならない。書き忘れは設定ファイルの誤りとして拒否する

```yaml
    rulesets:
      - name: legacy
        state: absent
```

- 名前は、半角の英数字、空白、記号だけで書き、前後に空白を置かない。大文字と小文字だけが違う名前は同じ Ruleset とみなすため、同じリポジトリに並べられない
- `bypass_actors` の `actor_type` は `RepositoryRole`、`User`、`Integration`、`DeployKey` のどれか。`bypass_mode` は `always`、`pull_request`、`exempt` のどれかで、省略できない。`RepositoryRole` の `actor_id` は `2`、`4`、`5` のどれかで、ロール名ではなく数値で書く。どの数値がどのロールかは公開されていないため、`show` の表示から書き写す。`DeployKey` には `actor_id` を書かず、`bypass_mode` は `always` だけ
- `pull_request` と `required_status_checks` の `parameters` には、下の表で必須としたキーをすべて書く
- `deletion` と `non_fast_forward` には `parameters` を書かない
- 一覧の要素の順序は問わない。並びだけが違っても変更にならない
- 非公開リポジトリには書けない。書くと `plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る
- GitHub 上に、半角の英数字、空白、記号以外の文字（`é` など）を含む名前の Ruleset があると、そのリポジトリにまだない名前の Ruleset は作成できない。GitHub がその名前を書いた名前と同じとみなすかを reco が判断できないため、`plan` は `(existence unknown)` と `Applicability: not applicable` を示す。GitHub 上にすでにある Ruleset の名前を書いた場合は、内容を比べられる
- reco がまだ解釈しないルール（`creation` など）や `pull_request` の設定（空でない `required_reviewers` など）を持つ Ruleset の名前を書くと、`plan` に `Applicability: not applicable` と、reco の解釈しない部分の一覧が出る。そうした Ruleset は設定ファイルから外す。`state: absent` を書いた場合も同じで、消える内容の全体を示せないため、reco は消さない
- `é` などを含む名前の Ruleset があるリポジトリで、GitHub 上にない名前に `state: absent` を書くと、その名前の Ruleset がないことを reco が確かめられないため、`plan` は `(existence unknown) -> absent` と `Applicability: not applicable` を示す

`pull_request` の `parameters` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `required_approving_review_count` | `0` から `10` の整数（必須） | マージの前に要る承認の数 |
| `dismiss_stale_reviews_on_push` | `true` / `false`（必須） | 新しいコミットが push されたら、それまでの承認を取り消すか |
| `require_code_owner_review` | `true` / `false`（必須） | Code owner が指定されたファイルを変える Pull Request に、その Code owner の承認を求めるか |
| `require_last_push_approval` | `true` / `false`（必須） | 最後の push を、push した人以外が承認することを求めるか |
| `required_review_thread_resolution` | `true` / `false`（必須） | コードについての会話をすべて解決してからマージさせるか |
| `allowed_merge_methods` | `merge` / `squash` / `rebase` の一覧（必須。1件以上） | Pull Request をマージできる方式 |
| `require_extra_approval_for_unattributed_changes` | `true` / `false`（必須） | 人間の共同作業者のいない Copilot の Pull Request に、もう一つの承認を求めるか |

- `allowed_merge_methods` は、同じ方式を二度書けない。3つの方式をどれも許すときは `[merge, squash, rebase]` と書く
- `allowed_merge_methods` と `require_extra_approval_for_unattributed_changes` を省略できないのは、GitHub が、Ruleset のルールを書き換えるときに送らなかった値を、`[merge, squash, rebase]` と `true` に戻すため。省略を許すと、書いていない値が適用で変わってしまう
- **reco は、`allowed_merge_methods` をリポジトリの General のマージ方式（`allow_squash_merge` など）と突き合わせません**。General で無効にした方式だけを許す Ruleset も、GitHub は受け付け、reco も止めず、`plan` にも示さない。ただし、GitHub の公式資料によると、その状態では対象のブランチへの Pull Request をマージできない場合がある。設定ファイルの `general` のマージ方式と一緒に見直す

`required_status_checks` の `parameters` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `required_status_checks` | チェックの一覧（必須。`[]` 可） | 成功を必須にするステータスチェック。チェックごとに `context` と、必要なら `integration_id` を書く |
| `required_status_checks[].context` | 文字列（必須） | ステータスチェックの名前 |
| `required_status_checks[].integration_id` | 正の整数（省略可） | そのチェックを送ってよい GitHub App の ID。省略すると、どの発行元のチェックでも受け付ける |
| `strict_required_status_checks_policy` | `true` / `false`（必須） | Pull Request のブランチを、対象のブランチの最新の内容で検証し直すことを求めるか |
| `do_not_enforce_on_create` | `true` / `false`（必須） | チェックが成功していなくても、対象のブランチの作成を許すか |

- `context` には、Pull Request のチェックの一覧に出る名前を、大文字と小文字、前後の空白も含めてそのまま書く。名前が違うと別のチェックになる
- **reco は、書いたチェックが実際にワークフローなどで作られるかを確かめません**。作られないチェックを必須にすると、対象のブランチへの Pull Request をマージできなくなる。チェックの名前は、そのブランチの Pull Request で一度実行されたものから書き写す
- `integration_id` には、リポジトリにインストールされた App の ID を書く。GitHub Actions は `15368`。インストールされていない App の ID は、`plan` では分からず、`apply` で GitHub が受け付けずに失敗する
- 同じ `context` と `integration_id` の組を二度書けない。同じ `context` でも、`integration_id` の有無や値が違えば別のチェックとして書ける
- `do_not_enforce_on_create` を省略できないのは、GitHub が、Ruleset のルールを書き換えるときに送らなかった値を `false` に戻すため

GitHub 上にある Ruleset を管理に取り込むときは、`reco show` の `rulesets` の表示から、その Ruleset の要素を設定ファイルに書き写します。表示は設定ファイルと同じキーと値で示すため、書き写したあとの `plan` は、その Ruleset に差分を示しません。ただし、`unavailable` のコメントが付いた Ruleset は取り込めません。`enforcement: evaluate`、`conditions: {}`、`2`・`4`・`5` 以外の `RepositoryRole` のように設定ファイルに書けない値を持つ Ruleset は、そのまま書き写すと設定ファイルの誤りになります。

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

一覧の値は、要素を消したときの扱いで2種類に分かれます。どちらの一覧かは、キーの名前からは分かりません。たとえば `environments[].variables` と `deployment_branch_policy.patterns` は、どちらも要素に `name` を持ちますが、扱いが違います。

| 一覧 | 一覧から要素を消したとき |
|---|---|
| `environments`、`actions.variables`、`environments[].variables`、`rulesets` | 消した要素の Environment、変数、Ruleset は GitHub 上に残る。変数と Ruleset を消すには、要素を消すのではなく `state: absent` を書く。Environment は reco では消せない |
| `selected_actions.patterns_allowed`、`deployment_branch_policy.patterns`、`reviewers`、`oidc_subject.include_claim_keys`、Ruleset の中の一覧（`include`、`exclude`、`bypass_actors`、`rules`、`allowed_merge_methods`、`required_status_checks`） | 書いた一覧が GitHub 上の一覧の全体になるため、消した要素は `apply` で GitHub 上からもなくなる |

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

## References

- [YAML 設定仕様](../spec/configuration.md)：キーと値、組み合わせの規則、一覧の読み方（2.1節）、Ruleset のパラメータを省略できないこと（13.5節）
- [Overview](overview.md)：GitHub の設定画面の項目と、設定ファイルのキーの対応
- [Update a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#update-a-repository)：`general` と、`security` の `secret_scanning`、`secret_scanning_push_protection` の値の意味
- [REST API endpoints for GitHub Actions permissions](https://docs.github.com/en/rest/actions/permissions?apiVersion=2026-03-10)：`actions` の `permissions`、`workflow_permissions`、`access`、`fork_pr_contributor_approval`、`fork_pr_workflows_private_repos`、`artifact_and_log_retention` の値の意味
- [Set the customization template for an OIDC subject claim for a repository](https://docs.github.com/en/rest/actions/oidc?apiVersion=2026-03-10#set-the-customization-template-for-an-oidc-subject-claim-for-a-repository)：`actions.oidc_subject` の値の意味
- [REST API endpoints for GitHub Actions variables](https://docs.github.com/en/rest/actions/variables?apiVersion=2026-03-10)：`actions.variables` と `environments[].variables` の名前と値の規則
- [Create or update an environment](https://docs.github.com/en/rest/deployments/environments?apiVersion=2026-03-10#create-or-update-an-environment)：`environments` の値の意味
- [Create a deployment branch policy](https://docs.github.com/en/rest/deployments/branch-policies?apiVersion=2026-03-10#create-a-deployment-branch-policy)：`deployment_branch_policy.patterns` の要素の `name` と `type`
- [Enable vulnerability alerts](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-vulnerability-alerts)：`security.dependabot_alerts` に当たる操作。キーは機能名の「Dependabot alerts」から取っており、API の名前（`vulnerability-alerts`）とは違う
- [Enable Dependabot security updates](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-dependabot-security-updates)：`security.dependabot_security_updates` に当たる操作
- [Enable private vulnerability reporting for a repository](https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#enable-private-vulnerability-reporting-for-a-repository)：`security.private_vulnerability_reporting` に当たる操作
- [Create a repository ruleset](https://docs.github.com/en/rest/repos/rules?apiVersion=2026-03-10#create-a-repository-ruleset)：`rulesets` の値の意味
