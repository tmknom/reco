# 設定ファイルの書き方

reco は、リポジトリごとの設定値を書いた YAML の設定ファイルを読み、GitHub 上の設定と比べて、書いた値になるように設定を変えます。

## 設定ファイルの場所と構造

reco は、既定ではカレントディレクトリの `reco.yaml` を読みます。別の場所のファイルを使うときは、`reco plan --config path/to/file.yaml` のように指定します。

設定ファイルのルートには `repositories` だけを置き、その下にリポジトリごとの設定を並べます。リポジトリごとに、次のキーを書きます。

| キー | 書くもの |
|---|---|
| `name` | 対象のリポジトリの名前。`OWNER/REPO` の形で書く（必須） |
| `general` | [リポジトリの一般設定](#リポジトリの一般設定) |
| `actions` | [GitHub Actions の設定](#github-actions-の設定) |

`general` と `actions` は、少なくともどちらか一方を書きます。次は、一つのリポジトリでマージ後のブランチの自動削除だけを管理する、最小の例です。

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
```

真偽値は、引用符を付けない小文字の `true` か `false` で書きます。

## リポジトリの一般設定

`general` には、Pull Request のマージに関する設定を書きます。

| キー | 値 | 意味 |
|---|---|---|
| `delete_branch_on_merge` | `true` / `false` | Pull Request をマージしたあと、head ブランチを自動で削除するか |
| `allow_auto_merge` | `true` / `false` | Pull Request の自動マージを許可するか |

GitHub Free のアカウントが所有する非公開リポジトリでは自動マージを使えないため、`allow_auto_merge: true` を書いても反映されないことがあります。

## GitHub Actions の設定

`actions` には、`permissions`（Actions の実行許可）と `workflow`（ワークフローの権限）を書きます。使わないほうは省けます。

### Actions の実行許可

`actions.permissions` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `enabled` | `true` / `false` | このリポジトリで GitHub Actions を有効にするか |
| `allowed_actions` | `all` / `local_only` / `selected` | 実行を許可する Actions と再利用可能なワークフローの範囲（Actions 許可ポリシー） |
| `selected_actions` | 下の `selected_actions` の表のキー | `allowed_actions: selected` のときに許可するもの |

`allowed_actions` の値の意味は次のとおりです。

| 値 | 許可するもの |
|---|---|
| `all` | すべての Actions と再利用可能なワークフロー |
| `local_only` | ローカルの Actions と再利用可能なワークフローだけ |
| `selected` | `selected_actions` に書いた条件に合うものだけ |

`selected_actions` には、次のキーをすべて書きます。

| キー | 値 | 意味 |
|---|---|---|
| `github_owned_allowed` | `true` / `false` | GitHub が作った Actions を許可するか |
| `verified_allowed` | `true` / `false` | Marketplace で認証済みの作成者が提供する Actions を許可するか |
| `patterns_allowed` | 文字列のリスト | 許可する Actions と再利用可能なワークフローのパターン。何も許可しないときは `[]` と書く |

書くときは、次の点に気を付けます。

- `allowed_actions` を書くと、Actions も有効になる。Actions を無効のままにしておきたいリポジトリには、`allowed_actions` を書かない
- `enabled: false` と、`allowed_actions` や `selected_actions` は同時に書けない
- `selected_actions` は、`allowed_actions: selected` のときだけ書ける
- `enabled` の変更は、現在は `plan` で確かめられるだけである。この変更を含む設定ファイルで `apply` すると、承認を求めず、何も変えずにエラーで止まる
- `allowed_actions` を `selected` から `all` や `local_only` に変えると、GitHub は `selected_actions` の値を消す。消える値は変更計画の `Side effect:` の行に出る。詳しくは [Actions 許可ポリシーの切り替え](actions-policy.md) にある

### ワークフローの権限

`actions.workflow` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `default_workflow_permissions` | `read` / `write` | ワークフローの `GITHUB_TOKEN` に与える既定の権限 |
| `can_approve_pull_request_reviews` | `true` / `false` | GitHub Actions が Pull Request を作成し、承認することを許可するか |

`default_workflow_permissions` が決めるのは、ワークフローが権限を明示しないときの既定の権限です。`read` にしても、ワークフローやジョブの `permissions` で書き込み権限を明示したワークフローは書き込めます。

## 未記載の設定の扱い

reco が変えるのは、設定ファイルに値を書いた設定だけです。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: false
```

この例で reco が見るのは `allow_auto_merge` だけです。`false` は「`false` にしたい」という意味であり、「管理しない」という意味ではありません。書いていない `delete_branch_on_merge` は変えません。

設定ファイルから設定を消しても、GitHub 上の値は元に戻りません。値を変えたいときは、消すのではなく、設定したい値を書いて `apply` します。

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
      delete_branch_on_merge: true
```

- reco は、並べた順にリポジトリを更新する
- YAML のアンカー（`&`）、エイリアス（`*`）、マージキー（`<<`）は使えない。多くのリポジトリに同じ設定を書くときも、リポジトリごとに書き並べる

## よくある書き方の誤り

設定ファイルに誤りがあると、reco は GitHub にアクセスする前に、誤りの箇所（行と列）と理由を示して終了します。出会いやすい誤りは次のとおりです。

真偽値を引用符で囲むと、文字列として扱われ、拒否されます。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: "true"
```

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: repositories[0].general.allow_auto_merge (line 4, column 25): must be true or false (unquoted, lowercase); got \"true\""}
```

`allowed_actions: selected` のときに `selected_actions` のキーの一部を書き忘れると、足りないキーが示されます。

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

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: repositories[0].actions.permissions.selected_actions (line 7, column 11): missing patterns_allowed; selected_actions requires all of github_owned_allowed, verified_allowed, patterns_allowed"}
```

## References

- [YAML 設定仕様](../spec/configuration.md)：設定の型と許容値、未記載の設定の扱い（2章）、入力の組み合わせ（4.1節）、入力検証の全条件（6章）
- [CLI 仕様](../spec/cli.md)：`--config` の扱い、副作用の表示
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：Actions の各設定の意味
- [GitHub リポジトリ API 仕様](../research/repositories.md)：GitHub Free の非公開リポジトリでの自動マージ
