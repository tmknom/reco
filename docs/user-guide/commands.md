# Commands

## コマンドの一覧

```bash
reco [--log-level LEVEL] show  --repo OWNER/REPO
reco [--log-level LEVEL] plan  [--config PATH] [--repo OWNER/REPO]
reco [--log-level LEVEL] apply [--config PATH] [--repo OWNER/REPO]
```

| コマンド | 用途 | GitHub の設定を変えるか |
|---|---|---|
| `show` | 一つのリポジトリの現在の設定を見る | 変えない |
| `plan` | 設定ファイルと現在の設定を比べ、変更計画を見る | 変えない |
| `apply` | 変更計画を見て承認し、設定を変える | 承認したあとに変える |

## オプション

| オプション | 使えるコマンド | 意味 | 省略したとき |
|---|---|---|---|
| `--config PATH` | `plan`、`apply` | 読む設定ファイルのパス | `./reco.yaml` |
| `--repo OWNER/REPO` | すべて | 対象にするリポジトリを一つ指定する。`plan` と `apply` では、設定ファイルに書いたリポジトリだけを指定できる | `plan` と `apply` は設定ファイルに書いた全リポジトリが対象。`show` では省略できない |
| `--log-level LEVEL` | すべて | 診断ログの量（`debug`、`info`、`warn`、`error` など） | `info` |
| `--help`（`-h`） | すべて | 使い方を表示する | |
| `--version`（`-v`） | コマンドを付けない `reco` | バージョンを表示する | |

## `show`

`--repo` で指定したリポジトリの、設定ファイルに書ける設定の現在の値を表示します。設定ファイルを書く前に、今の値を確かめるときに使います。

```bash
$ reco show --repo example/project-a
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: false
      allow_auto_merge: false
      allow_squash_merge: true
      allow_merge_commit: true
      allow_rebase_merge: true
      allow_update_branch: false
    actions:
      permissions:
        enabled: true
        allowed_actions: all
        sha_pinning_required: false
      workflow:
        default_workflow_permissions: read
        can_approve_pull_request_reviews: false
      access:
        access_level: none
      fork_pull_requests:
        # approval_policy: unavailable (the repository is private)
        private_repositories:
          run_workflows_from_fork_pull_requests: false
          send_write_tokens_to_workflows: false
          send_secrets_and_variables: false
          require_approval_for_fork_pr_workflows: false
      retention:
        days: 90 # maximum allowed: 400
      oidc_subject:
        use_default: true
        use_immutable_subject: true
      variables:
        - name: "GO_VERSION"
          value: "1.27"
    environments:
      - name: "production"
        deployment_branch_policy:
          mode: selected
          patterns:
            - type: branch
              name: "main"
            - type: tag
              name: "v*"
        required_reviewers:
          - type: User
            id: 583231 # octocat
        variables:
          - name: "REGION"
            value: "ap-northeast-1"
    security:
      dependabot_alerts: true
      dependabot_security_updates: true
      # secret_scanning: unavailable (the repository is private)
      # secret_scanning_push_protection: unavailable (the repository is private)
      # private_vulnerability_reporting: unavailable (the repository is private)
    # rulesets: unavailable (the repository is private)
```

表示は設定ファイルと同じ構造で、それぞれの値の位置が、設定ファイルにその設定を書く位置です。表示は、設定ファイルを書く出発点に使えます。管理したい設定だけを残し、値を書き換えて使います。ただし、`unavailable` のコメントが付いた Ruleset や、`enforcement: evaluate` のように設定ファイルに書けない値を持つ Ruleset は、そのままでは設定ファイルとして読めません。その Ruleset を `rulesets` の一覧から消してから使ってください。

表示には、次の行も出ます。`#` から行末まではコメントで、設定ファイルとしては読まれません。

