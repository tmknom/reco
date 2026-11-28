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

Ruleset の中身は項目ごとの管理の単位に分けず、値の全体を一つの値とします（ドメインモデルの管理の単位の形）。GitHub の更新は、送った `rules` で全体を置き換えるためです（GitHub Rules API 仕様 4.2）。期待値は「存在し、値が宣言と同じ」か、「存在しない」です。

二つの Ruleset の値は、次のすべての項目が同じときに限り同じ値です。

| 項目 | 比べ方 |
|---|---|
| `target`、`enforcement` | 文字列の一致 |
| 対象の条件 | `ref_name` があるかないか。あれば `include` と `exclude` を、それぞれ文字列の集合として比べる（順序と重複を問わず、大文字小文字を区別する）。`ref_name` のない値（`conditions: {}`）は、`include` と `exclude` がどちらも空の値とも異なる |
| Bypass | `actor_type`、`actor_id`、`bypass_mode` の組の集合 |
| ルール | ルール型とパラメータの値の組の集合。`deletion` と `non_fast_forward` は型の一致、`pull_request` と `required_status_checks` は下のとおり |

`pull_request` の二つのルールは、次のすべてが同じときに限り同じです。

| パラメータ | 比べ方 |
|---|---|
| `required_approving_review_count`、`dismiss_stale_reviews_on_push`、`require_code_owner_review`、`require_last_push_approval`、`required_review_thread_resolution` | 値の一致 |
| `allowed_merge_methods` | 二つの値の両方が持つときだけ、方式の集合の一致（順序と重複を問わない）。片方が持たなければ比べない |
| `require_extra_approval_for_unattributed_changes` | 二つの値の両方が持つときだけ、真偽値の一致。片方が持たなければ比べない |

`required_status_checks` の二つのルールは、次のすべてが同じときに限り同じです。

| パラメータ | 比べ方 |
|---|---|
| `required_status_checks` | `context` と `integration_id` の組の集合（順序と重複を問わず、`context` は大文字小文字を区別した文字列の一致）。`integration_id` のないチェックは、発行元を限らないという一つの値で、どの ID を持つチェックとも異なる |
| `strict_required_status_checks_policy` | 真偽値の一致 |
| `do_not_enforce_on_create` | 二つの値の両方が持つときだけ、真偽値の一致。片方が持たなければ比べない |

`pull_request` の `allowed_merge_methods` と `require_extra_approval_for_unattributed_changes`、`required_status_checks` の `do_not_enforce_on_create` は、ルールの省略できるパラメータです（[ドメインモデル](../domain.md) の省略できる項目のうち、送らなかったときに GitHub が決まった値にするもの）。宣言で省略した値はそのパラメータを持たず、観測した値は必ず持ちます。

- **集合として比べる理由**：GitHub は更新で `rules` の順を保たず、Bypass の順も保つとは限らない（GitHub Rules API 仕様 4.2、6章）。`include` と `exclude` は送った順で返ったが、Ruleset は `include` のどれかに一致し `exclude` のどれにも一致しない ref に効くため、順は意味を持たない（同 5章）
- **`include`、ステータスチェック、許すマージ方式の重複**：GitHub は `include`、ステータスチェック、`allowed_merge_methods` の重複を残して保存する（同 5章、7.1、7.3）。重複を問わないため、観測の `[a, a]` と宣言の `[a]` は同じ値で、更新しない。どのチェックも必須という意味も、どの方式を許すかという意味も、順序と重複で変わらない
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
- `enforcement`（文字列）
- `conditions`（オブジェクト）。`ref_name` があれば、オブジェクトで、`include` と `exclude` がどちらも文字列の配列
- `bypass_actors`（配列）。要素はオブジェクトで、`actor_type` が文字列。`actor_type` が `RepositoryRole`、`User`、`Integration`、`DeployKey` のどれかなら、`bypass_mode` が文字列で、`DeployKey` 以外は `actor_id` が整数、`DeployKey` は `actor_id` が欠けるか `null`。ほかの `actor_type` の要素は、ほかの項目の形が主体の種類ごとに違い得るため、契約に含めない（下の解釈しない部分）
- `rules`（配列）。要素はオブジェクトで、`type` が文字列。`type` が `pull_request` なら、`parameters` がオブジェクトで、`required_approving_review_count`（整数）、`dismiss_stale_reviews_on_push`、`require_code_owner_review`、`require_last_push_approval`、`required_review_thread_resolution`（boolean）がそろう。`type` が `required_status_checks` なら、`parameters` がオブジェクトで、`required_status_checks` が配列、その要素がオブジェクトで `context` が文字列、`strict_required_status_checks_policy` が boolean

