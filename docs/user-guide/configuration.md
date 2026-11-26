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
> GitHub の仕様上、GitHub Free のアカウントが所有する非公開リポジトリでは、自動マージは使えません。

`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge` の3つは、Pull Request をマージする方法を許可する設定で、まとめて **マージ方式** と呼びます。GitHub は3つをすべて無効にすることを受け付けないため、3つとも `false` と書いた設定ファイルはエラーになります。一部を省き、書いたものをすべて `false` にした場合も、書いていないマージ方式が GitHub 上ですべて無効なら、`plan` が `Plan cannot be applied.` を表示します。reco は、書いていないマージ方式を代わりに有効にしません。有効にしておくマージ方式を `true` と書き足します。

`allow_update_branch` を `true` にしても、reco も GitHub もブランチを自動では更新しません。

## GitHub Actions の設定

`actions` には、`permissions`（Actions の実行許可）、`workflow`（ワークフローの権限）、`access`（ほかのリポジトリのワークフローからのアクセス）、`fork_pull_requests`（Fork からの Pull Request のワークフローの承認と実行条件）、`retention`（Artifact や Log などの保持期間）、`oidc_subject`（OIDC トークンの subject の形）、`variables`（Repository Variables）を書きます。使わないものは省けます。

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

`none` はほかのリポジトリからのアクセスを許可しません。`user` は、同じユーザーが所有するほかの非公開リポジトリのワークフローからのアクセスを許可します。

- この設定は非公開リポジトリにだけ適用される。公開リポジトリに書くと `apply` できない
- `organization` は設定ファイルに書けない。Organization が所有するリポジトリのための値で、reco が扱うユーザー所有のリポジトリでは GitHub が受け付けないため。Organization から移転したリポジトリなどで `show` が `organization` を示したときは、`none` か `user` を書けば、書いた値に変わる
- Actions が無効のリポジトリでも書ける

### Fork からの Pull Request のワークフロー

`actions.fork_pull_requests` には、公開リポジトリ用の `approval_policy` と、非公開リポジトリ用の `private_repositories` の、どちらか一方か両方を書きます。両方を書いたときは、リポジトリの公開範囲に合わない側のために `apply` できません。合わない側を消してください。

#### 公開リポジトリの承認

| キー | 値 | 意味 |
|---|---|---|
| `approval_policy` | `first_time_contributors_new_to_github` / `first_time_contributors` / `all_external_contributors` | Fork からの Pull Request のワークフローの実行に承認が要るユーザーの範囲。承認できるのは、リポジトリへの書き込み権限を持つユーザー |

`approval_policy` の値の意味は次のとおりです。

| 値 | 承認が要るユーザー |
|---|---|
| `first_time_contributors_new_to_github` | このリポジトリにコミットか Pull Request がマージされたことがなく、GitHub に登録して間もないユーザー |
| `first_time_contributors` | このリポジトリにコミットか Pull Request がマージされたことのないユーザー |
| `all_external_contributors` | リポジトリのメンバーか所有者でも、Organization のメンバーでもない、すべてのユーザー |

- この設定は公開リポジトリにだけ適用される。非公開リポジトリに書くと `apply` できない
- 値は小文字で書く。`FIRST_TIME_CONTRIBUTORS` のような大文字の値は、GitHub が受け付けないため書けない
- Actions が無効のリポジトリでも書ける

#### 非公開リポジトリの実行条件

`actions.fork_pull_requests.private_repositories` には、次の4つのキーをすべて書きます。

```yaml
repositories:
  - name: example/private-project
    actions:
      fork_pull_requests:
        private_repositories:
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
- この設定は非公開リポジトリにだけ適用される。公開リポジトリに書くと `apply` できない。`fork_pull_requests` にほかのキーがなければ、`fork_pull_requests` ごと消す
- Actions が無効のリポジトリでも書ける

### 保持期間

`actions.retention` には、次のキーを書きます。

```yaml
repositories:
  - name: example/project-a
    actions:
      retention:
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

- 書いた名前の Environment がなければ、reco は作成する。`deployment_branch_policy`、`required_reviewers`、`variables` のどれも省いた Environment は、あるかどうかだけを管理する
- 書かなかった Environment は、変更も削除もしない。設定ファイルから名前を消しても、GitHub 上の Environment は残る
- 名前は、半角の英数字、空白、記号だけで書く。大文字と小文字だけが違う名前は同じ Environment とみなすため、同じリポジトリに並べられない
- 保護ルールのあるブランチが一つもなければ、`protected_branches` でもすべてのブランチからデプロイできる（GitHub の仕様）。ブランチの保護ルールは reco では作れない
- `patterns` には、デプロイを許可するパターンをすべて、順不同で書く。一つも許可しないときは `patterns: []` と書く。`all` と `protected_branches` では `patterns` を書けない
- `apply` は、GitHub 上にあって `patterns` にないパターンを取り除き、`patterns` にあって GitHub 上にないパターンを加える
- `type` が違えば、名前が同じでも別のパターンになる。大文字と小文字だけが違う名前（`main` と `MAIN`）も別のパターンになる
- `*` で始まる名前は、引用符で囲まないと設定ファイルを読めない。パターンの名前は引用符で囲んで書くとよい
- `selected` の Environment を `all` などに変えると、GitHub は並んでいたパターンを残さない

### デプロイを承認できる担当者

`environments` の各 Environment の `required_reviewers` には、その Environment を使うデプロイジョブの実行を承認できる担当者（GitHub の画面の「Required reviewers」）を並べます。

```yaml
repositories:
  - name: example/project-a
    environments:
      - name: production
        required_reviewers:
          - type: User
            id: 583231
          - type: User
            id: 1234567
```