| 行 | 意味 |
|---|---|
| `# variables: none` | `actions` の下では Repository Variables が、Environment の下ではその Environment の変数（Environment Variables）が一つもない |
| `# environments: none` | Environment が一つもない |
| `# access:` と `#   access_level: unavailable (the repository is public)` | 公開リポジトリのため、`access_level` を読み取れない。この設定は非公開リポジトリにだけ書ける |
| `# approval_policy: unavailable (the repository is private)` / `# private_repositories: unavailable (the repository is public)` | Fork からの Pull Request のワークフローの承認ポリシー（`approval_policy`）は公開リポジトリにだけ、実行条件（`private_repositories`）は非公開リポジトリにだけあるため、非公開リポジトリでは `approval_policy` を、公開リポジトリでは `private_repositories` を読み取れない。読み取れた側は、`fork_pull_requests` とともに設定ファイルに書ける行として表示する |
| `id: 583231 # octocat` | Environment のデプロイを承認できる担当者の ID が583231で、その利用者の login が `octocat`。設定ファイルには `id` の数値を書く |
| `required_reviewers: []` | Environment にデプロイを承認できる担当者がいない |
| `# required_reviewers: no reviewers (a protection rule without reviewers remains)` | 担当者が1人もいない承認の設定が GitHub 上に残っている。設定ファイルに `required_reviewers: []` と書くと、この設定をなくす差分が `plan` に出る |
| `days: 90 # maximum allowed: 400` | 保持日数は90日で、このリポジトリで設定できる上限は400日 |
| `dependabot_security_updates: true # paused by GitHub (still enabled)` | Dependabot security updates は有効だが、GitHub が一時停止している。設定ファイルでは `true` と書いたのと同じ |
| `# secret_scanning: unavailable (the repository is private)` / `# secret_scanning_push_protection: unavailable (the repository is private)` / `# private_vulnerability_reporting: unavailable (the repository is private)` | 非公開リポジトリのため、Secret scanning、リポジトリの push protection、Private vulnerability reporting の値を読み取れない |
| `# rulesets: none` | Ruleset が一つもない |
| `# rulesets: unavailable (the repository is private)` | 非公開リポジトリのため、Ruleset を読み取れない |
| `# unavailable (the ruleset contains parts that reco does not interpret)` | 直前の `- name:` の行の Ruleset が、reco の解釈しない部分（ルール型、キー、パラメータなど）を含む。続くコメントの行が、解釈しなかったものを1行に一つずつ示す |

Environment の `wait_timer`、`prevent_self_review`、`can_admins_bypass` のように、設定ファイルに書けない設定は、上のコメントで示す設定を除いて表示しません。

Ruleset は、ルールを迂回できる主体（`bypass_actors`）も含めて `rulesets` に表示します。Ruleset を持つ公開リポジトリの `show` は、Ruleset への書き込みのアクセスを持たないトークンではエラーになります。GitHub が、そのトークンには迂回できる主体を返さないためです。

Repository Variables と Environment Variables は、名前と値をそのまま表示します。改行や制御文字を含む値は、`\n` のようなエスケープで1行に示し、長い値も省略しません。表示は stdout に出るため、CI で `show` を実行すると値がログに残ります。`plan` と `apply` が表示する変更計画にも、設定ファイルに書いた Repository Variables と Environment Variables のうち、GitHub 上にないものは書いた値が、値が違うものは今の値と書いた値が、`state: absent` を書いたものは消える今の値が出ます。パスワードやトークンのような秘密情報は、Variables ではなく Secrets に入れてください。

## `plan`

設定ファイルと GitHub 上の現在の設定を比べ、変える予定の設定（変更計画）を表示します。

