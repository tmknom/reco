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

## リポジトリの一般設定

`general` には、Pull Request のマージに関する設定を書きます。

| キー | 値 | 意味 |
|---|---|---|
| `delete_branch_on_merge` | `true` / `false` | Pull Request をマージしたあと、head ブランチを自動で削除するか |
| `allow_auto_merge` | `true` / `false` | Pull Request の自動マージを許可するか |

> [!NOTE]
> GitHub の仕様上、GitHub Free のアカウントが所有するプライベートリポジトリでは、自動マージは使えません。

## GitHub Actions の設定

`actions` には、`permissions`（Actions の実行許可）と `workflow`（ワークフローの権限）を書きます。使わないほうは省けます。

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
- `enabled: false` で Actions を無効にしても、GitHub は `allowed_actions`、`selected_actions`、`sha_pinning_required` の値を残す。あとで `enabled: true` に戻すと、無効にする前の設定で有効になる。再開するときの Actions 許可ポリシーを自分で決めたいときは、`enabled: true` の代わりに `allowed_actions` を書く
- `allowed_actions` を書くと、Actions も有効になる
- `allowed_actions` を `selected` から `all` や `local_only` に変えると、GitHub は `selected_actions` の値を消す
- Actions を無効のまま送った `sha_pinning_required` の値は GitHub が保存しないため、Actions が無効のリポジトリで `sha_pinning_required` を書くときは、`enabled: true` か `allowed_actions` も書く。どちらも書かないと、`plan` と `apply` はその変更に `Applicability: not applicable` を添え、`Plan cannot be applied.` を示す。`apply` は何も変えずにエラーで終了する
- `sha_pinning_required` の変更を含むと、`apply` はその変更にまだ対応していないため、何も変えずにエラーで終了する。`plan` では変更を確認できる

### ワークフローの権限

`actions.workflow` には、次のキーを書きます。

| キー | 値 | 意味 |
|---|---|---|
| `default_workflow_permissions` | `read` / `write` | ワークフローの `GITHUB_TOKEN` に与えるデフォルト権限 |
| `can_approve_pull_request_reviews` | `true` / `false` | GitHub Actions が Pull Request を作成し、承認することを許可するか |

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
