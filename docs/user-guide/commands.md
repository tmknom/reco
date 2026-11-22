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
    security:
      dependabot_alerts: true
      dependabot_security_updates: true
      # secret_scanning: unavailable (the repository is private)
      # secret_scanning_push_protection: unavailable (the repository is private)
      # private_vulnerability_reporting: unavailable (the repository is private)
```

表示は設定ファイルと同じ構造で、それぞれの値の位置が、設定ファイルにその設定を書く位置です。表示をそのまま設定ファイルにすると、表示したすべての設定を今の値のまま管理する設定ファイルになります。管理したい設定だけを残し、値を書き換えて使います。

表示には、次の行も出ます。`#` から行末まではコメントで、設定ファイルとしては読まれません。

| 行 | 意味 |
|---|---|
| `# variables: none` | Repository Variables が一つもない |
| `# environments: none` | Environment が一つもない |
| `# access:` と `#   access_level: unavailable (the repository is public)` | 公開リポジトリのため、`access_level` を読み取れない。この設定は非公開リポジトリにだけ書ける |
| `dependabot_security_updates: true # paused by GitHub (still enabled)` | Dependabot security updates は有効だが、GitHub が一時停止している。設定ファイルでは `true` と書いたのと同じ |
| `# secret_scanning: unavailable (the repository is private)` / `# secret_scanning_push_protection: unavailable (the repository is private)` / `# private_vulnerability_reporting: unavailable (the repository is private)` | 非公開リポジトリのため、Secret scanning、リポジトリの push protection、Private vulnerability reporting の値を読み取れない |

Environment の `wait_timer`、`required_reviewers`、`can_admins_bypass` のように、設定ファイルに書けない設定は表示しません。

Repository Variables は、名前と値をそのまま表示します。改行や制御文字を含む値は、`\n` のようなエスケープで1行に示し、長い値も省略しません。表示は stdout に出るため、CI で `show` を実行すると値がログに残ります。`plan` と `apply` が表示する変更計画にも、設定ファイルに書いた変数のうち、GitHub 上にないものは書いた値が、値が違うものは今の値と書いた値が出ます。パスワードやトークンのような秘密情報は、Variables ではなく Secrets に入れてください。

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
| `<設定>: (<今の状態>) -> <書いた値>` | 今は効いていない設定（Actions が無効、Environment がまだない、など）。承認すると右の値に変える |
| `Applicability: not applicable (<理由>)` | 安全に変えるための条件が成り立たないため、reco はすぐ上の変更をしない。括弧の中は、何が成り立っていないかと、設定ファイルの直し方 |
| `Patterns to remove:`、`Patterns to add:` | Environment のデプロイを許可するパターンのうち、設定ファイルにないものと、GitHub 上にないもの |
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

Repository Variables は、設定ファイルに書いた順に一つずつ作成するか値を変え、結果もリポジトリの行の下に `actions.variables.<名前>: updated` のように変数ごとに出ます。

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
