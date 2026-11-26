# Rulesets の設計

設定宣言の `rulesets`（[YAML 設定仕様](../../spec/configuration.md) 13章）に属する、リポジトリの Ruleset の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。関連リソースに共通する規則は [ドメインモデル](../domain.md) の関連リソースと存在に従い、ここでは繰り返しません。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Rulesets | Rulesets | リポジトリに属する Ruleset からなる設定の領域 |
| Ruleset | Ruleset | ブランチへの操作を制限する、リポジトリに属する関連リソース。値を持つ種類で、GitHub が ID を付ける。名前はリポジトリの中で、`target` をまたいでも一意である |
| Ruleset の値 | Ruleset Value | `target`、`enforcement`、対象の条件、Bypass の集合、ルールの集合の組。名前は値に含めない |
| 対象の条件 | Conditions | Ruleset が効く ref を、含めるパターン（`include`）と除くパターン（`exclude`）で表す `ref_name` |
| Bypass | Bypass Actor | Ruleset のルールを迂回できる主体。主体の種類（`actor_type`）、主体の ID（`actor_id`）、迂回の方式（`bypass_mode`）の組 |
| ルール | Rule | Ruleset が課す制限の一つ。ルール型（`type`）と、型が持つならパラメータ（`parameters`）からなる |

Ruleset は、一覧と取得の応答が ID を含み、同じ名前で作り直すと ID が変わることを確かめた種類です（[GitHub Rules API 仕様](../../research/rules.md) 2章）。そのため、GitHub が ID を付ける種類として扱います（[ドメインモデル](../domain.md) の関連リソースの同一性）。

Ruleset の中身は項目ごとの管理の単位に分けず、値の全体を一つの値とします（ドメインモデルの管理の単位の形）。GitHub の更新は、送った `rules` で全体を置き換えるためです（GitHub Rules API 仕様 4.2）。期待値は「存在し、値が宣言と同じ」です。

二つの Ruleset の値は、次のすべての項目が同じときに限り同じ値です。

| 項目 | 比べ方 |
|---|---|
| `target`、`enforcement` | 文字列の一致 |
| 対象の条件 | `ref_name` があるかないか。あれば `include` と `exclude` を、それぞれ文字列の集合として比べる（順序と重複を問わず、大文字小文字を区別する）。`ref_name` のない値（`conditions: {}`）は、`include` と `exclude` がどちらも空の値とも異なる |
| Bypass | `actor_type`、`actor_id`、`bypass_mode` の組の集合 |
| ルール | ルール型とパラメータの値の組の集合。`pull_request` は5項目の一致、`deletion` は型の一致 |

- **集合として比べる理由**：GitHub は更新で `rules` の順を保たず、Bypass の順も保つとは限らない（GitHub Rules API 仕様 4.2、6章）。`include` と `exclude` は送った順で返ったが、Ruleset は `include` のどれかに一致し `exclude` のどれにも一致しない ref に効くため、順は意味を持たない（同 5章）
- **`include` の重複**：GitHub は `include` の重複を残して保存する（同 5章）。重複を問わないため、観測の `[a, a]` と宣言の `[a]` は同じ値で、更新しない
- **Bypass の同じ主体**：`actor_type` と `actor_id` が同じ要素を二つ送ると、GitHub は先の要素だけを残す（同 6章）。観測した値は受け取ったまま比べ、宣言では二つ書けない（YAML 設定仕様 13.4節）

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 観測

### 読み取り要求

Rulesets は、`GET /repos/{owner}/{repo}` の応答の `private` が偽のときだけ、Private vulnerability reporting のあとに読みます。真のときは Ruleset の要求を一つも送らず、Rulesets の一覧を、リポジトリが非公開であることを理由とする観測不能にします（下の取得条件による観測不能）。公開と観測したあとの一覧と取得の失敗は、`403` と `404` を含めて観測操作の失敗です。変更計画の確認と設定変更は、`rulesets` を宣言していないリポジトリでは一覧も送りません。

