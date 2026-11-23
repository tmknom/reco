# Rulesets の設計

リポジトリの Ruleset の設計を定めます。設定宣言の `rulesets` はまだ受け付けておらず、本文書は観測と、その単位を定めます。文書の構成は [設定ごとの設計](README.md) に従います。関連リソースに共通する規則は [ドメインモデル](../domain.md) の関連リソースと存在に従い、ここでは繰り返しません。

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

Ruleset の中身は項目ごとの管理の単位に分けず、値の全体を一つの値とします（ドメインモデルの管理の単位の形）。GitHub の更新は、送った `rules` で全体を置き換えるためです（GitHub Rules API 仕様 4.2）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |

## 観測

### 読み取り要求

Rulesets は、`GET /repos/{owner}/{repo}` の応答の `private` が偽のときだけ、Private vulnerability reporting のあとに読みます。真のときは Ruleset の要求を一つも送らず、Rulesets の一覧を、リポジトリが非公開であることを理由とする観測不能にします（下の取得条件による観測不能）。公開と観測したあとの一覧と取得の失敗は、`403` と `404` を含めて観測操作の失敗です。`rulesets` は YAML 設定仕様にまだ章がないため、観測の順（[GitHub API 通信の設計](../github-access.md) 2.2節）では `security` のあとに置きます。

一覧は `GET /repos/{owner}/{repo}/rulesets?includes_parents=false&per_page=100` で読みます。`includes_parents=false` は、Organization 所有のリポジトリで、上位で構成された Ruleset が一覧に加わるのを避けるためです（GitHub Rules API 仕様 3.1）。一覧は `total_count` を返さないため、`total_count` のない一覧の読み方（[GitHub API 通信の設計](../github-access.md) 2.3節）で読み、ページ数の上限は1ページです。GitHub が一つのリポジトリに置ける Ruleset は75個で（GitHub Rules API 仕様 1章）、`per_page` の上限の100より少ないためです。1ページ目が100件を返したら、最後のページに達しないため観測操作の失敗とします。1ページの本文の上限は既定の値（同 2.1節）です。

一覧の要素は `id` と `name` しか使えません。`conditions`、`rules`、`bypass_actors` を含まないためです（GitHub Rules API 仕様 3.1）。値は、Ruleset ごとに `GET /repos/{owner}/{repo}/rulesets/{ruleset_id}` で読みます。この取得は関連リソースごとの読み取り要求で、範囲は GitHub API 通信の設計 2.4節に従います。状態確認は、一覧のすべての Ruleset に一覧の並びで送ります。詳細の取得は、一覧のすべての要素を読み、名前の重複を確かめてから送ります。

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

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Rules API 仕様](../../research/rules.md) | Repository File |