各項目の扱いの理由は次のとおりです。

- **`bypass_actors` の欠落と `null`**：GitHub は、Ruleset への書き込みのアクセスを持たない利用者には `bypass_actors` を返さない（GitHub Rules API 仕様 3.2）。権限の不足による欠落のため、空集合と読まず、観測操作の失敗とする（ドメインモデルの現在状態・観測結果・観測失敗）
- **`rules` の欠落と `null`**：作成で `rules` を省いても `[]` で返った（GitHub Rules API 仕様 4.1）。欠けた応答の意味を根拠づけられない
- **`conditions` の `null`**：公式のスキーマは `null` を取り得るとするが、`conditions` を送らずに作った Ruleset は `{}` を返した（GitHub Rules API 仕様 5章）。`null` を `{}` と同じと読む根拠がない
- **`bypass_mode`、`DeployKey` の `actor_id`**：`bypass_mode` は、欠けたときに GitHub の既定値（`always`）で補うと、Bypass の方式を推測で示すことになる。`DeployKey` の `actor_id` は、公式資料が `null` とし、整数を送っても `null` で保存された（GitHub Rules API 仕様 6章）
- **`pull_request` の必須の5項目と、`required_status_checks` の必須の項目**：値に含める項目で、欠けると値を作れない。省略できるパラメータの欠落は、下の解釈しない部分とする

`id`、`name`、`target`、`enforcement`、`conditions`、`bypass_actors`、`rules` のほかのトップレベルのフィールド（`source_type`、`source`、`current_user_can_bypass`、`node_id`、`_links`、`created_at`、`updated_at`、reco の知らないフィールド）は読まず、値にも、解釈しない部分にもしません。Ruleset の値を変える更新で送らないフィールドで、省いても既存の値が残るためです（ドメインモデルの現在状態・観測結果・観測失敗、GitHub Rules API 仕様 4.2）。

### 解釈しない部分

取得契約に適合する応答のうち、次のどれか一つでもあれば、その Ruleset の値は解釈しない部分を含みます（ドメインモデルの現在状態・観測結果・観測失敗）。値を作らず、解釈しなかったものを、種類と、GitHub が返した文字列（値かキーの名前）の組で持ちます。同じ応答が取得契約に適合しない部分も含むときは、取得契約に適合しない応答とします。

| 解釈しなかったもの | 当たる形 |
|---|---|
| `target` | `branch` 以外の値。Tag と Push の Ruleset は、対象の条件とルールの意味が違うため |
| `enforcement` | `disabled`、`active`、`evaluate` 以外の値。GitHub が値を足し得る列挙で（公式の OpenAPI 記述は3値）、足された値の意味を reco は知らないため |
| 対象の条件のキー | `conditions` の `ref_name` 以外のキー、`ref_name` の `include`・`exclude` 以外のキー |
| Bypass の主体の種類 | `actor_type` が `RepositoryRole`、`User`、`Integration`、`DeployKey` 以外。この要素のほかの項目は確かめない |
| Bypass の方式 | `bypass_mode` が `always`、`pull_request`、`exempt` 以外 |
| Bypass のキー | 要素の `actor_type`・`actor_id`・`bypass_mode` 以外のキー |
| ルール型 | `type` が reco の解釈するルール型（`pull_request`、`deletion`、`required_status_checks`、`non_fast_forward`）以外。この要素のほかの項目は確かめない |
| ルールのキー | 解釈するルール型の要素の、`type`・`parameters` 以外のキー。`deletion` と `non_fast_forward` の `parameters`（`{}` と `null` を含む） |
| ルールのパラメータ | 下の `pull_request` と `required_status_checks` の表で、解釈しないとするもの |

解釈しなかったものは、`target`、`enforcement`、`conditions`、`bypass_actors`、`rules` の順に、配列は応答の順に並べ、同じものは最初の一つだけを持ちます。一つの要素の中では、値を確かめる項目（`bypass_mode`、`pull_request` の `allowed_merge_methods`・`require_extra_approval_for_unattributed_changes`・`required_reviewers`、`required_status_checks` の `do_not_enforce_on_create` と `integration_id`）を先に、reco が読まないキーを名前の順にあとに並べ、ルールの要素のキーを `parameters` の中より先に並べます。JSON のオブジェクトのキーの並びは意味を持たず、応答の並びに合わせる根拠がないためです。`target` が `branch` 以外でも、ほかの部分も同じように確かめます。利用者が、その Ruleset のどこを reco が解釈しないかを一度に知るためです。

`enforcement` の `evaluate`、`conditions: {}`（`ref_name` がない）、`2`・`4`・`5` 以外の `RepositoryRole` の `actor_id`、`include` の重複、同じ主体の Bypass と同じ型のルールの重複は、解釈する値として受け取ったまま読みます。宣言では書けない値ですが、どれも Ruleset の値として意味が定まるためです（ドメインモデルの概念が属する文脈）。

`pull_request` のパラメータは次のとおり読みます。`allowed_merge_methods` と `require_extra_approval_for_unattributed_changes` は宣言できるため、定義された値なら値に含めます（解釈する範囲は宣言で受け付ける範囲をすべて含む）。宣言できない `required_reviewers` は、GitHub が取得で既定値つきで返すため（GitHub Rules API 仕様 7.1）、既定値と同じときに限り解釈できたものとし、値に含めません。必須の5項目と宣言したパラメータだけを送る更新が、`required_reviewers` を既定値にするためです（同 7.1）。

| パラメータ | 扱い |
|---|---|
| `required_approving_review_count`、`dismiss_stale_reviews_on_push`、`require_code_owner_review`、`require_last_push_approval`、`required_review_thread_resolution` | 値に含める。範囲（承認数の `0`〜`10` など）は確かめない |
| `allowed_merge_methods` | `merge`、`squash`、`rebase` だけからなる空でない配列なら値に含める。応答の順のまま、重複も受け取ったまま読む。`[]`、定義にない値や文字列でない要素を含む配列、欠落、`null`、配列でない値は解釈しない。GitHub は `[]` と定義にない値を拒否したため、返ったときの意味を定める根拠がない |
| `require_extra_approval_for_unattributed_changes` | `true` と `false` を値に含める。欠落、`null`、真偽値でない値は解釈しない |
| `required_reviewers` | `[]` なら解釈できた既定値。空でない配列、欠落、配列でない値は解釈しない |
| `dismissal_restriction` とほかのキー | 解釈しない |

`pull_request` の任意の3項目の欠落と型違いを取得契約に適合しない応答にせず、解釈しない部分とするのは、どちらでも Ruleset の値を作らず、そのほうが状態確認を止めないためです。

`required_status_checks` のパラメータは次のとおり読みます。`do_not_enforce_on_create` は宣言できるため、どちらの値も解釈できた値として値に含めます。

| パラメータ | 扱い |
|---|---|
| `required_status_checks` | 値に含める。チェックを応答の順のまま、重複も、空文字列の `context` も受け取ったまま読む |
| チェックの `integration_id` | キーがなければ、発行元を限らないチェック。正の整数なら値に含める。`null`、`0` 以下、整数でない値は解釈しない（`required_status_checks.integration_id`）。GitHub は送らなかった `integration_id` を返さず、`null` と `0` を拒否したため、意味を定める根拠がない（GitHub Rules API 仕様 7.3） |
| チェックのほかのキー | 解釈しない（`required_status_checks.<キー>`） |
| `strict_required_status_checks_policy` | 値に含める |
| `do_not_enforce_on_create` | `true` と `false` を値に含める。欠落、`null`、真偽値でない値は解釈しない。取得は常に返したが、`pull_request` の任意の3項目と同じく、欠落を取得契約に適合しない応答にしない |
| ほかのキー | 解釈しない |

### 取得条件による観測不能

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| リポジトリが非公開（`private: true`） | Rulesets の一覧。どの名前の Ruleset も、同じ理由の観測不能になる（ドメインモデルの存在の観測） | GitHub Free の個人アカウントが所有する非公開リポジトリでは、一覧と取得が `403` を返した（GitHub Rules API 仕様 1章）。送れば観測操作の失敗になり、同じ観測のほかの設定も確認できなくなる |

GitHub Pro 以上のアカウントや Organization が所有する非公開リポジトリで Ruleset を利用できるかは確かめておらず、確かめるまでは非公開を一律に観測不能とします。

### 確かめていない条件

- GitHub が Ruleset の名前を同じとみなす範囲が、Unicode の単純な大文字小文字の畳み込みより広いか。広ければ、同じとみなされる名前が一覧に二つあることは起きない。狭ければ、別々の Ruleset の名前を重複と誤り、状態確認が観測操作の失敗で止まる
- Organization が所有するリポジトリで、`dismissal_restriction` を設定した Ruleset の取得がそれを返すか。返さないまま保存されるなら、reco はそれを観測できず、`pull_request` の必須の5項目と宣言したパラメータだけを送る更新が外し得る。個人アカウントが所有するリポジトリでは、GitHub は送った値を `200` のまま保存しないと見られ、外す値はない（GitHub Rules API 仕様 7.1）
- Admin 以外の書き込みのアクセス（Write、Maintain）を持つ利用者のトークンで、取得が `bypass_actors` を返すか

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 適用可能性と副作用

Rulesets に固有の適用不能の条件は、リポジトリが非公開であることと、省略したパラメータを GitHub が変えることです。非公開のリポジトリでは、Rulesets が非公開を理由とする観測不能のため、宣言した Ruleset はどれも比較不能になります。取得条件による比較不能は一般には適用可能になり得ますが、GitHub Free の非公開リポジトリでは Ruleset の作成も `403` を返すため（GitHub Rules API 仕様 1章）、適用不能にします。公開範囲を変える宣言は reco にないため、ほかの宣言では解消しません。ほかの設定の照合は続けます。

省略したパラメータに当たるのは、次の3つです。宣言がどれかを省略し、計画時に観測した値が、GitHub が送らなかったときに決める値と違う値の変更は、適用不能です。GitHub は、`rules` を置き換える更新で送らなかったパラメータを、次の値にするためです。

| ルール型 | パラメータ | 送らなかったときに GitHub が決める値 | 根拠 |
|---|---|---|---|
| `pull_request` | `allowed_merge_methods` | `merge`、`squash`、`rebase` の集合 | GitHub Rules API 仕様 7.1 |
| `pull_request` | `require_extra_approval_for_unattributed_changes` | `true` | 同 7.1 |
| `required_status_checks` | `do_not_enforce_on_create` | `false` | 同 7.3 |

`allowed_merge_methods` の観測値は集合として比べ、`["rebase", "merge", "squash", "squash"]` は当たりません。扱いの一般の規則は、ドメインモデルの照合結果と適用可能性に従います。

値が解釈しない部分を含むことと、名前の同一性を判定できないことによる適用不能は、ドメインモデルの照合結果と適用可能性と、照合と変更予定に従います。適用不能の理由には、解釈しなかったものを、観測の解釈しない部分の並びで添えます。`enforcement: evaluate` や `conditions: {}` の Ruleset は、宣言が受け付けない値でも解釈する値のため（観測の解釈しない部分）、照合は Mismatch で、宣言した値へ置き換える変更は適用可能です。

存在しないことを宣言した Ruleset にも、非公開、解釈しない部分、判定できない名前の3つの適用不能が同じく当たります。省略したパラメータを GitHub が変えることによる適用不能は当たりません。削除は値を送らないためです。`enforcement: evaluate` や `conditions: {}` の Ruleset の削除は、消える値の全体を示せるため適用可能です。

宣言の内容から、Pull Request をマージできなくなる設定（承認者のいない個人アカウントのリポジトリで承認数を1以上にするなど）を判定しません。設定の望ましさの評価になるためです（[Constitution](../../CONSTITUTION.md) の Non-Goals）。リポジトリの General で無効にしたマージ方式だけを `allowed_merge_methods` に宣言した場合も同じで、適用可能のまま、注記も添えません。GitHub はこの食い違いをどちら向きにも拒否せず、Ruleset の作成と更新も、General のマージ方式の更新も反映します（GitHub Rules API 仕様 7.1）。そのため、Ruleset の変更計画の確認と設定変更は General を読みません。

Ruleset の変更予定に、変更に伴う副作用と再開する設定はありません。`PUT` と削除がリポジトリのマージ方式と Branch protection を変えないことを確かめています（GitHub Rules API 仕様 1章、4.2）。宣言にないルールや Bypass がなくなることは、Ruleset という一つの管理設定の値の変更です（ドメインモデルの一覧の全体を宣言する設定との違い）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 論理的な設定変更と更新操作

Ruleset の変更予定は、Ruleset ごとに、一つの更新操作からなる一つの論理的な設定変更です。作成は値を一つの要求で設定するため、作成と値の設定を分けません。削除も一つの更新操作です。論理的な設定変更は、Security の論理的な設定変更のあとに、宣言の `rulesets` の並びで実行します。

| 変更予定 | 更新操作 | 関係する現在値と承認時に予定した値 | 期待状態 |
|---|---|---|---|
| 作成 | `POST /repos/{owner}/{repo}/rulesets` | 宣言の名前の Ruleset が存在しないこと | 存在し、値が宣言と同じであること |
| 値の変更 | `PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}` | 宣言の名前の Ruleset が存在し、`id` が計画時と同じで、値の全体が計画時の観測値と同じであること | 存在し、値が宣言と同じであること |
| 削除 | `DELETE /repos/{owner}/{repo}/rulesets/{ruleset_id}` | 値の変更と同じ | 存在しないと確かめられること |

名前は、GitHub と同じく ASCII の大文字小文字を区別せずに比べます。作成の「存在しないこと」も、値の変更で引く Ruleset も、この比べ方で決まります。綴りだけが承認のあとに変わった Ruleset は、`id` と値の全体が同じなら同じ Ruleset として値を変えます。`PUT` は `name` を送らないため、綴りは GitHub 上のまま残ります。

値の変更の `PUT` の宛先は、承認時に予定した `id`（計画時の観測で、その名前の Ruleset が持っていた `id`）です。更新直前の観測では、値の全体を計画時の観測値と比べます。`PUT` は Ruleset の全体を置き換え、宣言が変えない部分を承認のあとに外部主体が足していても消すため、変える予定の部分だけを比べることはしません。宣言で省略したパラメータも比べます（ドメインモデルの承認と変更前提）。宣言の名前の Ruleset が消されたか名前を変えられて存在しない場合と、`id` が計画時と違う（作り直された）場合は、関係する現在値の変化として `PUT` を送りません。

削除の `DELETE` の宛先も、承認時に予定した `id` です。変更前提は値の変更と同じで、更新直前の観測の値の全体を計画時の観測値と比べます。計画時の観測値は、承認の前に消える値として示した全体で、省略できるパラメータも含みます。

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
- **`pull_request`**：`parameters` に必須の5項目を必ず送り、`allowed_merge_methods`（宣言の並び）と `require_extra_approval_for_unattributed_changes` は宣言したときだけ送る。省略したときは GitHub が適用可能性と副作用の表の値で保存し（同 7.1）、適用可能な変更予定では観測値もその値のため、値は変わらない。`required_reviewers` は送らず、既定値の `[]` で保存される
- **`deletion` と `non_fast_forward`**：`{"type": "deletion"}` と `{"type": "non_fast_forward"}` だけを送り、`parameters` を持たせない（同 7.2、7.4）
- **`required_status_checks`**：`parameters` に、`required_status_checks`（空なら `[]`）と `strict_required_status_checks_policy` を必ず送る。チェックは `context` と、宣言したときだけ `integration_id` を送る。`do_not_enforce_on_create` は宣言したときだけ送る。省略したときの扱いは `pull_request` の省略できるパラメータと同じである（同 7.3）。インストールされていない App の `integration_id` は、GitHub が `422` で拒否し（同 7.3）、更新操作の失敗になる

正常応答は `201` で、本文は詳細の取得と同じ形です（同 4.1）。更新の契約は詳細の取得契約で、本文が適合しなければ成否不明とします。解釈しない部分を含む本文は契約に適合する応答として成功とし、作成した値は更新後の観測で確かめます。存在しない `RepositoryRole` の ID に GitHub は `500` を返して変更を反映しませんが（同 1章）、`5xx` の一般の扱いどおり成否不明とします。

値の変更の `PUT` には、作成の `POST` から `name` を除いた5つのフィールドを、同じ細則ですべて送ります。名前を送らないのは、reco が Ruleset の名前を変えないためです（ドメインモデルの関連リソースの同一性）。ほかのフィールドを省略しないのは、`PUT` が省略したフィールドの既存の値を保持するためです（GitHub Rules API 仕様 4.2）。省略すると、承認の前に示した値と違う値が残ります。`name` を除いた5つを送る `PUT` が `name` と `id` を保ったまま値を置き換えることを確かめています（同 4.2）。`pull_request` の `required_reviewers` は、送らない更新で既定値に戻るため（同 7.1）、期待状態と一致します。

`PUT` の正常応答は `200` で、本文は詳細の取得と同じ形です（同 4.2）。更新の契約と、解釈しない部分を含む本文、`5xx` の扱いは作成と同じです。

削除の `DELETE` は本文を送りません。正常応答は本文のない `204` です（同 1章）。存在しない `id` への `DELETE` に GitHub が返す `404`（同 1章）は、`4xx` の一般の扱いどおり更新操作の失敗で、存在しないことの確認に読み替えません。

作成、値の変更、削除のどの更新エンドポイントでも、reco が観測するのは宛先の Ruleset だけのため、ほかの設定は変更対象外の設定として比べません。削除が同じリポジトリのほかの Ruleset を変えないことを確かめています（同 1章）。値の変更では、宣言で省略したパラメータごとに、更新直前と更新後の観測値を、変更対象外の設定と同じく比べます（ドメインモデルの管理の単位と管理設定）。`allowed_merge_methods` は集合として比べます。適用可能な変更予定では、更新の前後のどちらも、GitHub が送らなかったときに決める値のはずです。作成は、更新直前に比べる値がありません。更新操作の成功後の不一致に添える `Possible cause:` の候補もありません。GitHub が `200` を返しながら送った値を変えて保存した事例（同じ種類のルールをまとめる、名前の前後の空白を除くなど）は、どれも宣言の検証で送らない形です。

### 防げない競合

更新直前の観測と `POST` のあいだに外部主体が同じ名前（大文字小文字だけが違う名前を含む）の Ruleset を作った場合、GitHub は `422` で作成を拒否し、Ruleset を作りません（GitHub Rules API 仕様 2章）。同じ名前の Ruleset が二つになることはなく、作成は更新操作の失敗になります。

更新直前の観測と `PUT` のあいだに外部主体が Ruleset を変えた場合、`PUT` はその変更も置き換えます。このあいだに Ruleset が消された場合、GitHub は `404` を返し（同 4.2）、値の変更は更新操作の失敗になります。このあいだに名前を変えられた場合は、`id` が変わらないため（同 2章）、`PUT` は宣言にない名前になった Ruleset の値を置き換えます。`PUT` の宛先は `id` でしか示せず、名前を条件にした更新要求がないためです。更新後の観測では宣言の名前の Ruleset が存在しないため、期待状態との不一致で失敗になり、利用者は `plan` でその Ruleset が作成として出ることから気づけます。

更新直前の観測と `DELETE` のあいだに外部主体が Ruleset を変えた場合、`DELETE` はその変更ごと Ruleset を消します。このあいだに Ruleset が消された場合、GitHub は `404` を返し（同 1章）、削除は更新操作の失敗になります。このあいだに名前を変えられた場合は、`id` が変わらないため（同 2章）、`DELETE` は宣言にない名前になった Ruleset を消します。`DELETE` の宛先も `id` でしか示せず、名前を条件にした削除の要求がないためです。消えるのは、承認の前に値の全体を示し、更新直前の観測で `id` と値を確かめた Ruleset そのものです。