一覧は `GET /repos/{owner}/{repo}/rulesets?includes_parents=false&per_page=100` で読みます。`includes_parents=false` は、Organization 所有のリポジトリで、上位で構成された Ruleset が一覧に加わるのを避けるためです（GitHub Rules API 仕様 3.1）。一覧は `total_count` を返さないため、`total_count` のない一覧の読み方（[GitHub API 通信の設計](../github-access.md) 2.3節）で読み、ページ数の上限は1ページです。GitHub が一つのリポジトリに置ける Ruleset は75個で（GitHub Rules API 仕様 1章）、`per_page` の上限の100より少ないためです。1ページ目が100件を返したら、最後のページに達しないため観測操作の失敗とします。1ページの本文の上限は既定の値（同 2.1節）です。

一覧の要素は `id` と `name` しか使えません。`conditions`、`rules`、`bypass_actors` を含まないためです（GitHub Rules API 仕様 3.1）。値は、Ruleset ごとに `GET /repos/{owner}/{repo}/rulesets/{ruleset_id}` で読みます。この取得は関連リソースごとの読み取り要求で、範囲は GitHub API 通信の設計 2.4節に従います。状態確認は、一覧のすべての Ruleset に一覧の並びで送ります。変更計画の確認と設定変更は、宣言の名前と同じと判定した Ruleset にだけ送り、同じかどうかを判定できない名前の Ruleset には送りません。詳細の取得は、一覧のすべての要素を読み、名前の重複を確かめてから送ります。

一覧の名前の扱いは次のとおりです。

- **一覧の名前の重複**：Unicode の単純な大文字小文字の畳み込みで同じになる名前が二つ以上あれば、取得契約に適合しない応答とする。GitHub は大文字小文字だけが違う名前を同じとみなし、`Ä` と `ä` も同じとみなした（GitHub Rules API 仕様 2章）。二つを同じ Ruleset とみなすはずで、どちらを照合し更新するかが決まらないためである
- **一覧と詳細の食い違い**：ドメインモデルの存在の観測に従う。取得の `name` は、一覧の `name` と綴りの完全一致で比べる

### 取得契約

一覧の要素の取得契約は、`id`（整数）と、空でない `name`（文字列）がそろうことです。

詳細の取得契約は次のとおりです。どれかが成り立たない応答は、取得契約に適合しない応答です。

- `id`（整数）と `name`（文字列）
- `target`（文字列）
- `enforcement`（`disabled`、`active`、`evaluate` のどれか）
- `conditions`（オブジェクト）。`ref_name` があれば、オブジェクトで、`include` と `exclude` がどちらも文字列の配列
- `bypass_actors`（配列）。要素はオブジェクトで、`actor_type` が文字列。`actor_type` が `RepositoryRole`、`User`、`Integration`、`DeployKey` のどれかなら、`bypass_mode` が文字列で、`DeployKey` 以外は `actor_id` が整数、`DeployKey` は `actor_id` が欠けるか `null`。ほかの `actor_type` の要素は、ほかの項目の形が主体の種類ごとに違い得るため、契約に含めない（下の解釈しない部分）
- `rules`（配列）。要素はオブジェクトで、`type` が文字列。`type` が `pull_request` なら、`parameters` がオブジェクトで、`required_approving_review_count`（整数）、`dismiss_stale_reviews_on_push`、`require_code_owner_review`、`require_last_push_approval`、`required_review_thread_resolution`（boolean）がそろう

各項目の扱いの理由は次のとおりです。

- **`bypass_actors` の欠落と `null`**：GitHub は、Ruleset への書き込みのアクセスを持たない利用者には `bypass_actors` を返さない（GitHub Rules API 仕様 3.2）。権限の不足による欠落のため、空集合と読まず、観測操作の失敗とする（ドメインモデルの現在状態・観測結果・観測失敗）
- **`rules` の欠落と `null`**：作成で `rules` を省いても `[]` で返った（GitHub Rules API 仕様 4.1）。欠けた応答の意味を根拠づけられない
- **`conditions` の `null`**：公式のスキーマは `null` を取り得るとするが、`conditions` を送らずに作った Ruleset は `{}` を返した（GitHub Rules API 仕様 5章）。`null` を `{}` と同じと読む根拠がない
- **`enforcement`、`bypass_mode`、`DeployKey` の `actor_id`**：`enforcement` の値は公式資料が三つに限る。`bypass_mode` は、欠けたときに GitHub の既定値（`always`）で補うと、Bypass の方式を推測で示すことになる。`DeployKey` の `actor_id` は、公式資料が `null` とし、整数を送っても `null` で保存された（GitHub Rules API 仕様 6章）
- **`pull_request` の5項目**：値に含める項目で、欠けると値を作れない

`id`、`name`、`target`、`enforcement`、`conditions`、`bypass_actors`、`rules` のほかのトップレベルのフィールド（`source_type`、`source`、`current_user_can_bypass`、`node_id`、`_links`、`created_at`、`updated_at`、reco の知らないフィールド）は読まず、値にも、解釈しない部分にもしません。Ruleset の値を変える更新で送らないフィールドで、省いても既存の値が残るためです（ドメインモデルの現在状態・観測結果・観測失敗、GitHub Rules API 仕様 4.2）。

### 解釈しない部分

取得契約に適合する応答のうち、次のどれか一つでもあれば、その Ruleset の値は解釈しない部分を含みます（ドメインモデルの現在状態・観測結果・観測失敗）。値を作らず、解釈しなかったものを、種類と、GitHub が返した文字列（値かキーの名前）の組で持ちます。同じ応答が取得契約に適合しない部分も含むときは、取得契約に適合しない応答とします。

| 解釈しなかったもの | 当たる形 |
|---|---|
| `target` | `branch` 以外の値。Tag と Push の Ruleset は、対象の条件とルールの意味が違うため |
| 対象の条件のキー | `conditions` の `ref_name` 以外のキー、`ref_name` の `include`・`exclude` 以外のキー |
| Bypass の主体の種類 | `actor_type` が `RepositoryRole`、`User`、`Integration`、`DeployKey` 以外。この要素のほかの項目は確かめない |
| Bypass の方式 | `bypass_mode` が `always`、`pull_request`、`exempt` 以外 |
| Bypass のキー | 要素の `actor_type`・`actor_id`・`bypass_mode` 以外のキー |
| ルール型 | `type` が reco の解釈するルール型（`pull_request`、`deletion`）以外。この要素のほかの項目は確かめない |
| ルールのキー | 解釈するルール型の要素の、`type`・`parameters` 以外のキー。`deletion` の `parameters`（`{}` と `null` を含む） |
| ルールのパラメータ | 下の `pull_request` の表で、解釈しないとするもの |

解釈しなかったものは、`target`、`conditions`、`bypass_actors`、`rules` の順に、配列は応答の順に並べ、同じものは最初の一つだけを持ちます。一つの要素の中では、値を確かめる項目（`bypass_mode`、`pull_request` の任意の3項目）を先に、reco が読まないキーを名前の順にあとに並べ、ルールの要素のキーを `parameters` の中より先に並べます。JSON のオブジェクトのキーの並びは意味を持たず、応答の並びに合わせる根拠がないためです。`target` が `branch` 以外でも、ほかの部分も同じように確かめます。利用者が、その Ruleset のどこを reco が解釈しないかを一度に知るためです。

`enforcement` の `evaluate`、`conditions: {}`（`ref_name` がない）、`2`・`4`・`5` 以外の `RepositoryRole` の `actor_id`、`include` の重複、同じ主体の Bypass と同じ型のルールの重複は、解釈する値として受け取ったまま読みます。宣言では書けない値ですが、どれも Ruleset の値として意味が定まるためです（ドメインモデルの概念が属する文脈）。

`pull_request` のパラメータは次のとおり読みます。GitHub は取得で任意の3項目を既定値つきで返すため（GitHub Rules API 仕様 7.1）、既定値と同じときに限り、解釈できたものとします。既定値の3項目は、値に含めません。必須の5項目だけを送る更新が、この3項目を既定値にするためです（同 7.1）。

