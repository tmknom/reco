# General の設計

設定宣言の `general`（[YAML 設定仕様](../../spec/configuration.md) 3章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| リポジトリ一般設定 | General | Pull Request の Auto-merge の許可やマージ後のブランチ自動削除など、リポジトリ全般の振る舞いに関する設定の領域 |
| マージ方式 | Merge Method | Pull Request をマージする方法の許可。squash merge（`allow_squash_merge`）、merge commit（`allow_merge_commit`）、rebase merge（`allow_rebase_merge`）の3つ |
| 更新後のマージ方式の値 | Merge Methods After Update | 変更計画を適用したあとの各マージ方式の値。宣言したマージ方式は期待値、宣言していないマージ方式は計画時の観測値 |
| 無効のまま残るマージ方式 | Merge Method Left Disabled | 宣言されておらず、計画時に無効だったマージ方式。更新後にすべてのマージ方式が無効になる変更予定の、適用不能の理由に添える |

`delete_branch_on_merge`、`allow_auto_merge`、`allow_squash_merge`、`allow_merge_commit`、`allow_rebase_merge`、`allow_update_branch` は、それぞれ単独の管理の単位です。3つのマージ方式は設定グループではなく、一部だけを宣言できます。`allow_update_branch` はマージ方式に数えません。General の設定は、Secret scanning の2設定と同じ更新エンドポイント（`PATCH /repos/{owner}/{repo}`）で更新します。この依存の扱いは [Security の設計](security.md) の Secret scanning が定めます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

リポジトリ ID を返す `GET /repos/{owner}/{repo}` は General の値も返すため、どの観測も General を含みます。

トークン所有者が Write 以上のロールを持たないリポジトリでは、`GET /repos/{owner}/{repo}` の応答から General の設定がそろって欠けます（[GitHub リポジトリ API 仕様](../../research/repositories.md) 1.1）。この欠落は取得条件による観測不能として扱わず、観測操作の失敗のままとします。原因は権限の不足であり、ドメインモデルは権限による失敗を観測結果に含めないためです。観測不能として `plan` を進めても、同じ権限では更新できず、利用者が対処すべき原因が後の失敗まで見えなくなります。

`GET /repos/{owner}/{repo}` の応答の `private` と `permissions` は、ほかの設定の領域の取得条件と応答の解釈に使います（[Actions の設計](actions.md)、[Security の設計](security.md)）。`private` は、どの設定の領域を読む観測でも取得契約に含め、欠けた応答は取得契約に適合しない応答とします。欠けた `private` を公開とみなすと、取得条件による観測不能を根拠なく作るためです。`permissions` は取得契約に含めず、Dependabot の領域を読まない観測は、欠けていても `null` でも失敗しません（読む観測での扱いは [Security の設計](security.md) の Dependabot の観測）。ただし、型の合わない `permissions`（オブジェクトでない、`admin` が boolean でない）は解釈できない本文であり、どの設定の領域を読む観測でも取得契約に適合しない応答とします。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

`delete_branch_on_merge`、`allow_auto_merge`、`allow_update_branch` の変更予定と、マージ方式を有効にする変更予定は、ほかの宣言と観測によらず適用可能です。General の設定は取得条件を持たないため、照合結果は一致か不一致で、比較不能は生じません。

マージ方式を無効にする変更予定（計画時の観測値が `true`、期待値が `false`）は、更新後のマージ方式の値が3つとも `false` になるとき、適用不能です。GitHub は、3つのマージ方式をすべて無効にする `PATCH` を `422` で拒否し、どのマージ方式の値も変えません（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）。これは GitHub が値の組み合わせを受け付けない場合の扱い（[ドメインモデル](../domain.md) の照合結果と適用可能性）に当たります。無効にする変更予定が複数あれば、どれも適用不能です。

この適用不能を解消できるのは、宣言していないマージ方式を有効にする変更だけで、[アーキテクチャ設計](../architecture.md) 5.4節の、利用者が宣言していない設定を reco が変えなければ実行できない場合に当たります。

| 宣言（期待値） | 宣言していないマージ方式の計画時の観測値 | 無効にする変更予定 |
|---|---|---|
| 1つか2つを宣言し、すべて `false` | どれかが `true` | 適用可能 |
| 1つか2つを宣言し、すべて `false` | すべて `false` | 適用不能。無効のまま残るマージ方式を理由に添える |
| 1つ以上を `true` | 問わない | 適用可能 |
| 3つとも `false` | なし | 入力検証で拒否（[YAML 設定仕様](../../spec/configuration.md) 3章） |

表の最後の行の宣言を入力検証で拒否するため、この適用不能は、無効のまま残るマージ方式を少なくとも一つ伴います。適用不能の理由は、宣言と計画時の観測値から組み立て、GitHub の応答からは組み立てません。`422` の応答本文の `errors[0].field` は、送ったフィールドにかかわらず `merge_commit_allowed` で、原因のマージ方式を示さないためです（GitHub リポジトリ API 仕様 4章）。

承認のあとに、宣言していないマージ方式を外部主体が無効にしていて、すべて無効になる場合は、GitHub が `422` で拒否し、どのマージ方式の値も変えないと見込みます。残る1方式だけを無効にする要求、残る2方式を一度に無効にする要求、3方式を一度に無効にする要求で、この応答を確かめています（GitHub リポジトリ API 仕様 4章）。そのため、宣言していないマージ方式の値は変更前提に含めず、適用条件にもしません（[ドメインモデル](../domain.md) の承認と変更前提）。

General の設定の変更予定に、変更に伴う副作用はありません。マージ方式の無効化は、マージメッセージの設定、`allow_auto_merge`、`delete_branch_on_merge` を変えませんでした（GitHub リポジトリ API 仕様 4章）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 論理的な設定変更と更新操作

同じ対象リポジトリのマージ方式の変更予定（1つから3つ）は、一つの更新操作からなる一つの論理的な設定変更にまとめます。更新操作は、まとめた変更予定の期待値を一つの `PATCH` で送ります。merge commit だけが有効なリポジトリで、有効な方式を入れ替える宣言（`allow_merge_commit` を `false`、`allow_squash_merge` を `true`）の変更予定ごとに別々の `PATCH` を送ると、先に無効にする要求が3方式をすべて無効にする要求になり、GitHub が `422` で拒否して値を変えないためです。一つの `PATCH` で送る入れ替えは `200` で両方が反映されます（GitHub リポジトリ API 仕様 4章）。

有効にする方式を先に送る順序で解く案は採りません。後の `PATCH` が完了しなかったとき、変更前にも宣言にもない組み合わせ（両方が有効）が GitHub に残るためです。宣言して期待値と一致したマージ方式と、宣言していないマージ方式は、`PATCH` に含めません。

`delete_branch_on_merge`、`allow_auto_merge`、`allow_update_branch` の変更予定は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。マージ方式とは互いに依存せず、まとめる理由がないためです。論理的な設定変更の実行順は YAML 設定仕様の表の並びで、マージ方式をまとめた論理的な設定変更は、まとめたうちで最初のマージ方式の位置に並びます。

マージ方式をまとめた更新操作では、まとめた各マージ方式を関係する現在値（[ドメインモデル](../domain.md) の承認と変更前提）とし、宣言していないマージ方式は含めません。その理由は、適用可能性と副作用の節が定めます。

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

General は、宣言された変更対象だけを `PATCH /repos/{owner}/{repo}` に送り、省略したフィールドの保持に依拠して部分更新します。省略したフィールドが保持されることを実際の動作で確認しています（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）。これが、宣言された変更対象だけを送る方式の根拠です。

更新後に再取得して更新直前の値と比べるのは、この更新エンドポイントで reco が観測する設定のうち、その論理的な設定変更が変えない設定です（`delete_branch_on_merge` を変える論理的な設定変更では、General のほかの5設定と Secret scanning の2設定。`allow_squash_merge` と `allow_merge_commit` をまとめた論理的な設定変更では、`delete_branch_on_merge`、`allow_auto_merge`、`allow_rebase_merge`、`allow_update_branch` と Secret scanning の2設定）。マージ方式と `allow_update_branch` のどれか一つだけを送る要求と、3方式を一度に送る要求で、送った項目のほかのマージ関連のフィールドが変わらないことを確かめています（GitHub リポジトリ API 仕様 4章）。Secret scanning の2設定は、非公開リポジトリでは観測不能のため比べません。トップレベルだけを送る `PATCH` が `security_and_analysis` の中の項目を変えなかったことを、公開リポジトリでの実際の動作で確認しており（GitHub リポジトリ API 仕様 5章）、これを非公開リポジトリでの保全の根拠とします。名前、説明など、reco が読まないフィールドと、取得条件の判定にだけ読む公開範囲は再確認せず、保持の確認結果を根拠に保全されるとみなします。

Secret scanning の2設定を比べるため、General を変える更新操作の更新直前と更新後の観測は Secret scanning の領域も読みます（[Security の設計](security.md) の Secret scanning の観測）。

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
| マージ方式を一つだけ宣言し、不一致だった | 変更予定が一つでも、マージ方式をまとめる論理的な設定変更として扱う。送る項目はその一つだけ | 一つだけのときは、まとめない別の更新操作にする |
| `allow_rebase_merge` を宣言して一致し、ほかの2方式の変更予定がある | `allow_rebase_merge` はまとめず、送らない。更新後には変更対象外の設定として比べる | 同じマージ方式なので、一致していても送る |
| 承認のあとに、宣言していないマージ方式を外部主体が無効にしていて、送ると全無効になる | 変更前提は成立し、`PATCH` を送る。GitHub が `422` で拒否して値を変えず、変更結果は失敗 | 更新直前に全無効になることを確かめ、未実行にする |
| マージ方式をまとめた `PATCH` が `200` を返し、一部の方式だけが期待値と違った | 期待状態は不一致で、変更結果は失敗。違ったマージ方式を示す | 一部は反映されたので完了。または、方式ごとに完了と失敗を分ける |
| マージ方式と `allow_update_branch` の変更予定がある | 2つの論理的な設定変更で、マージ方式をまとめた変更、`allow_update_branch` の変更の順に実行する | `allow_update_branch` もマージ方式とまとめる |

## References

- [設定ごとの設計](README.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
