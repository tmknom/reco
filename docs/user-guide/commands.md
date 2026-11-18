# コマンドリファレンス

## コマンドの一覧

```text
reco [--log-level LEVEL] show  --repo OWNER/REPO
reco [--log-level LEVEL] plan  [--config PATH] [--repo OWNER/REPO]
reco [--log-level LEVEL] apply [--config PATH] [--repo OWNER/REPO]
```

| コマンド | 用途 | 設定ファイル | GitHub の設定を変えるか |
|---|---|---|---|
| `show` | 一つのリポジトリの現在の設定を見る | 使わない | 変えない |
| `plan` | 設定ファイルと現在の設定を比べ、変更計画を見る | 使う | 変えない |
| `apply` | 変更計画を見て承認し、設定を変える | 使う | 承認したあとに変える |

## オプション

| オプション | 使えるコマンド | 意味 | 省略したとき |
|---|---|---|---|
| `--config PATH` | `plan`、`apply` | 読む設定ファイルのパス | `./reco.yaml` |
| `--repo OWNER/REPO` | すべて | 対象にするリポジトリを一つ指定する。`plan` と `apply` では、設定ファイルに書いたリポジトリだけを指定できる | `plan` と `apply` は設定ファイルに書いた全リポジトリが対象。`show` では省略できない |
| `--log-level LEVEL` | すべて | 診断ログの量（`debug`、`info`、`warn`、`error` など） | `info` |
| `--help`（`-h`） | すべて | 使い方を表示する | |
| `--version`（`-v`） | コマンドを付けない `reco` | バージョンを表示する | |

`--repo` で対象を一つに絞っても、reco は設定ファイル全体を読むため、ほかのリポジトリの書き方に誤りがあれば止まります。

## `show`

`--repo` で指定したリポジトリの、reco が扱える設定の現在の値を表示します。設定ファイルを書く前に、今の値を確かめるときに使います。

```text
$ reco show --repo example/project-a
Repository: example/project-a

General:
  delete_branch_on_merge: false
  allow_auto_merge: false
Actions permissions:
  enabled: true
  allowed_actions: all
  selected_actions: unavailable (allowed_actions is not "selected")
Workflow permissions:
  default_workflow_permissions: read
  can_approve_pull_request_reviews: false
```

`unavailable` は、リポジトリの状態によって GitHub がその設定の値を返さないことを示します。この例では、`allowed_actions` が `selected` でないため、`selected_actions` の値がありません。

## `plan`

設定ファイルと GitHub 上の現在の設定を比べ、変える予定の設定（変更計画）を表示します。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
Repositories checked: 1
```

変更計画の行は次のように読みます。

| 行 | 意味 |
|---|---|
| `Repository:` | 変える予定のあるリポジトリ。変える予定のないリポジトリは出ない |
| `<設定>: <現在の値> -> <書いた値>` | GitHub の値が設定ファイルの値と違う設定。承認すると右の値に変える |
| `Current: unavailable (<理由>)` と `Desired: <書いた値>` | リポジトリの状態によって GitHub が今の値を返さないため、比べられなかった設定。承認すると `Desired:` の値にする |
| `Side effect:` | この変更に伴って GitHub が消す値。読み取れた値は括弧の中に出る |
| `Plan:`、`Repositories checked:` | 変える予定のリポジトリの数、今の値と違うと分かった設定の数（`known change`）、今の値が分からなかった設定の数（`not-comparable settings`）、読み取ったリポジトリの数 |
| `No changes.` | 設定ファイルに書いた設定が、すべて GitHub 上でもそのとおりになっている |

`Side effect:` は、`allowed_actions` を `selected` から `all` や `local_only` に変えるときに出ます。あとで同じ値に戻す可能性があるなら、承認の前に括弧の中の値を控えておきます。この変更で何が起きるかは [Actions 許可ポリシーの切り替え](actions-policy.md) にあります。

## `apply`

変更計画を表示して承認を求め、承認されたら設定を変えます。

```text
$ reco apply
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
Repositories checked: 1

Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated

Apply complete.
Repositories updated: 1
```

`apply` は次の順に進みます。

1. 設定ファイルと GitHub 上の設定を比べ直し、変更計画を表示する。変える予定がなければ `No changes.` を表示して終わる
2. `Apply these changes? [y/N]:` で承認を求める
3. 承認されたら、設定ファイルに書いた順にリポジトリを一つずつ更新する。更新するたびに GitHub の値を読み直し、設定ファイルのとおりになったことを確かめてから次へ進む
4. リポジトリごとの結果を表示する

結果は次のように読みます。

| 表示 | 意味 |
|---|---|
| `updated` | 変更を反映し、設定ファイルのとおりになったことを確かめた |
| `no change` | 変える予定がなかった |
| `failed` | 変更を始めたが、完了を確かめられなかった |
| `not attempted` | 変更を始めなかった |
| `Apply complete.` | すべてのリポジトリが `updated` か `no change` だった |

どこかの変更が完了しなかったら、reco は残りの変更を始めずに止まります。すでに反映した変更は元に戻りません。`failed` や `not attempted` が出たときにすることは [困ったときは](troubleshooting.md#apply-が途中で止まった) にあります。

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

エラーは、stderr に JSON の1行で出ます。エラーの見方と、エラーごとにすることは [困ったときは](troubleshooting.md) にあります。

## 終了コード

reco は、成功したときは終了コード `0`、失敗したときは `1` で終わります。

## References

- [CLI 仕様](../spec/cli.md)：コマンドとオプション（2章）、認証情報の選択（3章）、変更計画の表示（4章）、`apply` の手順、承認、変更結果の表示（5章）、`show`（6章）、出力先と終了コード（7章）
- [要求定義](../spec/requirements.md)：対象リポジトリの選択（FR-002）