| パラメータ | 扱い |
|---|---|
| `required_approving_review_count`、`dismiss_stale_reviews_on_push`、`require_code_owner_review`、`require_last_push_approval`、`required_review_thread_resolution` | 値に含める。範囲（承認数の `0`〜`10` など）は確かめない |
| `allowed_merge_methods` | `merge`、`squash`、`rebase` の集合（順序と重複を問わない）なら解釈できた既定値。ほかの集合、欠落、`null`、文字列の配列でない値は解釈しない |
| `require_extra_approval_for_unattributed_changes` | `true` なら解釈できた既定値。ほかの値と欠落は解釈しない |
| `required_reviewers` | `[]` なら解釈できた既定値。空でない配列、欠落、配列でない値は解釈しない |
| `dismissal_restriction` とほかのキー | 解釈しない |

任意の3項目の欠落と型違いを取得契約に適合しない応答にせず、解釈しない部分とするのは、どちらでも Ruleset の値を作らず、そのほうが状態確認を止めないためです。

### 取得条件による観測不能

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| リポジトリが非公開（`private: true`） | Rulesets の一覧。どの名前の Ruleset も、同じ理由の観測不能になる（ドメインモデルの存在の観測） | GitHub Free の個人アカウントが所有する非公開リポジトリでは、一覧と取得が `403` を返した（GitHub Rules API 仕様 1章）。送れば観測操作の失敗になり、同じ観測のほかの設定も確認できなくなる |

GitHub Pro 以上のアカウントや Organization が所有する非公開リポジトリで Ruleset を利用できるかは確かめておらず、確かめるまでは非公開を一律に観測不能とします。

### 確かめていない条件

- GitHub が Ruleset の名前を同じとみなす範囲が、Unicode の単純な大文字小文字の畳み込みより広いか。広ければ、同じとみなされる名前が一覧に二つあることは起きない。狭ければ、別々の Ruleset の名前を重複と誤り、状態確認が観測操作の失敗で止まる
- `dismissal_restriction` を設定した Ruleset で、取得がそれを返すか
- Admin 以外の書き込みのアクセス（Write、Maintain）を持つ利用者のトークンで、取得が `bypass_actors` を返すか

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 適用可能性と副作用

Rulesets に固有の適用不能の条件は、リポジトリが非公開であることだけです。Rulesets が非公開を理由とする観測不能のため、宣言した Ruleset はどれも比較不能になります。取得条件による比較不能は一般には適用可能になり得ますが、GitHub Free の非公開リポジトリでは Ruleset の作成も `403` を返すため（GitHub Rules API 仕様 1章）、適用不能にします。公開範囲を変える宣言は reco にないため、ほかの宣言では解消しません。ほかの設定の照合は続けます。

値が解釈しない部分を含むことと、名前の同一性を判定できないことによる適用不能は、ドメインモデルの照合結果と適用可能性と、照合と変更予定に従います。適用不能の理由には、解釈しなかったものを、観測の解釈しない部分の並びで添えます。`enforcement: evaluate` や `conditions: {}` の Ruleset は、宣言が受け付けない値でも解釈する値のため（観測の解釈しない部分）、照合は Mismatch で、宣言した値へ置き換える変更は適用可能です。

宣言の内容から、Pull Request をマージできなくなる設定（承認者のいない個人アカウントのリポジトリで承認数を1以上にするなど）を判定しません。設定の望ましさの評価になるためです（[Constitution](../../CONSTITUTION.md) の Non-Goals）。

Ruleset の変更予定に、変更に伴う副作用と再開する設定はありません。`PUT` がリポジトリのマージ方式と Branch protection を変えないことを確かめています（GitHub Rules API 仕様 4.2）。宣言にないルールや Bypass がなくなることは、Ruleset という一つの管理設定の値の変更です（ドメインモデルの一覧の全体を宣言する設定との違い）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 論理的な設定変更と更新操作

Ruleset の変更予定は、Ruleset ごとに、一つの更新操作からなる一つの論理的な設定変更です。作成は値を一つの要求で設定するため、作成と値の設定を分けません。論理的な設定変更は、Security の論理的な設定変更のあとに、宣言の `rulesets` の並びで実行します。