```bash
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

変更計画の行は次のように読みます。

| 行 | 意味 |
|---|---|
| `Repository:` | 変える予定のあるリポジトリ。変える予定のないリポジトリは出ない |
| `<設定>: <現在の値> -> <書いた値>` | GitHub の値が設定ファイルの値と違う設定。承認すると右の値に変える |
| `environments.<名前>: absent -> present` | 設定ファイルに書いた Environment が GitHub 上にない。承認すると作成する |
| `actions.variables.<名前>: absent -> "<書いた値>"` | 設定ファイルに書いた Repository Variable が GitHub 上にない。値は引用符で囲み、改行などは `\n` のようなエスケープで1行に示す |
| `actions.variables.<名前>: "<現在の値>" -> "<書いた値>"` | Repository Variable の値が設定ファイルの値と違う。値の示し方は `absent -> "<書いた値>"` の行と同じ |
| `actions.variables.<名前>: "<現在の値>" -> absent` | `state: absent` を書いた Repository Variable が GitHub 上にある。承認すると削除する。左の値が、削除で失われる値である。引用符のない `absent` は「ない」ことを示し、値が文字列 `absent` の変数（`"absent"`）とは違う |
| `environments.<Environment の名前>.variables.<名前>: ...` | Environment Variable の作成、値の変更、削除。読み方は `actions.variables.<名前>` の行と同じ。Environment がまだないときは `(environment absent) -> "<書いた値>"` と出て、承認すると Environment を作成してから変数を作る。`state: absent` の変数は `(environment absent) -> absent` と出て、Environment を作成したあとに変数がないことを確かめるだけで、削除はしない |
| `<設定>: (<今の状態>) -> <書いた値>` | 今の値と比べられない設定。括弧の中は、比べられない理由となる今の状態（Actions が無効、Environment がまだない、Environment があるかを確かめられない、など）。承認すると右の値に変える |
| `environments.<名前>.deployment_branch_policy:` の下に、今の値の行と `-> <書いた値>` の行 | 今の値か書いた値が `selected` の `deployment_branch_policy`。読み方は1行の形と同じで、`selected` のパターンは `selected [branch "main"]` のように角括弧の中に並ぶ |
| `environments.<名前>.required_reviewers:` の下に、今の値の行と `-> <書いた値>` の行 | Environment のデプロイを承認できる担当者が、設定ファイルと違う。担当者は `[User 583231 (octocat), User 1234567]` のように並び、括弧の中は GitHub 上の担当者の login。設定ファイルに書いた利用者のうち、まだ担当者でない利用者は ID だけが出る。`[]` は担当者がいないこと、`[] (protection rule without reviewers)` は担当者が1人もいない承認の設定が GitHub 上に残っていること。承認すると `->` の行の担当者に変える |
| `Reviewers to remove:`、`Reviewers to add:` | 外れる担当者（GitHub 上にいて設定ファイルにない利用者）と、加わる担当者（設定ファイルにあって GitHub 上でまだ担当者でない利用者） |
| `Applicability: not applicable (<理由>)` | 安全に変えるための条件が成り立たないため、reco はすぐ上の変更をしない。括弧の中は、何が成り立っていないかと、設定ファイルの直し方 |
| `Patterns to remove:`、`Patterns to add:` | Environment のデプロイを許可するパターンのうち、設定ファイルにないものと、GitHub 上にないもの |
| `rulesets.<名前>: absent -> present` と、その下の `target:` から `rules:` までの行 | 設定ファイルに書いた Ruleset が GitHub 上にない。下の行は、作成する Ruleset の内容 |
| `rulesets.<名前>:` と、その下の変わる部分の行 | Ruleset の内容が設定ファイルと違う。`... to remove:` の行は、設定ファイルにないためなくなるパターン、迂回できる主体、ルール。`... to add:` の行は加わるもの。`enforcement: disabled -> active` や `Rule pull_request: ...` の行は、値が変わる項目。`conditions: {} -> ref_name` の行は、GitHub 上の Ruleset が対象のブランチの条件を持たないことを示す |
| `actions.fork_pull_requests.private_repositories:` の下に、今の値の行と `-> <書いた値>` の行 | 非公開リポジトリの Fork からの Pull Request のワークフローの実行条件。4項目をまとめて一つの変更として示す |
| `actions.oidc_subject:` の下に、今の値の行と `-> <書いた値>` の行 | OIDC トークンの subject の形の設定。`use_default`、`include_claim_keys`、`use_immutable_subject` をまとめて一つの変更として示す。設定ファイルに `use_immutable_subject` を書いていなければ、どちらの行にも出ない |
| `Note:` | すぐ上の変更が GitHub 上で何を意味するか。Fork からの Pull Request で起動したワークフローに、書き込み権限を持つ `GITHUB_TOKEN` が渡るようになること、Secrets と Variables が使えるようになることや、OIDC トークンの subject の形が変わり、外部のクラウドなどの信頼の条件と一致しなくなり得ることを示す。変更を止めるものではなく、承認の前に読むための行である |
| `Side effect:` | この変更に伴って GitHub が消すか変える値 |
| `Previous settings:` | Actions を有効に戻すと、設定ファイルに書いていない設定は、無効にする前の値で再開すること |
| `Plan:` | 変える予定のリポジトリの数と、変える設定の数 |
| `Repositories checked:` | 読み取ったリポジトリの数 |
| `No changes.` | 設定ファイルに書いた設定が、すべて GitHub 上でもそのとおりになっている |
| `Plan cannot be applied.` | `Applicability: not applicable` の変更が一つ以上ある。このままでは `apply` で変えられない |

## `apply`

変更計画を表示して承認を求め、承認されたら設定を変えます。

```bash
$ reco apply
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1

Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated

Apply complete.
Repositories updated: 1
```

`apply` は次の順に進みます。

1. 設定ファイルと GitHub 上の設定を比べ直し、変更計画を表示する
2. `Apply these changes? [y/N]:` で承認を求める
3. 承認されたら、設定ファイルに書いた順にリポジトリを一つずつ更新する
4. リポジトリごとの結果を表示する

結果は次のように読みます。

| 表示 | 意味 |
|---|---|
| `updated` | 変更を反映し、設定ファイルのとおりになったことを確かめた |
| `no change` | 変える予定がなかった |
| `failed` | 変更を始めたが、完了を確かめられなかった |
| `not attempted` | 変更を始めなかった |
| `Apply complete.` | すべてのリポジトリが `updated` か `no change` だった |

変更計画に `Plan cannot be applied.` が出たときは、承認を求めず、どのリポジトリの設定も変えずにエラーで終わります。直し方は [Troubleshooting](troubleshooting.md#plan-cannot-be-applied-と表示された) にあります。

`apply` は、設定ファイルに書いた Ruleset を、書いた順に一つずつ処理します。GitHub 上になければ作成し、あって内容が違えば、書いた内容で全体を置き換えます。置き換える直前の確認は [承認のあとで設定が変わったら止まる](safety.md#承認のあとで設定が変わったら止まる) にあります。

リポジトリのすべての変更が完了すれば、結果はリポジトリの行の `updated` だけです。変更が2件以上あり、完了しなかったものがあるときは、リポジトリの行の下に変更ごとの結果が出ます。

マージ方式（`general.allow_squash_merge`、`general.allow_merge_commit`、`general.allow_rebase_merge`）の変更は、変更計画では設定ごとに出ますが、`apply` は変えるマージ方式をまとめて1回の更新で GitHub に送ります。結果が変更ごとに出るときは、`general.allow_squash_merge + general.allow_merge_commit: updated` のように、まとめた設定を ` + ` でつないだ1行になります。

Repository Variables と Environment Variables は、設定ファイルに書いた順に一つずつ作成するか、値を変えるか、削除します。Environment Variables は、その Environment の作成と `deployment_branch_policy` の変更のあとに変えます。変数の変更ごとの結果は、`actions.variables.<名前>: updated` や `environments.<Environment の名前>.variables.<名前>: not attempted` のように出ます。

どこかの変更が完了しなかったら、reco は残りの変更を始めずに止まります。すでに反映した変更は元に戻りません。`failed` や `not attempted` が出たときにすることは [Troubleshooting](troubleshooting.md#apply-が途中で止まった) にあります。

### 承認

承認として受け付けるのは、小文字の `y` か `yes` だけです。それ以外の入力や、何も入力せずに Enter を押したときは、何も変えずに終わります。

承認は端末でだけ受け付けます。パイプで `y` を流し込んだときのように、標準入力が端末でないときは、変更計画を表示したあと、設定を変えずにエラーで終わります。

## 認証情報の環境変数

reco は、GitHub へのアクセスに使うトークンを、次の環境変数から読みます。

| 環境変数 | 使う条件 |
|---|---|
| `GH_TOKEN` | 空でなければ、こちらを使う |
| `GITHUB_TOKEN` | `GH_TOKEN` が未設定か空のときに使う |

## 出力先

| 出力先 | 出るもの |
|---|---|
| 標準出力（stdout） | `show` の現在の設定、変更計画、変更結果、`--help` の使い方、`--version` のバージョン |
| 標準エラー出力（stderr） | 承認を求める行、エラー、診断ログ |

## 終了コード

reco は、成功したときは終了コード `0`、失敗したときは `1` で終わります。`plan` も、変更計画に `Plan cannot be applied.` が出たときは、変更計画を最後まで表示したあと `1` で終わります。