| キー | 値 | 意味 |
|---|---|---|
| `required_reviewers` | 下の2つのキーを持つ要素の一覧（6件まで） | デプロイを承認できる担当者の一覧の全体 |
| `required_reviewers[].type` | `User`（必須） | 担当者の種類 |
| `required_reviewers[].id` | 正の整数（必須） | 担当者の数値の ID |

- 担当者のうち1人が承認すれば、デプロイジョブは進む
- `id` は利用者の login ではなく、数値の ID を書く。`reco show` が、いまの担当者を `id` と、行末のコメントの login で示すので、そこから書き写せる。まだ担当者でない利用者の ID は、GitHub CLI の `gh api users/<login> --jq .id` で調べられる
- 担当者をすべて、順不同で書く。GitHub 上にいて書かなかった担当者は、外す差分として `plan` に出る
- 担当者がいないことは `required_reviewers: []` と書く。`required_reviewers` の行を消すと、担当者を管理しなくなる
- 担当者をすべて外すと、GitHub は画面の「Prevent self-review」（自分が起動したデプロイを自分で承認できなくする設定）も一緒に消す。有効だったなら、`plan` の `Side effect:` の行に、`prevent_self_review` が消えることが出る。あとで担当者を設定し直しても、「Prevent self-review」は無効のままになる
- GitHub 上にない Environment に担当者を書くと、reco は Environment を作ってから担当者を設定する。担当者の設定が失敗すると、担当者のいない Environment が残る
- `type: Team` は書けない。個人アカウントが所有するリポジトリでは GitHub が Team を担当者として受け付けず、Organization が所有するリポジトリでの動作を reco は確かめていない
- この設定が止めるのは、その Environment を使うデプロイジョブであり、Ruleset の `pull_request` のような Pull Request のレビューとは別の設定である
- 担当者が1人だけのとき、「Prevent self-review」を有効にすると、その人が起動したデプロイを承認できる担当者がいなくなる

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

`rulesets` には、ブランチへの操作を制限する Ruleset を、名前ごとに並べます。書いた名前の Ruleset が GitHub 上になければ、`apply` が作成します。あって内容が違えば、`apply` が、書いた内容で Ruleset の全体を置き換えます。

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
          - type: deletion
```

| キー | 値 | 意味 |
|---|---|---|
| `name` | 文字列（必須） | Ruleset の名前 |
| `target` | `branch`（必須） | Ruleset を適用する対象。ブランチだけを扱う |
| `enforcement` | `active` / `disabled`（必須） | ルールを強制するか |
| `conditions.ref_name.include` | パターンの一覧（必須。`[]` 可） | 対象にするブランチ。`~DEFAULT_BRANCH`（デフォルトブランチ）、`~ALL`（すべてのブランチ）、`refs/heads/` で始まるパターン（`refs/heads/release/*` など） |
| `conditions.ref_name.exclude` | パターンの一覧（必須。`[]` 可） | 対象から外すブランチ |
| `bypass_actors` | 要素の一覧（必須。`[]` 可） | ルールを迂回できる主体。`actor_type`、`actor_id`、`bypass_mode` を書く |
| `rules` | 要素の一覧（必須。`[]` 可） | 課すルール。`pull_request`（Pull Request を経ることを必須にする）と `deletion`（ブランチの削除の制限）を書ける |

**Ruleset は、書いた内容がその Ruleset の全体になります**。GitHub 上の Ruleset にあって設定ファイルにないルール、迂回できる主体、パターンは、適用するとなくなります。`plan` は、なくなるものを `Rules to remove:` や `Bypass actors to remove:` の行で示します。

表のキーはどれも省略できません。書き忘れを既定値で補わないためです。ほかに気をつけることは次のとおりです。

- 書いた名前の Ruleset がなければ、`plan` は作成を示す。書かなかった Ruleset を、reco は変更も削除もしない
- 名前は、半角の英数字、空白、記号だけで書き、前後に空白を置かない。大文字と小文字だけが違う名前は同じ Ruleset とみなすため、同じリポジトリに並べられない
- `bypass_actors` の `actor_type` は `RepositoryRole`、`User`、`Integration`、`DeployKey` のどれか。`bypass_mode` は `always`、`pull_request`、`exempt` のどれかで、省略できない。`RepositoryRole` の `actor_id` は `2`、`4`、`5` のどれかで、ロール名ではなく数値で書く。どの数値がどのロールかは公開されていないため、`show` の表示から書き写す。`DeployKey` には `actor_id` を書かず、`bypass_mode` は `always` だけ
- `pull_request` の `parameters` は、例に示したキーをすべて書く。`required_approving_review_count` は `0` から `10` の整数
- 一覧の要素の順序は問わない。並びだけが違っても変更にならない
- 非公開リポジトリには書けない。書くと `plan` に `Applicability: not applicable` と `Plan cannot be applied.` が出る
- GitHub 上に、半角の英数字、空白、記号以外の文字（`é` など）を含む名前の Ruleset があると、そのリポジトリにまだない名前の Ruleset は作成できない。GitHub がその名前を書いた名前と同じとみなすかを reco が判断できないため、`plan` は `(existence unknown)` と `Applicability: not applicable` を示す。GitHub 上にすでにある Ruleset の名前を書いた場合は、内容を比べられる
- reco がまだ解釈しないルール（`non_fast_forward` など）や `pull_request` の設定（`allowed_merge_methods` など）を持つ Ruleset の名前を書くと、`plan` に `Applicability: not applicable` と、reco の解釈しない部分の一覧が出る。そうした Ruleset は設定ファイルから外す

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
