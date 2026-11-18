# はじめての適用

一つのリポジトリの一つの設定を、reco で変えてみます。

## 用意するもの

- 「インストール」の手順で入れた `reco`
- GitHub CLI（`gh`）へのログイン
- 自分の個人アカウントが所有し、自分が Admin ロールを持つ GitHub のリポジトリを一つ。設定が変わっても困らないリポジトリを選ぶ
- 作業用の空のディレクトリ

作業用のディレクトリに移り、トークンを環境変数に入れておきます。

```bash
export GH_TOKEN=$(gh auth token)
```

以下の `example/project-a` は、コマンドでも `reco.yaml` でも、自分のリポジトリ（`OWNER/REPO` の形）に読み替えてください。

## 現在の設定を確かめる

`show` で、対象のリポジトリの現在の設定を表示します。`show` は GitHub の設定を変えません。

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

ここでは、`General:` の `delete_branch_on_merge`（Pull Request をマージしたあと、head ブランチを自動で削除するか）を変えます。`unavailable` と表示される設定は、この手順では使わないので気にしなくてかまいません。

## 最初の設定ファイルを書く

作業用のディレクトリに `reco.yaml` を作り、`delete_branch_on_merge` に、`show` で見た現在の値と反対の値を書きます。次は、現在の値が `false` だった場合です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

reco が管理するのは、設定ファイルに書いた設定だけです。`allow_auto_merge` や Actions の設定は書いていないため、reco は変えません。

## 変更計画を確かめる

`plan` で、設定ファイルと現在の設定を比べた変更計画を表示します。`plan` は GitHub の設定を変えません。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
Repositories checked: 1
```

`general.delete_branch_on_merge: false -> true` は、GitHub の現在の値が `false` で、設定ファイルに書いた値が `true` であることを示します。

## 承認して適用する

`apply` を実行すると、`plan` と同じ形の変更計画が表示され、承認を求められます。内容が意図どおりなら `y` を入力して Enter を押します。

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

reco が設定を変えるのは、`y` か `yes` を入力したときだけです。何も入力せずに Enter を押すと、何も変えずに終わります。

`example/project-a: updated` は、このリポジトリの変更が完了したことを示します。

## 適用を確かめる

もう一度 `plan` を実行します。

```text
$ reco plan
No changes.
Repositories checked: 1
```

`No changes.` は、設定ファイルに書いた設定が GitHub 上でもそのとおりになっていることを示します。

## 元に戻したいとき

元に戻すには、`reco.yaml` の `delete_branch_on_merge` を元の値（この例では `false`）に書き直し、もう一度 `apply` します。設定ファイルから行を消すだけでは、GitHub の値は戻りません。

## References

- [インストール](installation.md)：`reco` の入れ方
- [トークンの用意](authentication.md)：`gh auth token` のトークンを渡す理由と、トークンの持ち主に要るロール
- [設定ファイルの書き方](configuration.md)：管理できるほかの設定と、未記載の設定の扱い
- [コマンドリファレンス](commands.md)：各コマンドの出力の読み方と、承認として受け付ける入力
- [複数リポジトリの運用](operations.md)：管理するリポジトリを増やす手順
- [困ったときは](troubleshooting.md)：エラーで止まったときにすること
- [CLI 仕様](../spec/cli.md)：対象が個人アカウントの所有するリポジトリであること（1章）、コマンドの一覧（2.1節）、`plan` の表示（4章）、`apply` の承認と表示（5.1節）、`updated` と `Apply complete.`（5.2節）、`show`（6章）
- [YAML 設定仕様](../spec/configuration.md)：未記載の設定を管理しないこと（2章）、`delete_branch_on_merge` の意味（3章）
