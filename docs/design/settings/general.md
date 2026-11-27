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
| リポジトリの機能 | Repository Feature | リポジトリの Issues（`has_issues`）、Projects（`has_projects`）、Wiki（`has_wiki`）を有効にするかの3設定 |
| Web コミットのサインオフ必須 | Web Commit Signoff Requirement | GitHub の Web 画面からのコミットに、作成者のサインオフを求めるかの設定（`web_commit_signoff_required`）。コミットの署名を求める設定とは異なる |

General の6設定は、それぞれ単独の管理の単位です。3つのマージ方式は設定グループではなく、一部だけを宣言できます。`allow_update_branch` はマージ方式に数えません。

マージメッセージ、リポジトリの機能、Web コミットのサインオフ必須は、reco が観測して `show` が示すだけで、宣言はまだ受け付けません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

General は、どの観測も最初に送る `GET /repos/{owner}/{repo}`（[GitHub API 通信の設計](../github-access.md) 2.2節）の応答から読むため、どの観測も General を含みます。取得条件を持たず、General の6設定の照合結果に比較不能は生じません。

トークン所有者が Write 以上のロールを持たないリポジトリでは、応答から General の6設定とマージメッセージの4項目がそろって欠けます（[GitHub リポジトリ API 仕様](../../research/repositories.md) 1.1）。この欠落は取得条件による観測不能にせず、観測操作の失敗とします。原因は権限の不足であり、観測不能として `plan` を進めても、同じ権限では更新できないためです。

`has_issues`、`has_projects`、`has_wiki`、`web_commit_signoff_required` は、Write 以上のロールを持たないリポジトリでも値とともに返ります（同 1.1）。公式の仕様で必須の `has_issues`、`has_projects`、`has_wiki` の欠落は、取得契約に適合しない応答です。省略を許される `web_commit_signoff_required` の欠落は、Web コミットのサインオフ必須だけを、省略を許された値が返らなかったことを理由とする観測不能にします（[GitHub API 通信の設計](../github-access.md) 2.1節）。

マージメッセージの4項目は、GitHub が値を足し得る列挙として読みます（GitHub API 通信の設計 2.1節）。タイトルか本文のどちらかが reco の知らない文字列なら、その方式のマージメッセージだけを、値が知らない値であることを理由とする観測不能にします。もう一方の方式のマージメッセージと、ほかの General の設定は観測値を持ちます。General は設定の領域に分かれず、どの観測も、`PATCH` の応答の確かめも同じ応答を読むため、知らない値を観測操作の失敗にすると、その値と関係のない設定の `plan` と `apply` まで止まるためです。

マージメッセージは、対応するマージ方式が無効のあいだも観測値を持ちます。GitHub は、無効のあいだも値を保持して返すためです（同 4章）。GitHub が更新で受け付けるタイトルと本文の組み合わせは限られますが（同 4章）、タイトルと本文がどちらも reco の知る値なら、観測値は返った組のまま持ちます（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

マージ方式を無効にする変更予定は、更新後のマージ方式の値が3つとも `false` になるとき、適用不能です。無効にする変更予定が複数あれば、どれも適用不能です。GitHub は3つをすべて無効にする `PATCH` を `422` で拒否し、どのマージ方式の値も変えないためです（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）。これは、GitHub が値の組み合わせを受け付けない場合（[ドメインモデル](../domain.md) の照合結果と適用可能性）に当たり、適用不能の理由に無効のまま残るマージ方式を添えます。3つとも `false` の宣言は入力検証で拒否するため、この適用不能は無効のまま残るマージ方式を少なくとも一つ伴います。ほかの General の変更予定は、ほかの宣言と観測によらず適用可能です。

適用不能の理由は、宣言と計画時の観測値から組み立て、GitHub の応答からは組み立てません。`422` の応答本文の `errors[0].field` は、送ったフィールドにかかわらず `merge_commit_allowed` で、原因のマージ方式を示さないためです（GitHub リポジトリ API 仕様 4章）。

宣言していないマージ方式は、変更前提に含めません。承認のあとに外部主体が宣言していないマージ方式を無効にしていて全無効になる場合も、GitHub が `422` で拒否して値を変えないことを、残る1方式・2方式・3方式を無効にする要求で確かめているためです（GitHub リポジトリ API 仕様 4章）。変更結果は失敗になります。

General の変更予定に、変更に伴う副作用はありません。マージ方式の無効化は、マージメッセージの設定、`allow_auto_merge`、`delete_branch_on_merge` を変えませんでした（GitHub リポジトリ API 仕様 4章）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 論理的な設定変更と更新操作

同じ対象リポジトリのマージ方式の変更予定（1つから3つ）は、一つの更新操作からなる一つの論理的な設定変更にまとめ、期待値を一つの `PATCH` で送ります。merge commit だけが有効なリポジトリで有効な方式を入れ替える宣言（`allow_merge_commit` を `false`、`allow_squash_merge` を `true`）を変更予定ごとに送ると、先に送る無効化が全無効の要求になって `422` で拒否されますが、一つの `PATCH` なら `200` で両方が反映されるためです（GitHub リポジトリ API 仕様 4章）。有効にする方式を先に送る順序で解く案は採りません。後の `PATCH` が完了しなかったとき、変更前にも宣言にもない組み合わせ（両方が有効）が残るためです。変更前提で比べる関係する現在値は、まとめた各マージ方式です。

`delete_branch_on_merge`、`allow_auto_merge`、`allow_update_branch` の変更予定は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。マージ方式と互いに依存せず、まとめる理由がないためです。実行順は YAML 設定仕様の表の並びで、マージ方式をまとめた論理的な設定変更は、まとめたうちで最初のマージ方式の位置に並びます。

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

General は、論理的な設定変更が変える設定だけを `PATCH /repos/{owner}/{repo}` に送り、省略したフィールドの保持に依拠して部分更新します。宣言して期待値と一致したマージ方式も送りません。マージ方式と `allow_update_branch` のどれか一つだけを送る要求と、3方式を一度に送る要求で、送らなかったマージ関連のフィールドが変わらないことを確かめており（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）、これを保全の根拠とします。

この更新エンドポイントで reco が観測する設定のうち、更新の前後で比べるのは、General の6設定と、Secret scanning の2設定（[Security の設計](security.md)）です。マージメッセージ、リポジトリの機能、Web コミットのサインオフ必須は、宣言を受け付けないあいだは比べません。そのため、General を変える更新操作の更新直前と更新後の観測は Secret scanning の領域も読み、変えない General の設定と Secret scanning の2設定を更新直前の値と比べます。公開リポジトリで応答が `security_and_analysis` を含まなければ、Secret scanning の領域の取得契約に適合せず、更新要求を送る前に止まります。非公開リポジトリでは Secret scanning の2設定が観測不能で比べられませんが、トップレベルだけを送る `PATCH` が `security_and_analysis` の中の項目を変えなかったことを公開リポジトリで確かめており（GitHub リポジトリ API 仕様 5章）、これを保全の根拠とします。名前や説明など reco が読まないフィールドも、同じ確認結果を根拠に保全されるとみなします。

GitHub Free のアカウントが所有する非公開リポジトリでは、`allow_auto_merge` を送る `PATCH` が `200` を返しても値が変わりません（GitHub リポジトリ API 仕様 4章）。`allow_auto_merge` を変える論理的な設定変更が、更新操作の成功のあとに期待状態と一致しなかったときは、原因の候補として、GitHub Free の非公開リポジトリでは自動マージを利用できず、GitHub が更新を反映しないまま成功を応答する場合があることを添えます。公式資料が自動マージを利用できない条件として挙げるのはこの場合だけで、ほかの条件で同じ応答が起きないかは確かめていません。

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

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [GitHub API 通信の設計](../github-access.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
