# Operations

## リポジトリを管理に加える

新しいリポジトリを設定ファイルに足す前に、`show` でそのリポジトリの現在の設定を見ます。`show` が設定を表示できれば、reco がそのリポジトリを読み取れることも確かめられます。

```bash
reco show --repo example/project-d
```

`show` の表示は設定ファイルと同じ構造です。表示の `repositories` の下の要素から、管理したい設定の行を残して値を書き換え、設定ファイルの `repositories` の下に書き足します。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true

  - name: example/project-d
    general:
      delete_branch_on_merge: true
```

`--repo` で足したリポジトリを指定して `plan` を実行し、変更計画が意図どおりかを確かめます。

```text
$ reco plan --repo example/project-d
Configuration: ./reco.yaml

Repository: example/project-d
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

意図どおりなら、`reco apply --repo example/project-d` で、足したリポジトリだけに適用します。

## 一つのリポジトリに絞って実行する

`plan` と `apply` に `--repo OWNER/REPO` を付けると、設定ファイルに書いたリポジトリのうち一つだけを対象にできます。reco が GitHub から読み取るのも、そのリポジトリだけです。ほかのリポジトリを権限の不足などで読み取れなくても、対象にしたリポジトリの `plan` と `apply` は実行できます。`--repo` を付けても、設定ファイルのどこかに書き方の誤りがあれば止まります。

## リポジトリを並べる順序

reco は、設定ファイルに並べた順にリポジトリを更新し、あるリポジトリの変更が完了しなかったら、それより後ろのリポジトリは更新しません。先に更新したいリポジトリを前に書きます。

途中で止まった `apply` のあとにもう一度 `apply` すると、reco は今の値から変更計画を作り直すため、残りの変更が対象になります。

## 定期的に差分を確かめる

GitHub の設定画面などで設定が変えられると、設定ファイルと GitHub の値が食い違います。定期的に `plan` を実行すると、この食い違いに気付けます。

```text
$ reco plan
No changes.
Repositories checked: 3
```

`Repository:` の行が出たら、そのリポジトリに食い違いがあるか、今の値を確かめられない設定があります。食い違いがある設定は、GitHub 側の値を残したいなら設定ファイルをその値に書き直し、設定ファイルの値にしたいなら `apply` します。

`plan` が比べるのは、設定ファイルに書いた設定だけです。書いていない設定が変わっても、変更計画には出ません。

## 変えた値を戻す

reco で変えた設定を元の値に戻すには、設定ファイルにその設定の元の値を書き、`apply` します。設定ファイルから行を消すだけでは、GitHub の値は戻りません。

戻す可能性がある変更は、承認の前に、変更計画で `->` の前に出ている元の値を控えておきます。`->` の前が括弧で示された変更は、reco がその値を読めていないため、変更計画からは控えられません。

書いた設定を変えると GitHub が一緒に変える、書いていない設定の値（`allowed_actions` を `selected` から変えると消える `selected_actions` など）は、`->` の前ではなく、変更計画の `Side effect:` の行に出ます。

## 管理をやめる

ある設定を reco で管理しないようにするには、設定ファイルからその設定の行を消します。リポジトリ全体の管理をやめるときは、`name` を含むそのリポジトリの設定を丸ごと消します。

- `general:` などの下に設定が残らなくなるときは、`general:` などの行も消す
- `allowed_actions` をやめるときは、`selected_actions` もまとめて消す

消しても、GitHub の値はその時点のまま変わりません。値を戻してから管理をやめたいときは、先に元の値を書いて `apply` し、そのあとで消します。

`environments` から名前を消しても、GitHub 上の Environment は残ります。reco が作成した Environment も同じです。不要な Environment は、`environments` からその Environment を（変数の行も含めて）消してから、GitHub の「Environments」ページで消します。`environments` に名前が残っていると、次の `apply` で作成し直されます。

Repository Variables と Environment Variables も、行を消しただけでは GitHub 上に残ります。変数を消すときは、行を消す前に `value` を `state: absent` に書き換えて `apply` します。`plan` が消える値を示すので、戻す可能性があるなら控えておきます。消えたことを確かめたら、その行を消して管理をやめられます（[Configuration](configuration.md) の Repository Variables）。

## References

- [CLI 仕様](../spec/cli.md)：`apply` が今の値から変更計画を作り直すこと（2.1節）、`--repo` で選べるリポジトリと、設定ファイル全体の検証（2.2節、2.3節）、変更計画の表示（4章）、更新の順序と、完了しなかったリポジトリのあとの扱い（5.1節）、`show`（6章）
- [要求定義](../spec/requirements.md)：設定ファイルに書いたリポジトリだけを対象にすること（FR-002）、異常のあとに更新を続けないこと（INV-005）
- [YAML 設定仕様](../spec/configuration.md)：未記載の設定を管理しないこと（2章）、空の階層を拒否すること（1章、10.1節）、`allowed_actions` と `selected_actions` の組み合わせ（4.1節）、存在しないことの宣言による変数の削除（7.3節）、Environment を削除しないこと（8章）
