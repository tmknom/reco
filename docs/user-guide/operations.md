# 複数リポジトリの運用

reco で管理するリポジトリを増やしたり、日常的に使ったりするときの手順を、作業ごとに示します。一つのリポジトリでの `show`、`plan`、`apply` の流れは [はじめての適用](getting-started.md) にあります。用語の意味は [用語集](glossary.md) にあります。

## リポジトリを管理に加える

新しいリポジトリを設定ファイルに足す前に、`show` でそのリポジトリの現在の設定を見ます。

```bash
reco show --repo example/project-d
```

`show` が設定を表示できれば、reco がそのリポジトリを読み取れることも確かめられます。読み取れないリポジトリを設定ファイルに足すと、`--repo` を付けない `plan` と `apply` は、ほかのリポジトリも含めて止まります（[全リポジトリを確かめてから更新を始める](safety.md#全リポジトリを確かめてから更新を始める)）。

`show` の値を見て、欲しい値を `repositories` の下に書き足します。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true

  - name: example/project-d
    general:
      delete_branch_on_merge: true
```

`plan` で、足したリポジトリの変更予定が意図どおりかを確かめます。`--repo` で足したリポジトリを指定すると、そのリポジトリの変更計画だけが表示されます。

```text
$ reco plan --repo example/project-d
Configuration: ./reco.yaml

Repository: example/project-d
  general.delete_branch_on_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
Repositories checked: 1
```

意図どおりなら、`reco apply --repo example/project-d` で、足したリポジトリだけに適用します。`--repo` を付けずに `apply` すると、ほかのリポジトリに変更予定があれば、それも同じ承認の対象になります。

## 一つのリポジトリに絞って実行する

`plan` と `apply` に `--repo OWNER/REPO` を付けると、設定ファイルに書いたリポジトリのうち一つだけを対象にできます。指定できるリポジトリの条件は [コマンドリファレンス](commands.md#オプション) にあります。

reco が GitHub から設定を読み取るのは、対象にしたリポジトリだけです。このため、ほかのリポジトリが読み取りに失敗する状態でも、対象にしたリポジトリの `plan` と `apply` は進められます。

ほかのリポジトリの宣言に書き誤りがあると、`--repo` を付けても止まるため、先に誤りを直します。

## リポジトリを並べる順序

reco は、設定ファイルに並べた順にリポジトリを一つずつ更新します（[複数のリポジトリを並べる](configuration.md#複数のリポジトリを並べる)）。あるリポジトリの変更が完了しなかったら、それより後ろに並ぶ、変更予定のあるリポジトリは更新されず、未実行（`not attempted`）のまま終わります（[異常を見つけたら、それ以上更新しない](safety.md#異常を見つけたらそれ以上更新しない)）。

更新の順序は、この並びだけで決まります。先に更新したいリポジトリ（たとえば変更を急ぐものや、影響を確かめたいもの）を前に書きます。

途中で止まった `apply` のあとにもう一度 `apply` すると、完了したリポジトリは、ほかの手段で変えられていなければ変更不要（`no change`）になり、残りのリポジトリが対象になります。止まった原因を確かめ、取り除いてから実行し直します。

## 定期的に差分を確かめる

GitHub の設定画面やほかの道具で管理設定が変えられると、設定ファイルと GitHub の値が食い違います。定期的に `plan` を実行すると、この食い違いに気付けます。

```text
$ reco plan
No changes.
Repositories checked: 3
```

`No changes.` なら、管理設定はすべて設定ファイルのとおりです。`Repository:` の節が出たら、そのリポジトリには、確認済みの差分か比較不能な設定予定があります。確認済みの差分は、GitHub 側の値を残したいなら設定ファイルをその値に書き直し、設定ファイルの値にしたいなら `apply` します。

`plan` が比べるのは管理設定だけです（[書かなかった設定はどうなるか](configuration.md#書かなかった設定はどうなるか)）。設定ファイルに書いていない設定が GitHub 側で変わっても、変更計画には出ません。

変更予定の有無は、終了コードではなく出力を読んで判断します（[自動化に組み込むとき](commands.md#自動化に組み込むとき)）。

## 変えた値を戻す

reco で変えた設定を元の値に戻すには、設定ファイルにその設定の元の値を書き、`apply` します。設定ファイルから行を消すだけでは、GitHub の値は戻りません（[書かなかった設定はどうなるか](configuration.md#書かなかった設定はどうなるか)）。

確認済みの差分なら、元の値は、変更する前の `plan` か `apply` の変更計画に、GitHub から読み取った値として示されています。戻す可能性がある変更は、承認の前にこの値を控えておきます。

`allowed_actions` を `selected` から `all` や `local_only` に変えたときに GitHub が消した `selected_actions`（[`selected` から `all` や `local_only` に変えるとき](configuration.md#selected-から-all-や-local_only-に変えるとき)）を戻すには、その変更計画の `Side effect:` の行に出ていた3項目の値（[副作用](reading-output.md#副作用)）を、`allowed_actions: selected` とともに書いて `apply` します。

```yaml
repositories:
  - name: example/project-a
    actions:
      permissions:
        allowed_actions: selected
        selected_actions:
          github_owned_allowed: true
          verified_allowed: true
          patterns_allowed:
            - "example/*"
```

この変更は二つの更新操作からなり、そのあいだに中間状態が生じます（[`all` や `local_only` から `selected` に変えるとき](configuration.md#all-や-local_only-から-selected-に変えるとき)）。

## 管理をやめる

ある設定を reco で管理しないようにするには、設定ファイルからその設定の行を消します。リポジトリ全体の管理をやめるときと、そのリポジトリに残った最後の管理設定をやめるときは、`name` を含むそのリポジトリの宣言を丸ごと消します。

`general:` などの下に設定が残らなくなるときは、`general:` などの行も消します。`allowed_actions` をやめるときは、`selected_actions` もまとめて消します。中身のない `general:` などや、`allowed_actions: selected` を伴わない `selected_actions` は、設定ファイルの誤りとして拒否されるためです。

設定の行を消すときも宣言を丸ごと消すときも、GitHub の値はその時点のまま変わりません。値を戻してから管理をやめたいときは、先に [変えた値を戻す](#変えた値を戻す) の手順で元の値を適用し、そのあとで消します。

消したあとに `plan` を実行し、消したリポジトリが `Repositories checked:` の数に含まれなくなったことや、消した設定が変更計画に出ないことを確かめます。

## ほかの人や自動化と同時に変えるとき

同じリポジトリの設定を、ほかの人やワークフローなどの仕組みが変えることがあります。`apply` の最中にそれが起きると、reco は変更前提が成り立たないことや、変更対象外の設定の予期しない変化を見つけて、止まることがあります（[承認のあとで状況が変わったら止まる](safety.md#承認のあとで状況が変わったら止まる)、[反映を確かめてから完了と報告する](safety.md#反映を確かめてから完了と報告する)）。止まったことを示す行の意味は、[更新を始めなかった理由](reading-output.md#更新を始めなかった理由) と [原因を断定しない行](reading-output.md#原因を断定しない行) にあります。

`apply` のあいだは、同じリポジトリの設定をほかの手段で変えないようにします。止まったときは `plan` を実行し直し、今の値を確かめてから、設定ファイルと GitHub のどちらの値を残すかを決めます。

reco が `apply` の中で見つけられない変更もあります（[ほかの人や自動化による同時の変更は防げない](safety.md#ほかの人や自動化による同時の変更は防げない)）。`apply` のあとに `plan` を実行すると、管理設定についてはその変更にも気付けます。

## References

- [CLI 仕様](../spec/cli.md)：`--repo` で選べるリポジトリと、設定ファイル全体の検証（2.2節、2.3節）、変更計画の表示と `plan` の終了コード（4章）、更新の順序と、完了しなかったリポジトリのあとの扱い（5.1節）、`not attempted` と `Precondition:`（5.2節）、`show`（6章）、終了コード（7章）
- [要求定義](../spec/requirements.md)：設定ファイルに書いたリポジトリだけを対象にすること（FR-002）、全リポジトリの事前検証（INV-002）、変更前提の確認（INV-004）、異常のあとに更新を続けないこと（INV-005）
- [YAML 設定仕様](../spec/configuration.md)：書かなかった設定を管理しないこと（2章）、空の階層を拒否すること（1章、6.1節）、`allowed_actions` と `selected_actions` の組み合わせ（4.1節）
- [アーキテクチャ設計](../design/architecture.md)：`selected` に変えるときの二つの更新操作と中間状態（5.4節）
