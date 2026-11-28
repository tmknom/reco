# Overview

reco（Repository Configurator）は、GitHub リポジトリの設定値を YAML の設定ファイルに書き、GitHub 上の現在の設定との差を示して、利用者の承認を得てから設定を変える CLI です。

## reco が解決する問題

GitHub の機能やセキュリティの設定は変わり続けます。多数のリポジトリを持つと、どのリポジトリでどの設定が有効になっているかを覚えておくことも、設定画面を一つずつ開いて確かめ続けることも難しくなり、直すべき設定を見落としやすくなります。

reco を使うと、個々のリポジトリの設定を覚えていなくても、設定ファイルを読めば各リポジトリで意図した設定値が分かり、`plan` を実行すれば今の設定がそこからずれていないかを確かめられます。

## 基本の流れ

設定ファイルには、管理したいリポジトリと、そのリポジトリで管理したい設定の値だけを書きます。次の例は、一つのリポジトリで、マージ後の head ブランチの自動削除を有効にしたいという設定です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

設定ファイルを書いてから適用するまでの流れは次のとおりです。

```mermaid
flowchart TB
    W[設定ファイルに設定値を書く] --> P["plan<br>現在の設定との差（変更計画）を見る"]
    P --> A["apply<br>変更計画を見て承認する"]
    A --> U[反映した結果を確かめる]
```

- **`plan`**：設定ファイルと GitHub 上の現在の設定を比べ、変更計画を表示する。GitHub の設定は変えない
- **`apply`**：実行した時点の値から変更計画を作り直して表示し、承認を求める。承認されたら、設定ファイルに書いた順にリポジトリを更新し、更新後の設定が設定ファイルのとおりになったことを確かめて結果を示す
- **`show`**：設定ファイルを使わずに、一つのリポジトリの現在の設定を表示する。設定ファイルを書く前に今の値を確かめるときに使う

## 始め方

1. [Installation](installation.md) に従って、reco をインストールする
2. [Getting Started](getting-started.md) に従って、reco を実行する
3. [Configuration](configuration.md) に従って、リポジトリ設定を記述する

## 管理できる設定

reco が管理可能な設定は、次のとおりです。具体的な設定方法は [Configuration](configuration.md) にあります。

設定ファイルのキーの名前は、ほとんどが GitHub の REST API のフィールド名と同じです。そのため、キーの名前で GitHub の REST API のドキュメントを検索すれば、多くの場合、値の意味を確かめられます。

| reco で設定できること | GitHub の設定画面の項目 | 設定ファイルのキー |
|---|---|---|
| マージ後に head ブランチを自動で削除するか | 「General」ページの「Pull Requests」の欄の「Automatically delete head branches」 | `general.delete_branch_on_merge` |
| 自動マージを許可するか | 「General」ページの「Pull Requests」の欄の「Allow auto-merge」 | `general.allow_auto_merge` |
| Pull Request を squash merge でマージできるようにするか | 「General」ページの「Pull Requests」の欄の「Allow squash merging」 | `general.allow_squash_merge` |
| Pull Request を merge commit でマージできるようにするか | 「General」ページの「Pull Requests」の欄の「Allow merge commits」 | `general.allow_merge_commit` |
| Pull Request を rebase merge でマージできるようにするか | 「General」ページの「Pull Requests」の欄の「Allow rebase merging」 | `general.allow_rebase_merge` |
| squash merge と merge commit でマージするときのコミットメッセージの形（タイトルと本文） | 「General」ページの「Pull Requests」の欄の「Allow squash merging」と「Allow merge commits」の下の選択肢 | `general.squash_merge_commit`、`general.merge_commit` |
| base ブランチより遅れた Pull Request に、ブランチの更新を常に提案するか | 「General」ページの「Pull Requests」の欄の「Always suggest updating pull request branches」 | `general.allow_update_branch` |
| Issues を使えるようにするか | 「Features」の欄の「Issues」 | `general.has_issues` |
| Projects を使えるようにするか | 「Features」の欄の「Projects」 | `general.has_projects` |
| Wiki を使えるようにするか | 「Features」の欄の「Wikis」 | `general.has_wiki` |
| Web 画面からのコミットに、作成者のサインオフを求めるか | 「Commits」の欄の「Require contributors to sign off on web-based commits」 | `general.web_commit_signoff_required` |
| Actions を有効にするか、無効にするか | 「Actions」の「General」ページの「Actions permissions」の欄 | `actions.permissions.enabled` |
| 実行を許可する Actions と再利用可能なワークフローの範囲（すべて、ローカルのものだけ、選んだものだけ） | 「Actions」の「General」ページの「Actions permissions」の欄。選んだものだけを許可する選択肢は「Allow *OWNER*, and select non-*OWNER*, actions and reusable workflows」 | `actions.permissions.allowed_actions` |
| 選んだものだけを許可するときに、GitHub が作った Actions を許可するか | 「Allow actions created by GitHub」 | `actions.permissions.selected_actions.github_owned_allowed` |
| 選んだものだけを許可するときに、Marketplace の認証済み作成者の Actions を許可するか | 「Allow actions by Marketplace verified creators」 | `actions.permissions.selected_actions.verified_allowed` |
| 選んだものだけを許可するときに、個別に許可する Actions と再利用可能なワークフロー | 「Allow or block specified actions and reusable workflows」 | `actions.permissions.selected_actions.patterns_allowed` |
| Actions を完全な長さのコミット SHA に固定することを必須にするか | 「Actions」の「General」ページの「Actions permissions」の欄の「Require actions to be pinned to a full-length commit SHA」 | `actions.permissions.sha_pinning_required` |
| `GITHUB_TOKEN` のデフォルト権限を、読み書きにするか、読み取りに限るか | 「Actions」の「General」ページの「Workflow permissions」の欄 | `actions.workflow_permissions.default_workflow_permissions` |
| GitHub Actions に Pull Request の作成と承認を許すか | 「Actions」の「General」ページの「Workflow permissions」の欄の「Allow GitHub Actions to create and approve pull requests」 | `actions.workflow_permissions.can_approve_pull_request_reviews` |
| ほかのリポジトリのワークフローに、このリポジトリの Actions と再利用可能なワークフローを使わせるか（非公開リポジトリだけ） | 「Actions」の「General」ページの「Access」の欄 | `actions.access.access_level` |
| Fork からの Pull Request のワークフローの実行に承認が要るユーザーの範囲（公開リポジトリだけ） | 「Actions」の「General」ページの「Approval for running fork pull request workflows from contributors」の欄 | `actions.fork_pr_contributor_approval.approval_policy` |
| Fork からの Pull Request のワークフローを実行するか、書き込み権限を持つ `GITHUB_TOKEN` や Secrets と Variables を渡すか、実行に承認を求めるか（非公開リポジトリだけ） | 「Actions」の「General」ページの「Fork pull request workflows」の欄 | `actions.fork_pr_workflows_private_repos` |
| チェック、ワークフローの実行、コミットのステータス、Artifact、Log を保持する日数 | 「Actions」の「General」ページの「Check, workflow run, status, artifact and log retention」の欄 | `actions.artifact_and_log_retention.days` |
| ワークフローが外部のクラウドなどへの認証に使う OIDC トークンの subject の形 | 設定画面の項目はない。GitHub の REST API の「Set the customization template for an OIDC subject claim for a repository」で設定する値 | `actions.oidc_subject` |
| ワークフローから読む Repository Variables の名前と値 | 「Secrets and variables」の「Actions」ページの「Variables」タブ | `actions.variables` |
| デプロイ先の Environment があるか（なければ作成し、`state: absent` と書けば削除する） | 「Environments」ページの Environment の一覧 | `environments[].name`、`environments[].state` |
| Environment へデプロイできるブランチとタグを、制限しないか、保護ルールのあるブランチに限るか、選んだパターンに限るか | 「Environments」ページの Environment の「Deployment branches」の選択肢の「No restriction」、「Protected branches only」、「Selected branches and tags」 | `environments[].deployment_branch_policy.mode` |
| 選んだパターンに限るときに、デプロイを許可するブランチとタグのパターン | 「Selected branches and tags」で追加するパターンの一覧 | `environments[].deployment_branch_policy.patterns` |
| その Environment を使うデプロイジョブの実行を承認できる担当者 | 「Environments」ページの Environment の「Required reviewers」 | `environments[].reviewers` |
| デプロイジョブを起動した利用者が、自分のジョブを承認できないようにするか | 「Required reviewers」の下の「Prevent self-review」 | `environments[].prevent_self_review` |
| デプロイジョブを実行するまで待たせる時間（分） | 「Environments」ページの Environment の「Wait timer」 | `environments[].wait_timer` |
| その Environment を使うジョブから読む Environment Variables の名前と値（Environment がなければ作成する） | 「Environments」ページの Environment の「Environment variables」の欄 | `environments[].variables` |
| 依存関係の脆弱性を Dependabot alerts で知らせるか | 「Advanced Security」ページの「Dependabot alerts」の項目 | `security.dependabot_alerts` |
| 脆弱性を直す Pull Request を Dependabot security updates に作らせるか | 「Advanced Security」ページの「Dependabot security updates」の項目 | `security.dependabot_security_updates` |
| リポジトリに push されたシークレットを Secret scanning で検出するか（公開リポジトリだけ） | 「Advanced Security」ページの「Secret Protection」の項目 | `security.secret_scanning` |
| シークレットを含む push をリポジトリで拒否するか（公開リポジトリだけ） | 「Advanced Security」ページの「Secret Protection」の欄の「Push protection」の項目 | `security.secret_scanning_push_protection` |
| 見つけた脆弱性を、公開せずにメンテナーへ報告できるようにするか（公開リポジトリだけ） | 「Advanced Security」ページの「Private vulnerability reporting」の項目 | `security.private_vulnerability_reporting` |
| ブランチを保護する Ruleset の内容（対象のブランチ、強制するか、迂回できる主体、Pull Request を経ることを必須にするか、ブランチの削除を制限するか、成功を必須にするステータスチェック、Force push を制限するか）（公開リポジトリだけ） | 「Rulesets」ページ | `rulesets` |

## References

- [YAML 設定仕様](../spec/configuration.md)：設定ファイルのキーと、受け付ける値（3章〜13章）
- [GitHub API 仕様](../research/README.md)：設定ファイルのキーに当たる REST API のフィールドと、GitHub の設定画面の項目名の出典
