# はじめての適用

一つのリポジトリの一つの設定を、reco で変えてみます。用語の意味は [用語集](glossary.md) にあります。

## 用意するもの

- [インストールと認証の準備](installation.md) で入れた `reco` と、GitHub CLI（`gh`）のログイン
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

ここでは、`General:` の `delete_branch_on_merge`（Pull Request をマージしたあと、head ブランチを自動で削除するか）を変えます。この設定が Pull Request やブランチにどう効くかは、[リポジトリの一般設定](configuration.md#リポジトリの一般設定) にあります。

`unavailable`（観測不能）は、リポジトリの状態によって GitHub がその設定の値を返さないことを示します（[比較不能な設定予定](reading-output.md#比較不能な設定予定)）。この手順では使わない設定なので、気にしなくてかまいません。

## 最初の設定ファイルを書く

作業用のディレクトリに `reco.yaml` を作り、`delete_branch_on_merge` に、`show` で見た現在の値と反対の値を書きます。次は、現在の値が `false` だった場合です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

reco が管理するのは、設定ファイルに書いた設定だけです。`allow_auto_merge` や Actions の設定は書いていないため、reco は変えません（[書かなかった設定はどうなるか](configuration.md#書かなかった設定はどうなるか)）。

## 変更計画を確かめる

`plan` で、設定ファイルと現在の設定を比べた変更計画を表示します。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
Repositories checked: 1
```

`general.delete_branch_on_merge: false -> true` は、GitHub の現在の値が `false` で、設定ファイルに書いた期待値が `true` であることを示します。最後の2行は、変更予定のあるリポジトリが1件、確認済みの差分が1件、比較不能な設定予定が0件、対象にしたリポジトリが1件であることを示します（[変更計画の集計](reading-output.md#変更計画の集計)）。

`plan` は GitHub の設定を変えません。

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

reco が設定を変えるのは、`y` か `yes` を入力したときだけです。何も入力せずに Enter を押すと、何も変えずに終わります（[承認](commands.md#承認)）。

`example/project-a: updated` は、このリポジトリの変更が完了したことを示します。`Apply complete.` は、完了しなかった変更がないことを示します（[`apply` の結果の区分](reading-output.md#apply-の結果の区分)）。

## 適用を確かめる

もう一度 `plan` を実行します。

```text
$ reco plan
No changes.
Repositories checked: 1
```

`No changes.` は、管理設定が期待値と一致すると確かめたことを示します。`reco show --repo example/project-a` でも、`delete_branch_on_merge` が `true` になったことを確かめられます。

## 元に戻したいとき

元に戻すには、`reco.yaml` の `delete_branch_on_merge` を元の値（この例では `false`）に書き直し、もう一度 `apply` します。設定ファイルから行を消すだけでは、GitHub の値は戻りません（[書かなかった設定はどうなるか](configuration.md#書かなかった設定はどうなるか)）。

## 次に読む文書

- 設定を増やす：[設定ファイルの書き方](configuration.md)
- 変更計画と変更結果の各行の意味を知る：[変更計画と変更結果の読み方](reading-output.md)
- 管理するリポジトリを増やす：[複数リポジトリの運用](operations.md)

## References

- [CLI 仕様](../spec/cli.md)：対象が個人アカウントの所有するリポジトリであること（1章）、コマンドの一覧（2.1節）、`plan` の表示（4章）、`apply` の承認と表示（5.1節）、`updated` と `Apply complete.`（5.2節）、`show`（6章）
- [YAML 設定仕様](../spec/configuration.md)：書かなかった設定を管理しないこと（2章）、`delete_branch_on_merge` の意味（3章）