この場合、更新後の観測では宣言の名前の Ruleset が存在しないため、期待状態と一致し、変更結果は完了になります。値の変更と違い、名前の変更は reco の表示に現れません。reco はこの競合を防がず、検出もしません。reco が変更対象に選んだのは、更新直前の観測で宣言の名前を持つと確かめた Ruleset で、そのあとの名前の変更は外部主体による同時変更に当たり、INV-001 が防ぐ対象に含みません。名前を変えられた Ruleset を reco は同じものとして追わないため、更新後の観測でも区別できません（ドメインモデルの関連リソースの同一性）。

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
| `pull_request` の `allowed_merge_methods` が `["squash"]` | 解釈できた値。`allowed_merge_methods` を値に含める | 既定値でないので解釈しない部分とする |
| `pull_request` の `required_reviewers` が空でない | 解釈しない部分を含み、値を作らない | 必須の5項目が読めたので値を作る |
| `pull_request` の `allowed_merge_methods` を省略し、GitHub 上は `["squash"]` で、ほかは宣言と同じ | Match。何も送らない | 省略を3方式とみなして Mismatch |
| 上の宣言で、`required_approving_review_count` が宣言と違う | Mismatch で適用不能。理由に `allowed_merge_methods` の観測値を添える | 更新して3方式に戻す |
| `allowed_merge_methods` が宣言と順序だけ違う | Match | 並びが違うので更新する |
| General で `allow_squash_merge` を無効にしたリポジトリで、`allowed_merge_methods: [squash]` を宣言した | 適用可能で、注記も添えない | マージできなくなるので適用不能にする。または、警告を添える |
| 取得の応答に `bypass_actors` がない | 観測操作の失敗 | Bypass がないとして空集合で読む |
| `target` が `tag` の Ruleset | 解釈しない部分を含み、値を作らない。存在はするため、名前は示す | 一覧から省く |
| 一覧が100件を返した | 観測操作の失敗 | 100件をすべての Ruleset とする |
| 宣言と同じ名前の Tag Ruleset がある | 解釈しない部分を含む観測不能で、適用不能。作成も計画しない（名前は `target` をまたいで一意） | 別の `target` なので Branch Ruleset を作成する。または、`tag` から `branch` へ変える |
| GitHub 上に `creation` を持つ Ruleset を、`pull_request` と `deletion` だけで宣言した | 解釈しない部分を含む観測不能で、適用不能 | 宣言にないルールなので外す |
| GitHub 上のステータスチェックが宣言と順序だけ違う | Match | 並びが違うので更新する |
| `do_not_enforce_on_create` を省略し、GitHub 上は `true` で、ほかは宣言と同じ | Match。何も送らない | 省略を `false` とみなして Mismatch。または、解釈しない部分として適用不能 |
| `do_not_enforce_on_create` を省略し、GitHub 上は `true` で、`strict_required_status_checks_policy` が宣言と違う | Mismatch で適用不能。理由に `do_not_enforce_on_create` の観測値を添える | 更新して `false` に変える。または、観測値を併送する |
| 取得の `integration_id` が `null` | 解釈しない部分を含み、値を作らない | 発行元を限らないチェックと読む |
| 非公開リポジトリで `rulesets` を宣言した | 比較不能で適用不能。Ruleset の要求を送らない。ほかの設定の照合は続ける | 一覧を送り、`403` で観測操作の失敗にする |
| 作成を承認したあと、外部主体が大文字小文字だけが違う名前の Ruleset を作った | 関係する現在値の変化。`POST` を送らず未実行 | 綴りが違うので作成する。または、同じ値なので完了 |
| 値の変更を承認したあと、外部主体がその Ruleset の名前を変えた | 宣言の名前の Ruleset が存在しなくなり、関係する現在値の変化。未実行 | `id` が同じなので、新しい名前の Ruleset へ `PUT` を送る |
| 存在しないことを宣言した Ruleset が `conditions: {}` で存在する | Mismatch で適用可能。`conditions: {}` を含む値の全体を示して削除する | 宣言できない値なので適用不能 |
| 存在しないことを宣言した Ruleset の省略できるパラメータが、GitHub が送らなかったときに決める値と違う | Mismatch で適用可能。削除は値を送らない | 省略したパラメータが変わるので適用不能 |
| 存在しないことを宣言した Ruleset が `creation` のルールを持つ | 解釈しない部分を含む観測不能で、適用不能。消える値の全体を示せない | 存在するので削除する |

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |
