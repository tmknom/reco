# 設定ファイルの書き方

reco は、リポジトリごとの欲しい状態を書いた YAML の設定ファイルを読み、GitHub 上の設定と比べて、書いた値になるように設定を変えます。用語の意味は [用語集](glossary.md) にあります。

## 設定ファイルの場所と構造

reco は、既定では作業ディレクトリの `reco.yaml` を読みます。別の場所のファイルを使うときは、`reco plan --config path/to/file.yaml` のように `--config` で指定します。

設定ファイルのルートには `repositories` だけを置き、その下にリポジトリごとの宣言を並べます。各宣言は、リポジトリの名前（`name`）と、一つ以上の設定を持ちます。`name` は `OWNER/REPO` の形で書き、URL や、末尾に `.git` を付けた形は受け付けません。次は、一つのリポジトリでマージ後のブランチの自動削除だけを管理する、最小の例です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

reco が扱える設定をすべて書くと、次のようになります。`example/project-a` は、自分のリポジトリの名前に置き換えてください。

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
            - "actions/checkout@v4"
      workflow:
        default_workflow_permissions: read
        can_approve_pull_request_reviews: false
```

設定は、`general`（リポジトリの一般設定）、`actions.permissions`（Actions の実行許可）、`actions.workflow`（ワークフローの権限）の三つに分かれます。どれも、使わない部分は丸ごと書かなくてかまいません。

## 書かなかった設定はどうなるか

reco が管理するのは、設定ファイルに値を書いた設定だけです。書かなかった設定は管理範囲の外にあり、reco はその値を `false` や GitHub の既定値に補って変えることはしません。reco がこれをどう守るかは、[reco の安全性の約束](safety.md#宣言していない設定を変えない) にあります。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: false
```

この例で reco が管理するのは `allow_auto_merge` だけです。`false` は「管理しない」ではなく、「`false` にしたい」という明示した期待値です。書かなかった `delete_branch_on_merge` は、GitHub 上の値が `true` でも `false` でも、reco は差分として扱わず、変えません。

設定ファイルから設定を消しても、GitHub 上の値は元に戻りません。消した設定は、その時点の値のまま管理範囲の外に出るだけです。値を変えたいときは、消すのではなく、欲しい値を書いて適用します。

## リポジトリの一般設定

`general` には、Pull Request のマージに関する二つの設定を書きます。

- **`delete_branch_on_merge`**：Pull Request をマージしたあと、head ブランチを自動で削除するか
- **`allow_auto_merge`**：Pull Request の自動マージを許可するか

どちらもリポジトリ単位の設定であり、変えても、reco が個々の Pull Request をマージしたり、既存のブランチを削除したりすることはありません。

GitHub Free のアカウントが所有する非公開リポジトリでは自動マージを使えず、`allow_auto_merge: true` を書いても、GitHub が値を変えないことがあります。このとき GitHub は更新を受け付けたと応答しますが、reco は更新後の値が期待値と一致しないことを確かめ、`apply` は失敗（`failed`）で終わります。

## Actions の実行許可

`actions.permissions.allowed_actions` には、このリポジトリで実行を許可する Actions と再利用可能なワークフローの範囲（Actions 許可ポリシー）を、次の三つの値のどれかで書きます。

- **`all`**：すべての Actions と再利用可能なワークフローを許可する
- **`local_only`**：ローカルの Actions と再利用可能なワークフローだけを許可する。どこまでをローカルとみなすかは GitHub が決める
- **`selected`**：`selected_actions` に書いた条件に合うものだけを許可する

`allowed_actions` を書くと、そのポリシーで Actions が実行できること、つまり Actions が有効であることも期待状態に含まれます。Actions が無効のリポジトリでは、reco は書いたポリシーで Actions を有効にします。reco が Actions を無効にすることはありません。Actions を無効のままにしておきたいリポジトリには、`allowed_actions` を書かないでください。

`allowed_actions` が `all` か `local_only` のときは、`selected_actions` を書けません。`selected` のときは、`selected_actions` に次の3項目をすべて書きます。

- **`github_owned_allowed`**：GitHub が所有する Actions を許可するか
- **`verified_allowed`**：Marketplace で認証済みの作成者が提供する Actions を許可するか
- **`patterns_allowed`**：許可する Actions と再利用可能なワークフローのパターンのリスト。何も許可しないときは `[]` と書く

3項目は一つの設定グループで、reco は3項目をまとめて一つの設定として比べ、変えます。

reco は `patterns_allowed` の要素の順序を区別しません。GitHub 上の値と要素が同じなら、並びが違っても差分にはなりません。

GitHub の資料は `patterns_allowed` を公開リポジトリに適用する設定としており、非公開リポジトリで書いた場合に GitHub がどう扱うかを reco は保証しません。

## Actions 許可ポリシーを変えるときに起きること

`allowed_actions` を `selected` とほかの値のあいだで変えると、GitHub の仕様により、`selected_actions` の値が消えたり、一時的に期待値と違う値になったりします。

### `selected` から `all` や `local_only` に変えるとき

`selected` から `all` や `local_only` に変えると、GitHub は `selected_actions` の3項目の値を消します。あとで `selected` に戻しても、消える前の値は戻りません。`all` や `local_only` のときは `selected_actions` を書けないため、この3項目は管理範囲の外にあります。

変更計画では、消える値が副作用（`Side effect:` の行）として、現在の値とともに示されます。あとで同じ値に戻したいときは、この行の値を控えておきます。reco がこの副作用をどう扱うかは、[reco の安全性の約束](safety.md#宣言していない設定を変えない) にあります。

Actions が無効のリポジトリで `all` や `local_only` を書いた場合は、無効にする前のポリシーが `selected` だったなら、同じく3項目が消えます。無効のあいだは値を読み取れないため、変更計画には消える可能性があることだけが示されます。

### `all` や `local_only` から `selected` に変えるとき

`selected_actions` は、`allowed_actions` が `selected` になるまで GitHub に書き込めません。そのため reco は、先に `allowed_actions` を `selected` にし、そのあとで `selected_actions` を書いた値にするという、二つの更新操作を順に行います。Actions が無効のリポジトリで `selected` を書いた場合も同じです。

二つの更新操作のあいだは中間状態になり、少なくとも1秒続きます。Actions が有効なリポジトリで `all` や `local_only` から変えるときは、中間状態の `selected_actions` は GitHub が決めた値（`github_owned_allowed: true`、`verified_allowed: false`、`patterns_allowed: []`）になります。この値は、宣言していない許可を一時的に生むことがあります。たとえば `local_only` から、`github_owned_allowed: false` の `selected` に変える場合、変更前も変更後も許可しない GitHub 所有の Actions が、中間状態のあいだだけ許可されます。

Actions が無効のリポジトリから変えるとき、無効にする前のポリシーが `selected` だったなら、中間状態の `selected_actions` は、`github_owned_allowed` が `true` になるほかは無効にする前の値のままです。この場合も、宣言していない許可が一時的に生まれることがあります。

二つの更新操作のあいだで `apply` が止まったときに中間状態が残ることと、そのときにすることは、[reco の安全性の約束](safety.md#二つの更新操作からなる変更の途中で止まると中間状態が残る) にあります。

## ワークフローの権限

`actions.workflow` には、ワークフローに与える権限を書きます。

- **`default_workflow_permissions`**：ワークフローの `GITHUB_TOKEN` に与える既定の権限。`read` か `write` のどちらか
- **`can_approve_pull_request_reviews`**：GitHub Actions が Pull Request を作成し、承認することを許可するか。GitHub の設定画面の「Allow GitHub Actions to create and approve pull requests」に当たる

`default_workflow_permissions` が決めるのは、ワークフローが権限を明示しないときの既定の権限です。`read` にしても、ワークフローやジョブの `permissions` で書き込み権限を明示したワークフローは書き込めます。

## 複数のリポジトリを並べる

複数のリポジトリを管理するときは、`repositories` の下にリポジトリごとの宣言を並べます。リポジトリごとに、管理する設定を別々に選べます。

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

並べた順が、reco が変更計画を示す順であり、承認後に更新する順です。一つのリポジトリの中の設定は、書いた順ではなく、reco が決めた順に示し、更新します。

YAML のアンカー（`&`）とエイリアス（`*`）は使えません。reco は、リポジトリごとの宣言をほかのリポジトリと共有したり継承したりすることを認めていないためです。多くのリポジトリに同じ設定を書くときも、リポジトリごとに書き並べる必要があります。

## よくある書き方の誤り

設定ファイルに誤りがあると、reco は GitHub にアクセスする前に、1行の JSON のエラーを示して終了します。`error` の値に、誤りの箇所（行と列）と理由が入ります。出会いやすい誤りは次の二つです。

真偽値を引用符で囲むと、文字列として扱われ、拒否されます。真偽値は、引用符を付けない小文字の `true` か `false` で書きます。

```yaml
repositories:
  - name: example/project-a
    general:
      allow_auto_merge: "true"
```

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: repositories[0].general.allow_auto_merge (line 4, column 25): must be true or false (unquoted, lowercase); got \"true\""}
```

`allowed_actions: selected` のときに `selected_actions` の3項目の一部を書き忘れると、足りない項目が示されます。

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

受け付ける入力の全条件は、References の YAML 設定仕様にあります。

## References

- [YAML 設定仕様](../spec/configuration.md)：設定の型と許容値、管理範囲の考え方、入力の組み合わせ（4.1節）、入力検証の全条件（6章）
- [CLI 仕様](../spec/cli.md)：`--config` の扱い、副作用の表示、`apply` の結果の区分
- [アーキテクチャ設計](../design/architecture.md)：Actions 許可ポリシーを変えるときの更新操作の順序と中間状態（5.4節）
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：Actions の各設定の意味、設定画面の項目名、Actions が無効のリポジトリでの中間状態の値
- [GitHub リポジトリ API 仕様](../research/repositories.md)：GitHub Free の非公開リポジトリでの自動マージ