| 変更予定 | 更新操作 | 関係する現在値と承認時に予定した値 | 期待状態 |
|---|---|---|---|
| 作成 | `POST /repos/{owner}/{repo}/rulesets` | 宣言の名前の Ruleset が存在しないこと | 存在し、値が宣言と同じであること |
| 値の変更 | `PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}` | 宣言の名前の Ruleset が存在し、`id` が計画時と同じで、値の全体が計画時の観測値と同じであること | 存在し、値が宣言と同じであること |

名前は、GitHub と同じく ASCII の大文字小文字を区別せずに比べます。作成の「存在しないこと」も、値の変更で引く Ruleset も、この比べ方で決まります。綴りだけが承認のあとに変わった Ruleset は、`id` と値の全体が同じなら同じ Ruleset として値を変えます。`PUT` は `name` を送らないため、綴りは GitHub 上のまま残ります。

値の変更の `PUT` の宛先は、承認時に予定した `id`（計画時の観測で、その名前の Ruleset が持っていた `id`）です。更新直前の観測では、値の全体を計画時の観測値と比べます。`PUT` は Ruleset の全体を置き換え、宣言が変えない部分を承認のあとに外部主体が足していても消すため、変える予定の部分だけを比べることはしません。宣言の名前の Ruleset が消されたか名前を変えられて存在しない場合と、`id` が計画時と違う（作り直された）場合は、関係する現在値の変化として `PUT` を送りません。

解釈しない部分を含む Ruleset と判定できない名前の扱いは、ドメインモデルの変更前提と期待状態の確認に従います。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-004` | Invariant |
| `alignment` | `INV-005` | Invariant |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 更新方式

作成の `POST` には、`name`（設定宣言の綴り）、`target`、`enforcement`、`conditions`（`{"ref_name": {"include": [...], "exclude": [...]}}`）、`bypass_actors`、`rules` をすべて送ります。トップレベルのフィールドを省略しないのは、省略したときの既定値を reco が値として持たないためです。本文の細則は次のとおりです。

- **空の集合**：`include`、`exclude`、`bypass_actors`、`rules` が空なら `[]` を送る。`[]` を明示した `POST` が空の配列で作ることを確かめている（GitHub Rules API 仕様 4.1）
- **並び**：`include`、`exclude`、`bypass_actors`、`rules` は宣言の並びで送る。比較は並びを問わないため、GitHub が並びを保つことには依拠しない
- **`DeployKey`**：`actor_id: null` を送る。公式資料が `DeployKey` では `null` にするとし、`null` の `POST` が受け付けられることを確かめている（同 6章）
- **`pull_request`**：`parameters` に宣言できる5項目だけを送る。任意の3項目は既定値で作られ、観測では既定値を解釈できる値として読むため、期待状態と一致する（同 7.1）
- **`deletion`**：`{"type": "deletion"}` だけを送り、`parameters` を持たせない（同 7.2）

正常応答は `201` で、本文は詳細の取得と同じ形です（同 4.1）。更新の契約は詳細の取得契約で、本文が適合しなければ成否不明とします。解釈しない部分を含む本文は契約に適合する応答として成功とし、作成した値は更新後の観測で確かめます。存在しない `RepositoryRole` の ID に GitHub は `500` を返して変更を反映しませんが（同 1章）、`5xx` の一般の扱いどおり成否不明とします。

値の変更の `PUT` には、作成の `POST` から `name` を除いた5つのフィールドを、同じ細則ですべて送ります。名前を送らないのは、reco が Ruleset の名前を変えないためです（ドメインモデルの関連リソースの同一性）。ほかのフィールドを省略しないのは、`PUT` が省略したフィールドの既存の値を保持するためです（GitHub Rules API 仕様 4.2）。省略すると、承認の前に示した値と違う値が残ります。`name` を除いた5つを送る `PUT` が `name` と `id` を保ったまま値を置き換えることを確かめています（同 4.2）。`pull_request` の任意の3項目は、必須の5項目だけを送る更新で既定値に戻るため（同 7.1）、期待状態と一致します。

`PUT` の正常応答は `200` で、本文は詳細の取得と同じ形です（同 4.2）。更新の契約と、解釈しない部分を含む本文、`5xx` の扱いは作成と同じです。

作成と値の変更のどちらの更新エンドポイントでも、reco が観測するのは宛先の Ruleset だけのため、更新後に比べる変更対象外の設定はありません。更新操作の成功後の不一致に添える `Possible cause:` の候補もありません。GitHub が `200` を返しながら送った値を変えて保存した事例（同じ種類のルールをまとめる、名前の前後の空白を除くなど）は、どれも宣言の検証で送らない形です。

### 防げない競合

更新直前の観測と `POST` のあいだに外部主体が同じ名前（大文字小文字だけが違う名前を含む）の Ruleset を作った場合、GitHub は `422` で作成を拒否し、Ruleset を作りません（GitHub Rules API 仕様 2章）。同じ名前の Ruleset が二つになることはなく、作成は更新操作の失敗になります。

更新直前の観測と `PUT` のあいだに外部主体が Ruleset を変えた場合、`PUT` はその変更も置き換えます。このあいだに Ruleset が消された場合、GitHub は `404` を返し（同 4.2）、値の変更は更新操作の失敗になります。このあいだに名前を変えられた場合は、`id` が変わらないため（同 2章）、`PUT` は宣言にない名前になった Ruleset の値を置き換えます。`PUT` の宛先は `id` でしか示せず、名前を条件にした更新要求がないためです。更新後の観測では宣言の名前の Ruleset が存在しないため、期待状態との不一致で失敗になり、利用者は `plan` でその Ruleset が作成として出ることから気づけます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## シナリオと反例

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| 非公開リポジトリの状態確認 | Ruleset の要求を送らず、Rulesets を非公開を理由とする観測不能として示す。ほかの設定を示して成り立つ | 一覧を送り、`403` で観測操作の失敗にする。または、Ruleset が一つもないと示す |
| 公開リポジトリで一覧が `403` を返した | 観測操作の失敗 | 非公開と同じ観測不能にする |
| `pull_request` の `allowed_merge_methods` が `["squash"]` | 解釈しない部分を含み、値を作らない | 必須の5項目が読めたので値を作る |
| `pull_request` の任意の3項目が既定値 | 解釈できた値。値は5項目だけを持つ | 宣言できない項目があるので解釈しない |
| 取得の応答に `bypass_actors` がない | 観測操作の失敗 | Bypass がないとして空集合で読む |
| `target` が `tag` の Ruleset | 解釈しない部分を含み、値を作らない。存在はするため、名前は示す | 一覧から省く |
| 一覧が100件を返した | 観測操作の失敗 | 100件をすべての Ruleset とする |
| 宣言と同じ名前の Tag Ruleset がある | 解釈しない部分を含む観測不能で、適用不能。作成も計画しない（名前は `target` をまたいで一意） | 別の `target` なので Branch Ruleset を作成する。または、`tag` から `branch` へ変える |
| GitHub 上に `non_fast_forward` を持つ Ruleset を、`pull_request` と `deletion` だけで宣言した | 解釈しない部分を含む観測不能で、適用不能 | 宣言にないルールなので外す |
| 非公開リポジトリで `rulesets` を宣言した | 比較不能で適用不能。Ruleset の要求を送らない。ほかの設定の照合は続ける | 一覧を送り、`403` で観測操作の失敗にする |
| 作成を承認したあと、外部主体が大文字小文字だけが違う名前の Ruleset を作った | 関係する現在値の変化。`POST` を送らず未実行 | 綴りが違うので作成する。または、同じ値なので完了 |
| 値の変更を承認したあと、外部主体がその Ruleset の名前を変えた | 宣言の名前の Ruleset が存在しなくなり、関係する現在値の変化。未実行 | `id` が同じなので、新しい名前の Ruleset へ `PUT` を送る |

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |
