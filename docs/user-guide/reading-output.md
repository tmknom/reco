# 変更計画と変更結果の読み方

`plan` が表示する変更計画（`apply` が承認を求める前に表示するものも同じ形）と、`apply` の変更結果について、各行が何を意味するかを示します。用語の意味は [用語集](glossary.md) にあります。

reco の出力は、確かめた事実と確かめられなかったことを分けて示し、変更結果では更新操作の結果と期待状態の確認結果も別々の行に示します。このため、一つの行だけで判断せず、同じ変更の行を合わせて読みます。

## 行の一覧

| 行 | 説明のあるセクション |
|---|---|
| `Configuration:`、`Repository:` | [変更計画の構成](#変更計画の構成) |
| `<設定>: <現在値> -> <期待値>` | [確認済みの差分](#確認済みの差分) |
| `Current: unavailable (...)`、`Desired:` | [比較不能な設定予定](#比較不能な設定予定) |
| `Side effect:` | [副作用](#副作用) |
| `Plan:`、`Repositories checked:`、`No changes.` | [変更計画の集計](#変更計画の集計) |
| `updated`、`failed`、`not attempted`、`no change`、`Apply complete.`、`Repositories updated:` | [`apply` の結果の区分](#apply-の結果の区分) |
| ` + ` でつないだ設定の名前 | [結果が論理的な設定変更ごとに分かれるとき](#結果が論理的な設定変更ごとに分かれるとき) |
| `Update request`、`Desired state` | [更新操作の結果と期待状態の確認結果](#更新操作の結果と期待状態の確認結果) |
| `Precondition:`、`Stopped:` | [更新を始めなかった理由](#更新を始めなかった理由) |
| `Other settings:`、`Possible cause:` | [原因を断定しない行](#原因を断定しない行) |
| `Note:` | [注記](#注記) |

## 変更計画の構成

次は、三つのリポジトリを設定ファイルに書き、`example/project-b` が変更不要だった場合の `plan` です。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Repository: example/project-c
  actions.workflow.default_workflow_permissions: write -> read

Plan: 2 repositories to update, 2 known changes, 0 not-comparable settings
Repositories checked: 3
```

- **`Configuration:` の行**：読んだ設定ファイルのパス
- **`Repository:` の節**：変更予定のあるリポジトリごとに一つあり、設定ファイルに書いた順に並ぶ。変更不要のリポジトリは節を持たない
- **節の中の行**：変更予定ごとに、設定の名前から始まる。並びは設定ファイルに書いた順ではなく、`general`、`actions.permissions`、`actions.workflow` の順である
- **最後の2行**：集計（[変更計画の集計](#変更計画の集計)）

## 確認済みの差分

`<設定>: <現在値> -> <期待値>` の行は、GitHub から読み取った値（左）が設定ファイルに書いた期待値（右）と違うと確かめた設定を示します。承認すると、reco はこの設定を右の値にする更新を行います。

`actions.permissions.selected_actions` は3項目をまとめた一つの設定（設定グループ）で、値が長いため、現在値と、`->` に続く期待値を別の行に示します。

```text
Repository: example/project-a
  actions.permissions.selected_actions:
    github_owned_allowed: true, verified_allowed: false, patterns_allowed: []
    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["actions/checkout@v4", "example/*"]
```

## 比較不能な設定予定

GitHub は、リポジトリの状態によって一部の設定の値を返しません。値を読み取れなかった（観測不能の）設定は、`Current: unavailable (<理由>)` と `Desired: <期待値>` の行で示されます。reco は読み取れなかった値を推測せず、期待値と同じかどうかも判断しません。

| 理由 | 意味 |
|---|---|
| `GitHub Actions is disabled` | リポジトリの Actions が無効のため、`allowed_actions` と `selected_actions` を読み取れない |
| `allowed_actions is not "selected"` | `allowed_actions` が `selected` でないため、`selected_actions` を読み取れない |

比較不能な設定予定も変更予定であり、承認すると、reco はその設定を期待値にする更新を行います。よく出会うのは、`allowed_actions` を `selected` に変える変更予定と並んで、`selected_actions` が比較不能として示される場合です。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: all -> selected
  actions.permissions.selected_actions
    Current: unavailable (allowed_actions is not "selected")
    Desired: github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["example/*"]
```

`Desired:` の値に `(enables GitHub Actions)` が添えられていれば、Actions が無効のリポジトリで、その変更によって Actions が有効になることを示します。

`show` の `unavailable` も、同じ理由で値を読み取れなかったことを示します。権限の不足などで読み取りそのものに失敗したときは、`unavailable` とは示さず、エラーで止まります。

## 副作用

`Side effect:` の行は、宣言した変更を行うと、GitHub が未管理設定の値を消すことを示します。現在は、Actions 許可ポリシーを `all` か `local_only` に変えて、`selected_actions` の値が消える場合だけです（[設定ファイルの書き方](configuration.md#selected-から-all-や-local_only-に変えるとき)）。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: selected -> all
    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["example/*"])
```

括弧の中は、消える設定の値を、変更計画を作った時点で読み取ったものです。同じ値に戻す可能性があるなら、承認する前にこの値を控えておきます。控えた値で戻す手順は [変えた値を戻す](operations.md#変えた値を戻す) にあります。

Actions が無効のリポジトリでは値を読み取れないため、`Side effect: GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (current values are unavailable)` のように、消える可能性だけが示されます。

## 変更計画の集計

| 行 | 意味 |
|---|---|
| `Plan:` の `repositories to update` | 変更予定のあるリポジトリの数 |
| `Plan:` の `known changes` | 確認済みの差分の数。比較不能な設定予定は含まない |
| `Plan:` の `not-comparable settings` | 比較不能な設定予定の数 |
| `Repositories checked:` | 対象にしたリポジトリの数。変更不要のリポジトリも含む |
| `No changes.` | 対象にしたすべてのリポジトリで、管理設定が期待値と一致すると確かめた。比較不能な設定予定が一つでもあれば出ない |

`known changes` と `not-comparable settings` は、設定の数を数えます（`selected_actions` の3項目は一つ）。reco が GitHub に送る更新要求の数や、`apply` の変更結果の行の数とは一致しないことがあります。

## `apply` の結果の区分

`apply` は、承認のあと `Applying changes...` に続けて、対象にしたすべてのリポジトリについて、変更結果か変更不要かを、設定ファイルに書いた順に示します。

```text
Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated
  example/project-b: no change
  example/project-c: updated

Apply complete.
Repositories updated: 2
```

| 表示 | 用語 | 読み違えやすい点 |
|---|---|---|
| `updated` | 完了 | GitHub が更新を受け付けただけでは出ない。完了とする条件は [reco の安全性の約束](safety.md#反映を確かめてから完了と報告する) にある |
| `failed` | 失敗 | GitHub の設定が変わっていないことを意味しない。更新を受け付けられたあとの確認で止まったものや、受け付けられたかどうか分からないものを含む |
| `not attempted` | 未実行 | その変更について、GitHub の設定を変える要求を送っていない。同じ `apply` のほかの変更は反映されていることがある |
| `no change` | 変更不要 | 変更計画を作ったときに確かめた事実で、更新を伴わない。先のリポジトリが完了しなかった場合も `no change` と示す |

`failed` と `not attempted` には、理由の行が続きます（[更新操作の結果と期待状態の確認結果](#更新操作の結果と期待状態の確認結果)、[更新を始めなかった理由](#更新を始めなかった理由)）。これらが出たあとにすることは [`apply` が途中で止まった](troubleshooting.md#apply-が途中で止まった) にあります。

`Apply complete.` と `Repositories updated:` は、すべてのリポジトリが `updated` か `no change` のときだけ出ます。`Repositories updated:` は `updated` のリポジトリの数で、`no change` は数えません。

## 結果が論理的な設定変更ごとに分かれるとき

一つのリポジトリに論理的な設定変更が二つ以上あり、完了しなかったものを含むときは（先のリポジトリが完了しなかったために一つも始めなかった場合を除く）、リポジトリの行に結果を示さず、その下に論理的な設定変更ごとの結果を実行した順に並べます。完了しなかったものには、理由の行が続きます。

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    actions.permissions.allowed_actions + actions.permissions.selected_actions: failed
      Update request (actions.permissions.allowed_actions): succeeded
      Update request (actions.permissions.selected_actions): rejected
      Desired state: does not match
    actions.workflow.default_workflow_permissions: not attempted
      Stopped: a preceding change did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

` + ` でつないだ名前は、二つの管理設定を一緒に変える、一つの論理的な設定変更です。`allowed_actions` を `selected` に変える変更と `selected_actions` を期待値にする変更は、変更計画では2行の変更予定ですが、変更結果では一つにまとまります。

それ以外のときは、リポジトリの行に結果を一つだけ示します。

## 更新操作の結果と期待状態の確認結果

`failed` の変更には、更新操作の結果（`Update request`）と、更新のあとに読み取り直した設定が期待状態と一致したか（`Desired state`）が、別の行で示されます。

```text
Applying changes...
  example/project-a: failed
    Update request: unknown
    Desired state: matches (update outcome still unknown)

Note: No automatic retry was performed.
```

| `Update request` の値 | 意味 |
|---|---|
| `succeeded` | GitHub が更新要求を受け付けた |
| `rejected` | GitHub が更新要求を拒否した |
| `unknown` | 成否不明。GitHub が受け付けたとも拒否したとも確かめられなかった。原因は、変更結果のあとに stderr へ出るエラーの行に示される |
| `not started` | 二つの更新操作からなる変更で、その更新操作を始めなかった |

二つの更新操作からなる変更では、`Update request (<設定>): <値>` の形で、更新操作ごとに1行ずつ、実行した順に示されます。

| `Desired state` の値 | 意味 |
|---|---|
| `matches` | 更新のあとの設定が期待状態と一致した |
| `does not match` | 更新のあとの設定が期待状態と一致しなかった |
| `not confirmed (<理由>)` | 一致したかを確かめられなかった。理由が `unavailable` に添える理由と同じ文言なら、GitHub が値を返さなかったことを示す |
| `not checked` | 最初の更新操作が拒否されたため、確かめなかった |

`Desired state: matches` でも `failed` になることがあります。更新操作が成否不明だったか、二つ目の更新操作を完了できなかったか、`Other settings:` の行が示す変化があった場合です。成否不明のあとの `matches` には `(update outcome still unknown)` が添えられ、reco はこの変更を完了として扱いません（[反映を確かめてから完了と報告する](safety.md#反映を確かめてから完了と報告する)）。

## 更新を始めなかった理由

`Precondition:` と `Stopped:` は、reco が更新操作を始めなかった理由を示します。どちらも、reco が安全のために止まったことを表します（[承認のあとで状況が変わったら止まる](safety.md#承認のあとで状況が変わったら止まる)、[異常を見つけたら、それ以上更新しない](safety.md#異常を見つけたらそれ以上更新しない)）。

| 行 | 意味 |
|---|---|
| `Precondition: <設定> changed since approval` | 変更前提が成り立たなかった。承認したあと、更新の直前までに、その設定の値が変わっていた。`Side effect:` で示した設定の値の変化や、読み取れなかった値が読み取れるようになったこと（その逆も）を含む。複数の設定が変わっていても一つだけ示す |
| `Precondition: <設定> is not the value expected after the preceding update` | 二つの更新操作からなる変更で、一つ目の更新操作のあと、その設定が予定した値になっていなかったため、二つ目を始めなかった。GitHub への反映の遅れか、ほかの人による変更かを、reco は断定しない |
| ほかの `Precondition:` の行 | 変更前提を確かめる読み取りに失敗したか、対象のリポジトリが承認したものと別のものになっていた。更新操作のあいだの待ち時間に Ctrl-C で中断したときも、読み取りに失敗したときと同じ行が出る |
| `Stopped: a preceding change did not complete` | 同じリポジトリで先に実行した変更が完了しなかったため、始めなかった |
| `Stopped: a preceding repository did not complete` | 先のリポジトリが完了しなかったため、このリポジトリの変更を一つも始めなかった |

`Precondition:` の行が `Update request (<設定>): not started` の行の下にあるときは、一つ目の更新操作が終わっているため、変更全体は `not attempted` ではなく `failed` です。このとき GitHub には中間状態が残っていることがあります（[二つの更新操作からなる変更の途中で止まると、中間状態が残る](safety.md#二つの更新操作からなる変更の途中で止まると中間状態が残る)）。

```text
Applying changes...
  example/project-a: failed
    Update request (actions.permissions.allowed_actions): succeeded
    Update request (actions.permissions.selected_actions): not started
    Precondition: actions.permissions.allowed_actions is not the value expected after the preceding update
    Desired state: does not match

Note: Some changes may already have been applied. No automatic rollback was performed.
```

`Precondition:` の行が出たあとにすることは [承認のあとで状況が変わったと報告された](troubleshooting.md#承認のあとで状況が変わったと報告された) にあります。

## 原因を断定しない行

`Other settings:` と `Possible cause:` は、どちらも reco が原因を断定せずに示す行です。

`Other settings: <設定> changed <前の値> -> <後の値> (cause not determined)` は、変更対象外の設定の予期しない変化を示します。同じリポジトリのほかの管理設定のことも、reco の設定ファイルに書けない設定（`actions.permissions.sha_pinning_required`）のこともあります。reco の更新が原因とは限らず、更新の前後のあいだに、ほかの人が変えた可能性もあります。

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: matches
    Other settings: actions.permissions.sha_pinning_required changed false -> true (cause not determined)

Note: Some changes may already have been applied. No automatic rollback was performed.
```

この行が出たあとにすることは [変えていない設定が変わったと報告された](troubleshooting.md#変えていない設定が変わったと報告された) にあります。

`Possible cause:` は、更新要求が受け付けられたのに期待状態と一致しなかった設定に、既知の原因の候補があるとき、その候補を可能性として示します。現在、候補を示すのは `general.allow_auto_merge` だけで、候補の内容は [設定ファイルの書き方](configuration.md#リポジトリの一般設定) にあります。reco は候補が当てはまるかを確かめていません。この行が出たあとにすることは [適用したのに反映されない](troubleshooting.md#適用したのに反映されない) にあります。

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: does not match
    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it

Note: Some changes may already have been applied. No automatic rollback was performed.
```

## 注記

更新操作を始めた変更（先に完了したリポジトリの変更を含む）があり、完了しなかった変更もあるときは、変更結果の最後に注記が出ます。

| 注記 | 出る条件 |
|---|---|
| `Note: Some changes may already have been applied. No automatic rollback was performed.` | 始めた更新操作に、成否不明のものがない |
| `Note: No automatic retry was performed.` | 始めた更新操作に、成否不明のものがある |

どちらの注記が出たときも、この `apply` の変更が GitHub に残っている可能性があります。reco は反映した変更を元に戻さず、成否不明の更新要求を送り直していません（[reco が引き受けないこと](safety.md#reco-が引き受けないこと)）。

注記も `Apply complete.` も出ていなければ、その `apply` は更新操作を一つも始めていません。

## References

- [CLI 仕様](../spec/cli.md)：変更計画の表示と集計（4章）、変更結果の表示区分と各行（5.2節）、異常時の表示例（5.3節）、`show` の `unavailable`（6章）
- [要求定義](../spec/requirements.md)：確かめた事実と分からないことを区別して示すこと（QR-002）、比較不能でも変更予定として扱うこと（FR-006、FR-011）
