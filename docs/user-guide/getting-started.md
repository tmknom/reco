# Getting Started

一つのリポジトリの一つの設定を、reco で変えてみます。

## 用意するもの

- [Installation](installation.md) の手順でインストールした `reco`
- GitHub CLI（`gh`）へのログイン
- Admin ロールを持つ GitHub のリポジトリ

トークンを `GH_TOKEN` 環境変数にセットします。

```bash
export GH_TOKEN=$(gh auth token)
```

以下の `example/project-a` は、コマンドでも `reco.yaml` でも、自分のリポジトリ（`OWNER/REPO` の形）に読み替えてください。

## 現在の設定を確かめる

`show` で、対象リポジトリの現在の設定を表示します。

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
      # reco does not accept the merge commit message settings in the configuration file yet
      # squash_merge_commit:
      #   title: COMMIT_OR_PR_TITLE
      #   message: COMMIT_MESSAGES
      # merge_commit:
      #   title: MERGE_MESSAGE
      #   message: PR_TITLE
      # reco does not accept the repository feature and web commit signoff settings in the configuration file yet
      # has_issues: true
      # has_projects: true
      # has_wiki: true
      # web_commit_signoff_required: false
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
      # variables: none
    # environments: none
    security:
      dependabot_alerts: true
      dependabot_security_updates: true
      # secret_scanning: unavailable (the repository is private)
      # secret_scanning_push_protection: unavailable (the repository is private)
      # private_vulnerability_reporting: unavailable (the repository is private)
    # rulesets: unavailable (the repository is private)
```

表示は、設定ファイルと同じ構造です。ここでは、`general` の `delete_branch_on_merge`（Pull Request をマージしたあと、head ブランチを自動で削除するか）が `false` のリポジトリで、`true` に変えます。

## 最初の設定ファイルを書く

設定ファイル「`reco.yaml`」を作成し、`show` の表示を写し、`delete_branch_on_merge` 以外の設定の行を消して、値を `true` に書き換えます。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

## 変更計画を確かめる

`plan` で、設定ファイルと現在の設定を比べた変更計画を表示します。

```bash
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

`general.delete_branch_on_merge: false -> true` は、GitHub の現在の値が `false` で、設定ファイルの値が `true` であることを示します。現在の設定との差分を確認したら、実際に適用します。

## 承認して適用する

`apply` を実行すると、`plan` と同じ形の変更計画が表示され、承認を求められます。内容が意図どおりなら `y` を入力して Enter を押します。

```bash
$ reco apply
Configuration: ./reco.yaml

Repository: example/project-a
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1

Apply these changes? [y/N]: # ⇐ y と入力する
```

すると次のように `example/project-a` リポジトリの設定が変更されます。

```bash
Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated

Apply complete.
Repositories updated: 1
```

承認を求められた際に、`y` か `yes` 以外を入力すると、何も変更せずに終了します。

## 適用を確かめる

もう一度 `plan` を実行します。

```bash
$ reco plan
No changes.
Repositories checked: 1
```

今度は現在の設定と設定ファイルのあいだに差分がないため、`No changes.` が表示されます。

## 元に戻したいとき

元に戻すには、`reco.yaml` の `delete_branch_on_merge` を元の値（この例では `false`）に書き直し、もう一度 `apply` します。

## References

- [Installation](installation.md)：`reco` のインストール方法
- [Configuration](configuration.md)：管理できるほかの設定と、未記載の設定の扱い
- [Commands](commands.md)：各コマンドの出力の読み方と、承認として受け付ける入力
- [Authentication](authentication.md)：`gh auth token` のトークンを渡す理由と、トークンの持ち主に要るロール
- [Operations](operations.md)：管理するリポジトリを増やす手順
- [Troubleshooting](troubleshooting.md)：エラーで止まったときにすること
