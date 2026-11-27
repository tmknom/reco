# General の設計

設定宣言の `general`（[YAML 設定仕様](../../spec/configuration.md) 3章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| リポジトリ一般設定 | General | Pull Request の Auto-merge の許可やマージ後のブランチ自動削除など、リポジトリ全般の振る舞いに関する設定の領域 |
| マージ方式 | Merge Method | Pull Request をマージする方法の許可。squash merge（`allow_squash_merge`）、merge commit（`allow_merge_commit`）、rebase merge（`allow_rebase_merge`）の3つ |
| 更新後のマージ方式の値 | Merge Methods After Update | 変更計画を適用したあとの各マージ方式の値。宣言したマージ方式は期待値、宣言していないマージ方式は計画時の観測値 |
| 無効のまま残るマージ方式 | Merge Method Left Disabled | 宣言されておらず、計画時に無効だったマージ方式。更新後にすべてのマージ方式が無効になる変更予定の、適用不能の理由に添える |
| マージメッセージ | Commit Message Default | squash merge と merge commit のそれぞれについて、GitHub がコミットメッセージの既定にするタイトル（title）と本文（message）の選び方の組。squash merge は `squash_merge_commit_title` と `squash_merge_commit_message`、merge commit は `merge_commit_title` と `merge_commit_message`。本文は組の一部で、マージメッセージ全体ではない。rebase merge にはない |
| 対応するマージ方式 | Corresponding Merge Method | マージメッセージごとに一つ決まるマージ方式。squash merge のマージメッセージには `allow_squash_merge`、merge commit のマージメッセージには `allow_merge_commit` |
| リポジトリの機能 | Repository Feature | リポジトリの Issues（`has_issues`）、Projects（`has_projects`）、Wiki（`has_wiki`）を有効にするかの3設定 |
| Web コミットのサインオフ必須 | Web Commit Signoff Requirement | GitHub の Web 画面からのコミットに、作成者のサインオフを求めるかの設定（`web_commit_signoff_required`）。コミットの署名を求める設定とは異なる |

General の管理の単位は、単独の10設定と、二つのマージメッセージです。3つのマージ方式は設定グループではなく、一部だけを宣言できます。`allow_update_branch` はマージ方式に数えません。マージメッセージは、squash merge と merge commit のそれぞれがタイトルと本文からなる設定グループで、省略できる項目はありません。GitHub が更新で受け付けるタイトルと本文の組み合わせは限られ（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）、受け付けない組み合わせの宣言は入力検証で拒否します（[ドメインモデル](../domain.md) の照合結果と適用可能性）。リポジトリの機能の3設定と Web コミットのサインオフ必須は、互いにも、ほかの General の設定とも依存しない単独の設定で、一部だけを宣言できます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

General は、どの観測も最初に送る `GET /repos/{owner}/{repo}`（[GitHub API 通信の設計](../github-access.md) 2.2節）の応答から読むため、どの観測も General を含みます。取得条件を持たず、General の照合結果に比較不能が生じるのは、マージメッセージが知らない値のときと、Web コミットのサインオフ必須の値が返らなかったとき（どちらも後述）だけです。

トークン所有者が Write 以上のロールを持たないリポジトリでは、応答から、マージ方式、`allow_update_branch`、`delete_branch_on_merge`、`allow_auto_merge` の6設定と、マージメッセージの4項目がそろって欠けます（[GitHub リポジトリ API 仕様](../../research/repositories.md) 1.1）。この欠落は取得条件による観測不能にせず、観測操作の失敗とします。原因は権限の不足であり、観測不能として `plan` を進めても、同じ権限では更新できないためです。

`has_issues`、`has_projects`、`has_wiki`、`web_commit_signoff_required` は、Write 以上のロールを持たないリポジトリでも値とともに返ります（同 1.1）。公式の仕様で必須の `has_issues`、`has_projects`、`has_wiki` の欠落は、取得契約に適合しない応答です。省略を許される `web_commit_signoff_required` の欠落は、Web コミットのサインオフ必須だけを、省略を許された値が返らなかったことを理由とする観測不能にします（[GitHub API 通信の設計](../github-access.md) 2.1節）。

マージメッセージの4項目は、GitHub が値を足し得る列挙として読みます（GitHub API 通信の設計 2.1節）。タイトルか本文のどちらかが reco の知らない文字列なら、その方式のマージメッセージだけを、値が知らない値であることを理由とする観測不能にします。もう一方の方式のマージメッセージと、ほかの General の設定は観測値を持ちます。General は設定の領域に分かれず、どの観測も、`PATCH` の応答の確かめも同じ応答を読むため、知らない値を観測操作の失敗にすると、その値と関係のない設定の `plan` と `apply` まで止まるためです。

マージメッセージは、対応するマージ方式が無効のあいだも観測値を持ちます。GitHub は、無効のあいだも値を保持して返すためです（同 4章）。GitHub が更新で受け付けるタイトルと本文の組み合わせは限られますが（同 4章）、タイトルと本文がどちらも reco の知る値なら、観測値は返った組のまま持ちます（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

マージ方式を無効にする変更予定は、更新後のマージ方式の値が3つとも `false` になるとき、適用不能です。無効にする変更予定が複数あれば、どれも適用不能です。GitHub は3つをすべて無効にする `PATCH` を `422` で拒否し、どのマージ方式の値も変えないためです（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）。これは、GitHub が値の組み合わせを受け付けない場合（[ドメインモデル](../domain.md) の照合結果と適用可能性）に当たり、適用不能の理由に無効のまま残るマージ方式を添えます。3つとも `false` の宣言は入力検証で拒否するため、この適用不能は無効のまま残るマージ方式を少なくとも一つ伴います。

マージメッセージの変更予定は、更新する時点で対応するマージ方式が有効と見込めるときに限り適用可能です。GitHub は、対応するマージ方式が無効のあいだもマージメッセージを値とともに返しますが、更新は `422` で拒否して値を変えないためです（GitHub リポジトリ API 仕様 4章）。これは、別の設定が有効なあいだに限って更新を受け付ける場合（[ドメインモデル](../domain.md) の照合結果と適用可能性）に当たり、対応するマージ方式に当てると次の表になります。

| 対応するマージ方式の計画時の観測 | 対応するマージ方式の宣言 | 不一致のマージメッセージの変更予定 |
|---|---|---|
| 有効 | なし、または `true` | 適用可能 |
| 有効 | `false` | 適用可能。マージ方式を無効にする前に更新する（論理的な設定変更と更新操作） |
| 無効 | `true` | 適用可能。マージ方式を有効にしたあとに更新する |
| 無効 | なし、または `false` | 適用不能。理由は対応するマージ方式が無効であること |

対応するマージ方式を無効にする宣言が、全無効のために適用不能になる場合も、マージメッセージの変更予定は上の表のとおり判断します。マージ方式の変更予定が適用不能なため、変更計画の全体は承認の対象になりません。rebase merge の状態は、マージメッセージの適用可能性に関わりません。

マージメッセージが、値が知らない値であることを理由に比較不能なら、その変更予定は適用不能です（ドメインモデルの照合結果と適用可能性）。Web コミットのサインオフ必須が、省略を許された値が返らなかったことを理由に比較不能なときも、その変更予定は適用不能です（同）。

`has_wiki` を `true` にする変更予定は適用可能です。GitHub Free のアカウントが所有する非公開リポジトリでは Wiki を利用できず、`has_wiki: true` を送る `PATCH` は `200` を返して値を変えませんが（GitHub リポジトリ API 仕様 4章）、取得の応答は料金プランを含まず、利用できないことを計画の時点で根拠づけられないためです。更新操作の成功のあとの不一致の扱いは、更新方式が定めます。

ほかの General の変更予定は、ほかの宣言と観測によらず適用可能です。

適用不能の理由は、宣言と計画時の観測値から組み立て、GitHub の応答からは組み立てません。`422` の応答本文の `errors[0].field` は、送ったフィールドにかかわらず `merge_commit_allowed` で、原因のマージ方式を示さないためです（GitHub リポジトリ API 仕様 4章）。対応するマージ方式が無効のときのマージメッセージの拒否でも同じです。

宣言していないマージ方式は、変更前提に含めません。承認のあとに外部主体が宣言していないマージ方式を無効にしていて全無効になる場合も、GitHub が `422` で拒否して値を変えないことを、残る1方式・2方式・3方式を無効にする要求で確かめているためです（GitHub リポジトリ API 仕様 4章）。変更結果は失敗になります。

General の変更予定に、変更に伴う副作用はありません。マージ方式の無効化は、マージメッセージの設定、`allow_auto_merge`、`delete_branch_on_merge` を変えませんでした。マージメッセージを送る要求も、もう一方の方式のマージメッセージ、マージ方式の3項目、ほかの General の設定を変えませんでした（GitHub リポジトリ API 仕様 4章）。リポジトリの機能と Web コミットのサインオフ必須のどれか一つだけを送る要求も、ほかの3つ、マージ関連の10フィールド、Secret scanning の2設定を変えませんでした（同 4章）。

Issues、Projects、Wiki を無効にすると、既存の Issue、Project、Wiki のページを GitHub の画面から利用できなくなります。これは宣言した設定の値が表す状態そのもので、宣言していない設定の値は変わらないため、変更に伴う副作用にも変更予定の注記にも当たらず、変更計画には何も添えません。無効にした Issues を有効に戻すと、無効にする前の Issue が残っていました（同 4章）。Wiki は、無効のあいだも内容が消えず、有効に戻すとページが戻ると公式資料が記載しています。Projects は、無効にしたときの扱いの記載がなく、確かめていません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 論理的な設定変更と更新操作

同じ対象リポジトリのマージ方式の変更予定（1つから3つ）は、一つの更新操作からなる一つの論理的な設定変更にまとめ、期待値を一つの `PATCH` で送ります。merge commit だけが有効なリポジトリで有効な方式を入れ替える宣言（`allow_merge_commit` を `false`、`allow_squash_merge` を `true`）を変更予定ごとに送ると、先に送る無効化が全無効の要求になって `422` で拒否されますが、一つの `PATCH` なら `200` で両方が反映されるためです（GitHub リポジトリ API 仕様 4章）。有効にする方式を先に送る順序で解く案は採りません。後の `PATCH` が完了しなかったとき、変更前にも宣言にもない組み合わせ（両方が有効）が残るためです。変更前提で比べる関係する現在値は、まとめた各マージ方式です。

`delete_branch_on_merge`、`allow_auto_merge`、`allow_update_branch`、リポジトリの機能の3設定、Web コミットのサインオフ必須の変更予定は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。マージ方式とも互いにも依存せず、まとめる理由がないためです。実行順は YAML 設定仕様の表の並びで、マージ方式をまとめた論理的な設定変更は、まとめたうちで最初のマージ方式の位置に並びます。

二つのマージメッセージの変更予定は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。実行順は原則として表の並び（マージ方式と `allow_update_branch` のあと）ですが、対応するマージ方式を無効にする変更予定があるときは、そのマージメッセージの変更を、マージ方式をまとめた論理的な設定変更の直前へ移します。マージメッセージの更新を、対応するマージ方式が有効な時点に置くためです。対応するマージ方式を有効にする変更予定があるときは、表の並びのままで、マージ方式の変更のあとに実行されます。

| 宣言の例 | 実行順 |
|---|---|
| `allow_squash_merge` を有効にし、`squash_merge_commit` を変える | マージ方式、`squash_merge_commit` |
| `allow_squash_merge` を無効にし、`squash_merge_commit` を変える | `squash_merge_commit`、マージ方式 |
| `allow_squash_merge` を有効、`allow_merge_commit` を無効にし、両方のマージメッセージを変える | `merge_commit`、マージ方式、`squash_merge_commit` |
| `allow_squash_merge` と `allow_merge_commit` を無効にし、両方のマージメッセージを変える | `squash_merge_commit`、`merge_commit`、マージ方式 |

移す先を、対応するマージ方式の位置ではなく、マージ方式をまとめた論理的な設定変更の位置にするのは、まとめた変更が最初のマージ方式の位置に並ぶためです。対応するマージ方式の直前に移すと、3つ目の例で `merge_commit` の変更が、`allow_merge_commit` を無効にしたあとに回ります。

マージメッセージをマージ方式と同じ `PATCH` にまとめる案は採りません。無効の方式を有効にするフィールドとその方式のマージメッセージは一つの要求で反映されますが、有効の方式を無効にするフィールドとその方式のマージメッセージを一つの要求で送ると `422` で何も変わりません（GitHub リポジトリ API 仕様 4章）。まとめると、方向によって要求を組み分けることになり、マージ方式の論理的な設定変更の更新方式と変更前提も変わります。順序で解けば、どちらも変えずに済みます。

変更前提で比べる関係する現在値は、そのマージメッセージだけで、対応するマージ方式は含めません。承認のあとに外部主体が対応するマージ方式を無効にしていても、GitHub が `422` で拒否して値を変えないためです（同 4章。[ドメインモデル](../domain.md) の承認と変更前提）。変更結果は失敗になります。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `alignment` | `INV-005` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 更新方式

General は、論理的な設定変更が変える設定だけを `PATCH /repos/{owner}/{repo}` に送り、省略したフィールドの保持に依拠して部分更新します。宣言して期待値と一致したマージ方式も送りません。マージ方式と `allow_update_branch` のどれか一つだけを送る要求と、3方式を一度に送る要求で、送らなかったマージ関連のフィールドが変わらないことを確かめており（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）、これを保全の根拠とします。リポジトリの機能と Web コミットのサインオフ必須のどれか一つだけを送る要求が、ほかの3つとマージ関連のフィールドを変えないことを公開と非公開のリポジトリで確かめており（同 4章）、これを保全の根拠とします。

マージメッセージの更新操作は、その方式のタイトルと本文の2フィールドを必ず両方送り、ほかのフィールドは送りません。GitHub は、片方だけを送る要求を、保存されている値と同じ値でも `422` で拒否することがあるためです。タイトルと本文を送る要求が、もう一方の方式のマージメッセージとほかの General の設定を変えないことを確かめており（同 4章）、これを保全の根拠とします。

この更新エンドポイントで reco が観測する設定のうち、更新の前後で比べるのは、General の単独の10設定と二つのマージメッセージと、Secret scanning の2設定（[Security の設計](security.md)）です。そのため、General を変える更新操作の更新直前と更新後の観測は Secret scanning の領域も読み、変えない General の設定と Secret scanning の2設定を更新直前の値と比べます。公開リポジトリで応答が `security_and_analysis` を含まなければ、Secret scanning の領域の取得契約に適合せず、更新要求を送る前に止まります。非公開リポジトリでは Secret scanning の2設定が観測不能で比べられませんが、トップレベルだけを送る `PATCH` が `security_and_analysis` の中の項目を変えなかったことを公開リポジトリで確かめており（GitHub リポジトリ API 仕様 5章）、これを保全の根拠とします。名前や説明など reco が読まないフィールドも、同じ確認結果を根拠に保全されるとみなします。マージメッセージが知らない値で観測不能なら、そのマージメッセージは比べられませんが、マージメッセージを送らない `PATCH` がマージメッセージを変えなかったこと（同 4章）を保全の根拠とします。Web コミットのサインオフ必須の値が更新直前か更新後に返らなければ、その設定は比べられませんが、ほかの General の設定と Secret scanning の項目だけを送る `PATCH` がその値を変えなかったこと（同 4章）を保全の根拠とします。

GitHub Free のアカウントが所有する非公開リポジトリでは、`allow_auto_merge` を送る `PATCH` が `200` を返しても値が変わりません（GitHub リポジトリ API 仕様 4章）。`allow_auto_merge` を変える論理的な設定変更が、更新操作の成功のあとに期待状態と一致しなかったときは、原因の候補として、GitHub Free の非公開リポジトリでは自動マージを利用できず、GitHub が更新を反映しないまま成功を応答する場合があることを添えます。公式資料が自動マージを利用できない条件として挙げるのはこの場合だけで、ほかの条件で同じ応答が起きないかは確かめていません。

GitHub Free のアカウントが所有する非公開リポジトリでは、`has_wiki: true` を送る `PATCH` も `200` を返して値を変えません（GitHub リポジトリ API 仕様 4章）。`has_wiki` を `true` にする論理的な設定変更が、更新操作の成功のあとに期待状態と一致しなかったときは、原因の候補として、GitHub Free の非公開リポジトリでは Wiki を利用できず、GitHub が更新を反映しないまま成功を応答する場合があることを添えます。`false` にする変更には添えません。`false` を送る要求は公開リポジトリで反映され、Wiki を利用できない非公開リポジトリでは値がもともと `false` のため、この原因では不一致を説明できないためです。GitHub Pro 以上のアカウントの非公開リポジトリでの応答は確かめていません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## シナリオと反例

以下は General の設定に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| `allow_merge_commit` を `false`、`allow_squash_merge` を `true` と宣言し、merge commit だけが有効だった | 2つの変更予定を一つの論理的な設定変更にまとめ、一つの `PATCH` で送る | 変更予定ごとに `PATCH` を送る。または、有効にする方式を先に送る |
| `allow_rebase_merge` を宣言して一致し、ほかの2方式の変更予定がある | `allow_rebase_merge` はまとめず、送らない。更新後には変更対象外の設定として比べる | 同じマージ方式なので、一致していても送る |
| 承認のあとに、宣言していないマージ方式を外部主体が無効にしていて、送ると全無効になる | 変更前提は成立し、`PATCH` を送る。GitHub が `422` で拒否して値を変えず、変更結果は失敗 | 更新直前に全無効になることを確かめ、未実行にする |
| マージ方式をまとめた `PATCH` が `200` を返し、一部の方式だけが期待値と違った | 期待状態は不一致で、変更結果は失敗。違ったマージ方式を示す | 一部は反映されたので完了。または、方式ごとに完了と失敗を分ける |
| マージ方式と `allow_update_branch` の変更予定がある | 2つの論理的な設定変更で、マージ方式をまとめた変更、`allow_update_branch` の変更の順に実行する | `allow_update_branch` もマージ方式とまとめる |
| `delete_branch_on_merge` だけを変える更新の前後で、マージメッセージが変わった | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 宣言していない設定なので比べない |
| マージメッセージを変える更新の前後で、マージ方式かもう一方のマージメッセージが変わった | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 送っていないので変わらない |
| `has_issues` だけを変える更新の前後で、`has_wiki` が変わった | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 宣言していない設定なので比べない |
| GitHub Free の非公開リポジトリで `has_wiki: true` を宣言した | 変更予定は適用可能。更新操作は成功し、期待状態は不一致で、変更結果は失敗。原因の候補を添える | 非公開リポジトリなので適用不能。または、`200` が返ったので完了 |
| `web_commit_signoff_required` を宣言せず、GitHub がその値を返さなかった | 変更計画は変わらない。General を変える更新の前後でも、この設定は比べない | 値を確かめられないので、General の変更をすべて適用不能にする |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [GitHub API 通信の設計](../github-access.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
