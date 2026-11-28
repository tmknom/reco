# Environments の設計

設定宣言の `environments`（[YAML 設定仕様](../../spec/configuration.md) 11章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。関連リソースに共通する規則は [ドメインモデル](../domain.md) の関連リソースと存在に従い、ここでは繰り返しません。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Environments | Environments | Environment と、その中の設定からなる設定の領域 |
| Environment | Environment | デプロイ先を表す、リポジトリに属する関連リソース。値を持たない種類で、GitHub が ID を付ける。中の設定として、デプロイ元制限、担当者の集合、自己承認の禁止、デプロイの待機時間を持ち、ほかに Environment の保護設定を観測する |
| デプロイ元制限 | Deployment Branch Policy | Environment にデプロイできるブランチとタグを限る設定グループ。方式（`all`、`protected_branches`、`selected`）と、方式が `selected` のときに限って持つ個別パターンの集合からなる |
| 個別パターン | Deployment Pattern | デプロイ元制限の値の要素。種類（ブランチかタグか）と名前の組で、名前は大文字と小文字を区別する |
| 担当者 | Reviewer | Environment を使うデプロイジョブの実行を承認できる利用者かチーム。種類と ID の組で一つに定まる。種類が利用者の担当者は、観測では login を伴うが、login は識別に含めない |
| 担当者の集合 | Reviewer Set | Environment の中の設定で、YAML のキーは `reviewers`。`required_reviewers` の保護ルールが持つ担当者の集合を値とする。保護ルールがないことを空集合で表す |
| 担当者のいない保護ルール | Protection Rule without Reviewers | 担当者が1人もいない `required_reviewers` の保護ルール。担当者の集合の値の一つで、空集合とは別の値である。宣言では書けない |
| 担当者がいない状態 | No Reviewers | 担当者の集合が、空集合か担当者のいない保護ルールであること |
| 自己承認の禁止 | Prevent Self-Review | Environment の中の設定で、YAML のキーは `prevent_self_review`。デプロイジョブを起動した利用者が自分のジョブを承認できないかを表す真偽値。GitHub 上では `required_reviewers` の保護ルールが持つ |
| デプロイの待機時間 | Wait Timer | Environment の中の設定で、YAML のキーは `wait_timer`。ジョブが起動されてから実行されるまで待たせる分数 |
| 保護ルールの設定 | Protection Rule Setting | 担当者の集合、自己承認の禁止、デプロイの待機時間の三つ。GitHub の `required_reviewers` と `wait_timer` の保護ルールが持つ Environment の中の設定で、Environment の `PUT` にその設定だけを送って更新する |
| Environment の保護設定 | Environment Protection | デプロイの待機時間、`required_reviewers` の保護ルールの全体（担当者の集合と自己承認の禁止）、管理者による迂回の可否（`can_admins_bypass`）。reco はすべてを観測し、`can_admins_bypass` のほかは宣言を受け付ける |
| Environment Variables | Environment Variables | Environment に属する Environment Variable からなる設定の領域。Environments とは別の設定の領域 |
| Environment Variable | Environment Variable | Environment に属し、名前と値を持つ関連リソース。GitHub は ID を付けないため、名前だけで識別する種類として扱う |

Environment は、一覧と個別取得の応答が ID を含み、同じ名前で作り直すと ID が変わることを確かめた種類です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)）。そのため、GitHub が ID を付ける種類として扱います（[ドメインモデル](../domain.md) の関連リソースの同一性）。

Environment Variable の管理の単位は、属する先の Environment の名前と変数の名前の組です。Environment の名前は Environment と同じ規則で、変数の名前と値は Repository Variable と同じ規則で比べます（[Actions の設計](actions.md) の設定と管理の単位）。

デプロイ元制限は、方式の値によって個別パターンの集合を持つかどうかが決まる設定グループです（[ドメインモデル](../domain.md) の一覧の全体を宣言する設定との違い）。GitHub は方式が `selected` のあいだだけ個別パターンを受け付け、方式を `selected` から変えると個別パターンを残しません。そのため、方式が `selected` でないときの値は観測不能ではなく、方式だけの値です。

担当者の集合は、種類と ID の組の集合を値とする設定で、順序、重複、login を比べません（[ドメインモデル](../domain.md) 4.2節）。担当者のいない保護ルールを空集合と別の値にするのは、GitHub がこの保護ルールを返すことがあり、保護ルールが残ったままでは、宣言した「担当者がいない」状態と GitHub 上の状態が同じとは確かめられないためです。この保護ルールは、リポジトリの権限を持たない利用者だけを担当者として送ると作られ、`reviewers: []` を送るとなくなります（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 3.2・4.1）。この保護ルールがデプロイを止めるかは確かめていません。

自己承認の禁止とデプロイの待機時間は、どちらも GitHub が値を保存しない状態を持つ単独の設定です。GitHub は、`wait_timer` に `0` を送ると待機時間の保護ルールをなくし、`0` の保護ルールを返しません。担当者をすべて外すと `required_reviewers` の保護ルールごとなくし、自己承認の禁止の値を残しません。あとで担当者だけを設定すると、`prevent_self_review: false` の保護ルールを作ります（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 3.2・4.1）。そのため、待機時間の保護ルールがない観測を待機時間 `0`、`required_reviewers` の保護ルールがない観測を自己承認の禁止 `false` と読みます。どちらも、GitHub がその値を要素がないことで表すと確かめた読み方で、観測不能の補完ではありません（[ドメインモデル](../domain.md) 4.3節）。担当者のいない保護ルールの自己承認の禁止は、`false` とみなさず、保護ルールが返した値で読みます。GitHub はこの保護ルールに `prevent_self_review: true` を残すことがあり、そこへ担当者を送ると `true` のまま担当者が戻るためです（同 3.2）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 観測

Environments は、`GET /repos/{owner}/{repo}/environments` の全ページで読みます（[GitHub API 通信の設計](../github-access.md) 2.3節）。一覧の成功応答が、作成と削除の直後も含めて存在する Environment をすべて含むことと、GitHub が ASCII の英字の大文字小文字だけが違う名前を同じ Environment とみなすことを、実際の動作で確かめています（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 2章・3.1）。GitHub はほかの違い（アクセントの有無など）も同じとみなすため、宣言の名前は同一性を判定できる名前に限り（YAML 設定仕様 11.1節）、判定できない名前の扱いはドメインモデルの同一性を判定できる名前に従います。一覧に、Unicode の単純な大文字小文字の畳み込みで同じになる名前が二つ以上あれば、取得契約に適合しない応答とします。GitHub は二つを同じ Environment とみなすはずで、どちらを照合し更新するかが決まらないためです。一覧の要素は個別取得の本文と全フィールドが一致するため（同 3.1）、個別取得は送りません。

Environment の取得契約は、`id`、`name`、`deployment_branch_policy`、`protection_rules`、`can_admins_bypass` がそろうことです。`can_admins_bypass` は公式のスキーマにありませんが、常に返ることを確かめています（同 3.2）。方式は `deployment_branch_policy` から、`null` を `all`、`protected_branches` だけが真を `protected_branches`、`custom_branch_policies` だけが真を `selected` と読みます。二つが同じ値の組は GitHub が更新で受け付けないため（同 1章）、取得で受けたら取得契約に適合しない応答とします。保護ルールは `type` で読み分け、`wait_timer` と `required_reviewers` の値だけを読みます。担当者の集合は `required_reviewers` の保護ルールから読み、保護ルールがなければ空集合、保護ルールの `reviewers` が空なら担当者のいない保護ルールとします。自己承認の禁止は同じ保護ルールの `prevent_self_review` から、デプロイの待機時間は `wait_timer` の保護ルールから読み、保護ルールがなければそれぞれ `false` と `0` とします（設定と管理の単位）。担当者の login は、種類が利用者の担当者についてだけ `reviewer.login` から読み（同 3.2）、表示にだけ使います。login が欠けているときと、ASCII の英数字と `-` と `_` だけからなる空でない文字列でないときは、login のない担当者とし、観測操作の失敗にはしません。login は値に含めないため、欠けても照合に影響しません。形を限るのは、API が返した文字列を表示の行にそのまま書かないためです。担当者の `type` は GitHub が値を足し得る列挙として読み（公式の OpenAPI 記述は `User` と `Team`）、どれか一つの担当者の `type` が reco の知らない文字列なら、その Environment の担当者の集合だけを、値が知らない値であることを理由とする観測不能にします。知らない種類の担当者を除いた集合は観測値にしません。`reviewer` の形は担当者の種類ごとに違い得るため、知らない種類の担当者には `type` のほかの項目を求めません。自己承認の禁止は同じ保護ルールの真偽値で、観測値を持ちます。デプロイの待機時間、デプロイ元制限、変数も観測値を持ちます。

個別パターンは、`GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` の全ページで読み、方式を `selected` と観測した Environment にだけ送ります。`selected` でないときは GitHub が `404` を返すためです（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 1章）。`selected` と観測したあとの `404` は観測操作の失敗です。個別パターンの取得契約は、`id`、`name`、`type` がそろうことです。`type` は GitHub が値を足し得る列挙として読み（公式の OpenAPI 記述は `branch` と `tag`）、どれか一つの個別パターンの `type` が reco の知らない文字列なら、その Environment のデプロイ元制限（方式と個別パターンの組）だけを、値が知らない値であることを理由とする観測不能にします（[GitHub API 通信の設計](../github-access.md) 2.1節）。方式は一覧の要素から読めているため、Environment の `PUT` の前後の比較にはそのまま使います。GitHub は先頭か末尾に ASCII 以外の空白（U+00A0 など）を持つ名前のパターンを受け付けますが（同 4.1）、宣言はこの名前を受け付けません（YAML 設定仕様 11.2節）。そのため、GitHub 上にこの名前のパターンがあると宣言で一致させられず、`plan` は毎回、取り除く個別パターンとして示します。

Environment Variables は、`GET /repos/{owner}/{repo}/environments/{environment_name}/variables` の全ページで、すべての個別パターンを読んだあとに読みます。`per_page`、1ページの本文の上限、要素の取得契約、名前の重複の扱いは、Repository Variables と同じです（[Actions の設計](actions.md) の観測）。重複は一つの Environment の一覧の中で判定します。ページ数の上限は、GitHub が一つの Environment に保存できる100件（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 3章）を1ページ30件で読み切る4ページです。一覧にあった Environment への `404` は、二つの読み取りのあいだに Environment がなくなった可能性があるため、観測操作の失敗とします。

個別パターンと Environment Variables は、Environment ごとに送る読み取り要求で、範囲は [GitHub API 通信の設計](../github-access.md) 2.4節に従います。Environment の名前は、パスの一つの要素として宛先に入れます（同 4章）。`show` は一覧にあるすべての Environment の変数を読むため、名前が `.` か `..` の Environment が一覧にあると、方式によらず観測操作の失敗で止まります。GitHub がこの名前の Environment を作れるかは確かめていません。`show` が読む総量（Environment ごとに最大4ページ × 9 MiB）は Environment の数に比例し、全体の上限はありません。`plan` と `apply` は宣言した Environment だけを読むため、宣言していない Environment のこの名前では止まらず、総量も宣言した Environment の数に比例します。

Environments と Environment Variables に、取得条件による観測不能はありません。非公開リポジトリでも一覧と個別パターンと変数を取得できたためです（GitHub デプロイ環境 API 仕様 1.1、GitHub Actions 変数 API 仕様 1.1）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 適用可能性と副作用

Environment そのものと Environment Variables に固有の適用不能の条件はありません。名前の同一性を判定できないことと、属する先が存在しないことによる適用可能性は、ドメインモデルの照合と変更予定に従います。変数を書いた Environment は `environments[]` に書いた Environment で、存在することを宣言しているため（YAML 設定仕様 11.4節）、属する先が宣言されていないことによる適用不能は生じません。非公開リポジトリでも Environment Variables の一覧、作成、更新を利用できたため（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 1.1）、公開範囲による適用不能も設けません。一つの Environment の変数の件数の上限（100件）と値の長さの上限は、計画の時点で確かめません。

担当者の集合にも、固有の適用不能の条件はありません。存在しない利用者の ID と、リポジトリの権限を持たない利用者は、計画の時点で確かめません。GitHub はこれらを次のように扱います（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- 存在しない利用者の ID だけを送ると `422` を返す。更新操作の失敗として報告される
- 権限を持たない利用者だけを送ると `200` を返し、担当者のいない保護ルールができる。更新後の期待状態との不一致として報告される
- 担当者にできる利用者と一緒に送ると `200` を返し、担当者にできない利用者を黙って除く。更新後の期待状態との不一致として報告される

非公開リポジトリでの宣言も適用不能にしません。GitHub Free の非公開リポジトリでは、担当者を送る `PUT` は、担当者をなくす `reviewers: []` も含めて `422` になりますが、reco は料金プランを観測できず、非公開リポジトリで担当者を使えるプランもあるためです。担当者と自己承認の禁止の組み合わせ（担当者が1人のときにその人のデプロイを誰も承認できなくなるなど）は、reco が設定の望ましさを評価しないため、判定せず注記も添えません。

自己承認の禁止には、固有の適用不能の条件があります。自己承認の禁止の変更予定は、更新時点に担当者がいない状態になる見込みで、`prevent_self_review` を送らずには期待値にならないとき、適用不能です。GitHub は、担当者がいない状態への `prevent_self_review` の `PUT` を、`true` も `false` も `422` で拒むためです（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- **更新時点の担当者**：担当者の集合を宣言していればその期待値、宣言していなければ計画時の観測値で決める。宣言していない担当者の集合は管理範囲外で、reco は変えないためである。存在しない Environment で担当者の集合を宣言していなければ、担当者がいない。宣言していない担当者の集合が、値が知らない値であることを理由に観測不能なら、担当者がいる。観測不能になるのは、知らない種類の担当者が1人以上いるときに限るためである
- **送らずに期待値になる場合**：期待値が `false` で、担当者の集合の宣言が `[]` か、Environment を作成する場合である。どちらも、更新後に `required_reviewers` の保護ルールがなく、`false` と読むため（論理的な設定変更と更新操作）
- **`reviewers: []` と `prevent_self_review: true`**：期待値だけで成り立たないと決まるため、入力検証で拒否する（[YAML 設定仕様](../../spec/configuration.md) 11.5節）

値が知らない値であることを理由に比較不能になった担当者の集合とデプロイ元制限の変更予定は、適用不能です（[ドメインモデル](../domain.md) の照合結果と適用可能性）。

デプロイの待機時間に固有の適用不能の条件はありません。GitHub Free の非公開リポジトリでは、`wait_timer` を含む `PUT` は `0` も含めて `422` になり何も変えませんが、担当者と同じ理由で適用不能にせず、更新操作の失敗として報告します。

担当者をすべて外す変更予定は、同じ Environment の自己承認の禁止を変えます。担当者をすべて外すと `required_reviewers` の保護ルールごとなくなり、その保護ルールの `prevent_self_review` も失われるためです（GitHub デプロイ環境 API 仕様 4.1）。変化を示す形は、自己承認の禁止の宣言の有無で次のように決まります。どの場合も、計画時の値を変更前提で確かめます（論理的な設定変更と更新操作）。

| 自己承認の禁止の宣言 | 計画時の値 | 示す形 |
|---|---|---|
| なし | `true` | 値が失われることを `Side effect:` で示す（表示の形は [CLI 仕様](../../spec/cli.md) 4章） |
| なし | `false` | 示さない。保護ルールがないときと同じく、自己承認を禁じない状態のまま変わらないため |
| `false` | `true` | 自己承認の禁止の変更予定（`true -> false`）。担当者の集合の論理的な設定変更に加わる |
| `false` | `false` | 一致 |

宣言した `false` に変わることを副作用として示さないのは、宣言した設定が期待値どおりになる変化は、未管理設定の変化ではなく管理設定の変更だからです（[ドメインモデル](../domain.md) 3.5節）。

担当者をすべて外す変更予定を除き、Environment と Environment Variable の変更予定に、変更に伴う副作用と再開する設定はありません。方式を `selected` から変えて個別パターンがなくなることは、デプロイ元制限という管理設定の値の変更です（ドメインモデルの一覧の全体を宣言する設定との違い）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 論理的な設定変更と更新操作

Environment とそのデプロイ元制限の変更予定は、次の表に従います。`PUT /repos/{owner}/{repo}/environments/{environment_name}` は作成と更新を兼ね、作成のときも本文の値を設定します（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

| Environment の観測 | デプロイ元制限の観測と宣言 | 変更予定 | 更新操作 |
|---|---|---|---|
| 存在する | 宣言と異なり、宣言が `selected` でない | 方式の変更 | (1) Environment の `PUT` |
| 存在する | 現在値が `selected` でなく、宣言が `selected` | 方式と個別パターンの変更 | (1) 方式を `selected` にする `PUT`、(2) 宣言した個別パターンごとの `POST`（宣言の順） |
| 存在する | 現在値も宣言も `selected` で、個別パターンの集合が違う | 個別パターンの変更 | (1) 宣言にない個別パターンごとの `DELETE`（観測の順）、(2) 現在値にない個別パターンごとの `POST`（宣言の順） |
| 存在しない | 宣言しない | 作成 | (1) 本文のない `PUT` |
| 存在しない | 宣言する（比較不能） | 作成とデプロイ元制限の変更 | (1) 方式を本文に含む `PUT`、宣言が `selected` なら (2) 宣言した個別パターンごとの `POST`（宣言の順） |

表の各行は、一つの論理的な設定変更です。

- **作成と方式をまとめる理由**：作成と方式を二つの更新操作に分けると、作成のあとの中間状態で、宣言していない方式（作成の既定値はすべてのブランチとタグを許可する）の Environment が残り得るためである。方式を本文に含む作成の `PUT` が方式を設定することは、実際の動作で確かめている（GitHub デプロイ環境 API 仕様 4.1）
- **作成と個別パターンの追加をまとめる理由**：二つに分けると、デプロイ元制限の一つの変更予定が二つの論理的な設定変更にまたがり、作成の論理的な設定変更が、宣言していない状態（`selected` でパターンが一つもない）で完了と報告することになるためである
- **個別パターンを一つずつ送る理由**：GitHub は個別パターンを一つずつ加える要求と取り除く要求だけを受け付ける（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 4章）
- **`DELETE` を `POST` より先にする理由**：途中で止まったときに残る状態を、デプロイを許可する範囲が狭い側にするためである。`POST` を先にすると、取り除く前のパターンと加えたパターンの両方を許可する、遷移元も宣言も許可しない組み合わせが残る。`DELETE` を先にすれば、中間状態の集合は、`DELETE` のあいだは遷移元の一部、`POST` のあいだは宣言の一部になる

変更前提で比べる関係する現在値は、Environment そのもの、デプロイ元制限の値の順です。

| 更新操作 | Environment そのもの | デプロイ元制限の値 |
|---|---|---|
| 存在する Environment の `PUT` | 存在し、ID が計画時と同じ | 計画時の観測値 |
| 作成の `PUT` | 存在しない | 比べない |
| 存在する Environment の `DELETE`、`POST` | 存在し、ID が計画時と同じ | 先行する更新操作による予定どおりの変化を反映した値 |
| 作成に続く `POST` | 先行する更新操作が作成して存在する | 先行する更新操作による予定どおりの変化を反映した値 |

方式の `PUT` か作成のあとの起点の値は、`selected` でパターンが一つもない値です。方式を `selected` に変えた直後と、`selected` を本文に含めて作った直後の一覧が空であることを確かめています（GitHub デプロイ環境 API 仕様 4.1）。

Environment Variable の変更予定は、変数ごとに一つの更新操作からなる一つの論理的な設定変更で、作成は `POST .../environments/{environment_name}/variables`、値の変更は `PATCH .../variables/{name}`、削除は `DELETE .../variables/{name}` です。Environment の作成とまとめないのは、デプロイ元制限と違い、変数のない Environment が Environment の存在という期待状態を満たしており、変数の作成は別の更新要求だからです。存在しない Environment の中で存在しないことを宣言した変数は、作成の論理的な設定変更の期待状態に加え、更新操作を持ちません。作られたばかりの Environment の変数の一覧が空で返ることを確かめています（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 4.3）。

担当者の集合、自己承認の禁止、デプロイの待機時間の変更予定は、次の表に従います。**作成直後の値** は、担当者の集合が空集合、自己承認の禁止が `false`、待機時間が `0` です。どの方式を本文に含めた作成の `PUT` も、本文のない作成の `PUT` も、`required_reviewers` と `wait_timer` の保護ルールを持たない Environment を作ることを確かめています（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

| Environment の観測 | 宣言 | 論理的な設定変更 | 更新操作 |
|---|---|---|---|
| 存在する（その設定が宣言と異なる） | 宣言する | その設定だけ | (1) その設定だけを送る Environment の `PUT` |
| 存在する（担当者の集合と自己承認の禁止が宣言と異なる） | 担当者の集合が `[]`、自己承認の禁止が `false` | 担当者の集合と自己承認の禁止 | (1) `reviewers: []` を送る Environment の `PUT` |
| 存在しない | 作成直後の値と異なる値を宣言する（比較不能） | 作成とは別 | (1) その設定だけを送る Environment の `PUT` |
| 存在しない | 作成直後の値と同じ値を宣言する（比較不能） | 作成の論理的な設定変更の期待状態に加える | なし |

- **作成とこれらの設定を分ける理由**：担当者を含めた作成の `PUT` は、担当者を拒否する `422` を返しても Environment を作ることがあるためである（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。reco は `4xx` のあとに値を読み直さないため、まとめると、作成が残ったのに更新操作の失敗と `Desired state: not checked` を示す。自己承認の禁止は作成と同時には送れず（担当者がいない状態では `422`）、待機時間も非公開リポジトリでは `422` になり得るため、同じく分ける。作成は、実際の動作で確かめた本文の `PUT` のまま送る。作成からこれらの設定までのあいだと、設定が完了しなかったときには、担当者も待機時間もない Environment が残る。この状態は、変更前の Environment が存在しない状態より保護を弱めないため受け入れる。存在しない Environment を参照するワークフローを実行すると、GitHub は保護ルールのない Environment をその名前で作り、ジョブはそれを使うためである（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 1.1）。デプロイ元制限は作成にまとめるのに担当者をまとめないのは、デプロイ元制限の方式は拒否されない値で送れるのに対し、担当者は値によって `422` で拒否されるためである
- **作成直後の値と同じ宣言を作成にまとめる理由**：送らずに期待状態が成り立ち、GitHub Free の非公開リポジトリで不要な `422` を生まない（[ドメインモデル](../domain.md) 4.8節の照合と変更予定）。担当者の集合のない Environment への `prevent_self_review: false` は、送れば `422` になる
- **自己承認の禁止を担当者の集合と別の `PUT` にする理由**：一つの `PUT` にまとめると、`422` で拒否したときに先に処理した担当者の変更を残すことがあり（同 4.1）、「何も変わっていない」と読める失敗の報告と実際の状態が食い違うためである。担当者の `PUT` と自己承認の禁止の `PUT` のあいだは「担当者がいて、自己承認の禁止が変更前の値（担当者がいなかったなら `false`）」の状態で、変更前より保護を弱めない
- **担当者を外す変更に `prevent_self_review: false` を加える理由**：担当者を外したあとの担当者がいない状態に `prevent_self_review` を送ると `422` になるため、別の更新操作にはできない

担当者の集合、自己承認の禁止、待機時間の更新操作の変更前提で比べる関係する現在値は、Environment そのもの、その設定の順です。計画時に存在した Environment では、Environment の ID と、その設定の計画時の観測値を比べます。同じ変更計画が作成する Environment では、その設定が作成直後の値であることだけを比べ、その値を観測できたことで Environment の存在もあわせて確かめます。

- **担当者をすべて外す更新操作**：担当者の集合のあとに、同じ Environment の自己承認の禁止が計画時の値のままであることを、宣言の有無によらず比べる。承認のあとに外部主体が `true` にした値を、変更計画に示さないまま失わせないためである（[ドメインモデル](../domain.md) 4.7節）
- **担当者の更新操作に続く自己承認の禁止の更新操作**：承認時に予定した自己承認の禁止の値は、計画時の観測値と同じである。保護ルールのない Environment に担当者を設定すると `false` の保護ルールが作られ、担当者の入れ替えは値を保つためである（GitHub デプロイ環境 API 仕様 3.2・4.1）。担当者の集合は、この更新操作の変更前提で比べない。担当者がいない状態で送れば GitHub は `422` で拒み、黙って成功しないためである

更新要求の本文と、更新後に比べる変更対象外の設定は、更新方式の各節が定めます。

同じ Environment に複数の設定の変更予定があるときも、それぞれ別の論理的な設定変更とし、別々の `PUT` で送ります（[アーキテクチャ設計](../architecture.md) 5.3節）。Environment の `PUT` は、一部のフィールドを `422` で拒否したときも、先に処理したフィールドの変更を残すことがあります（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

変数の更新操作の変更前提で比べる関係する現在値は、Environment そのもの、変数の順です。計画時に存在した Environment では、Environment の ID も比べます。作り直された Environment の変数の一覧は空になるため（同 4.3）、変数だけを比べると、変数の作成を承認したあとに Environment が作り直されたことを検出できないためです。同じ変更計画が作成する Environment では ID が定まらず、変数が存在しないと観測できたことで Environment の存在もあわせて確かめます（[ドメインモデル](../domain.md) の関連リソースの同一性）。

論理的な設定変更は、Environment ごとに、存在（デプロイ元制限をまとめた作成を含む）、デプロイ元制限、担当者の集合、自己承認の禁止、デプロイの待機時間、変数（宣言の順）の順に並べ、Environment をまたいでは宣言の順に並べます。自己承認の禁止を担当者の集合のあとにするのは、担当者がいない状態で送ると GitHub が `422` で拒むためです。設定の領域の順（全 Environment のあとに全 Environment Variable）にしないのは、変更計画の行を設定ファイルの行と同じ順に並べ、作成から変数の更新直前の観測までのあいだを短くするためです。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 更新方式

### Environment

`PUT /repos/{owner}/{repo}/environments/{environment_name}` に `deployment_branch_policy` だけを、観測と同じ表現で送ります。方式を宣言していない作成は本文を送りません。正常応答は作成も更新も `200` で、本文は個別取得と同じ形です。根拠は次の実際の動作の確認です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- `deployment_branch_policy` だけの `PUT` で、`wait_timer`、`reviewers`、`prevent_self_review`、`can_admins_bypass` と、それらの保護ルールの `id` が変わらなかった
- 存在する名前への本文のない `PUT` は、何も変えなかった
- 方式の両方 `true` と両方 `false` は `422` で、値は変わらなかった
- 別の名前の Environment を作成、更新、削除した前後で、ほかの Environment が `updated_at` を含む全フィールドと個別パターンまで一致した

更新後に比べるのは、宛先の Environment の保護設定（デプロイの待機時間、担当者の集合、自己承認の禁止、`can_admins_bypass`）です。待機時間と自己承認の禁止は、照合と同じく、保護ルールがない観測を `0` と `false` と読んで比べます（設定と管理の単位）。保護ルールのない状態から `prevent_self_review: true` の保護ルールが現れた変化も、自己承認の禁止の変化として示すためです。reco が種類を知らない保護ルールは観測しないため比べません。担当者の集合が、更新直前か更新後の片方で知らない値のために観測不能で、もう片方で観測値なら、予期しない変化とします。両方で観測不能なら、担当者の集合は比べません（[ドメインモデル](../domain.md) の変更計画・論理的な設定変更・変更結果）。比べないときの保全の根拠は、`wait_timer` だけ、`prevent_self_review` だけ、`deployment_branch_policy` だけの `PUT` が担当者を変えなかった確認結果です。

Environment の `PUT` の更新操作（作成、方式、担当者の集合、自己承認の禁止、デプロイの待機時間）は、どれも同じ範囲を更新後に比べます。デプロイ元制限の方式と保護設定のうち、その論理的な設定変更が変えない設定と、変更に伴う副作用として示した設定です。作成は更新直前に Environment がなく、比べる値がありません。

宛先の名前は、存在する Environment では変更前提で ID を確かめた更新直前の観測の綴りにし、作成では設定宣言の綴りにします。作成の変更前提の「存在しないこと」は、大文字小文字を区別せずに一覧と比べます。比べ方を誤ると、存在する Environment に作成の要求を送り、宣言した方式で上書きするためです。

### 担当者の集合

Environment の `PUT` に `reviewers` だけを送ります。要素は `type` と `id` の組で、設定宣言の順に並べます。担当者をすべて外すときは `reviewers: []` を送り、`null` は送りません。観測した担当者のいない保護ルールは宣言できない値のため、送ることはありません。正常応答は `200` で、本文は個別取得と同じ形です。根拠は次の実際の動作の確認です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- `reviewers` だけの `PUT` で、`prevent_self_review`、`wait_timer`、`can_admins_bypass`、デプロイ元制限の方式と個別パターン（同じ `id`）が変わらなかった
- 存在する Environment への、担当者だけを送った `PUT` の `422` では、何も変わらなかった
- 担当者にできない ID を担当者にできる利用者と一緒に送ると、`200` を返して担当者にできない ID を黙って除いた

担当者をすべて外す変更では、副作用として示した自己承認の禁止と、同じ論理的な設定変更の期待状態に加えた自己承認の禁止を、更新後に比べません。

### 自己承認の禁止

Environment の `PUT` に `prevent_self_review` だけを、`true` か `false` で送ります。正常応答は `200` で、本文は個別取得と同じ形です。根拠は次の実際の動作の確認です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- `prevent_self_review` だけの `PUT` で、担当者、`wait_timer`、`can_admins_bypass`、デプロイ元制限の方式と個別パターン（同じ `id`）、保護ルールの `id` が変わらなかった
- 担当者がいない状態への `PUT` は、`true` も `false` も `422` だった（適用可能性と副作用）

### デプロイの待機時間

Environment の `PUT` に `wait_timer` だけを、分の整数で送ります。`0` も送り、GitHub は待機時間の保護ルールをなくします。正常応答は `200` で、本文は個別取得と同じ形です。根拠は次の実際の動作の確認です（同 4.1）。

- `wait_timer` だけの `PUT`（`0` を含む）で、担当者、`prevent_self_review`、`can_admins_bypass`、デプロイ元制限の方式と個別パターン（同じ `id`）が変わらなかった
- `1` から `43199` までの値は反映され、`0` は保護ルールをなくした。`43200` は `200` を返して値を変えなかった（YAML 設定仕様 11.5節で入力不正にしている）
- GitHub Free の非公開リポジトリでは、`0` を含めて `422` を返し、何も変えなかった

### 個別パターン

`POST /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` に `name` と `type` を送って一つ加え、`DELETE .../deployment-branch-policies/{branch_policy_id}` で一つ取り除きます。

- **`type` を必ず送る**：GitHub は省略を `branch` と読む（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 4.1）
- **取り除く宛先の ID**：その更新操作の更新直前の観測から、種類と名前で引く。承認のあとに同じ種類と名前で作り直されたパターンは ID が変わり、計画時の ID では `404` になるためである（同 4.1）
- **正常応答**：加える要求は `200` で本文は作ったパターン、取り除く要求は本文のない `204`
- **`303`**：同じ種類と名前のパターンがすでにある場合に、加える要求へ GitHub が返す（同 4.1）。パターンは作られない。追従せず成否不明とし（[アーキテクチャ設計](../architecture.md) 5.2節）、成功や変更なしとは読み替えない。更新直前の観測で加えるパターンがないことを確かめてから送るため、観測と要求のあいだに外部主体が同じパターンを加えた場合に限って起きる

宛先の名前に `..` と `/` を含めても（`a/../b` など）、宣言した名前の Environment にだけ作用することを確かめています（同 1章）。この二つの更新エンドポイントで reco が観測する設定は個別パターンの集合だけで、それは論理的な設定変更が変える設定のため、更新後に比べる変更対象外の設定はありません。個別パターンを加えても取り除いても、`updated_at` と保護ルールの `id` を含む Environment の個別取得の本文が変わらなかったことを確かめています（同 4.2）。

### Environment Variable

本文、正常応答、GitHub の拒否の扱いは Repository Variable と同じです（[Actions の設計](actions.md) の更新方式、[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 5章）。宛先の Environment の名前は、変更前提で存在を確かめた更新直前の観測の綴りにし、変数の名前は宣言の綴りにします。存在しない Environment への作成は `404` で拒否され Environment を作りませんが（同 1章）、reco は更新直前の観測で Environment の存在を確かめてから送るため、この応答に頼りません。

更新エンドポイントは変数ごとに別で、reco がそこで観測するのはその変数だけのため、更新後に比べる変更対象外の設定はありません。変数の作成、値の変更、削除の前後で、Environment の `id` と `updated_at`、同じ Environment のほかの変数、別の Environment とその変数、同じ名前の Repository Variable が変わらなかったことを確かめており（同 5.4）、これを保全の根拠とします。

### 防げない競合

更新直前の観測と更新要求のあいだの外部主体の変更は、次のとおり防げません。

- 同じ名前の Environment が作られた場合、本文のない作成はそれを何も変えず、方式を本文に含む作成はその方式を宣言した方式にする。外部主体が作った Environment の方式が `selected` で、宣言した方式が `selected` でなければ、個別パターンが失われる
- 加えようとするパターンが先に加えられた場合は `303` で成否不明、取り除こうとするパターンが先に消された場合は `404` で更新操作の失敗になる
- 方式、担当者の集合、デプロイの待機時間の `PUT` の前に Environment が削除された場合、`PUT` は宣言した方式、担当者、待機時間を持つ Environment を作る。自己承認の禁止の `PUT` は、担当者がいないため `422` になる。担当者の `PUT` は `422` で拒否されたときも Environment を作ることがある（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。更新後の確認は Environment の ID を比べないため、作られた Environment の値が期待状態と変更対象外の設定の両方に一致すれば、完了と報告し得る

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## シナリオと反例

以下は Environment に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| Environment `cafe` を宣言し、GitHub 上には `café` だけがあった | 名前の同一性を判定できないことを理由とする観測不能。比較不能で適用不能。GitHub は二つを同じ Environment とみなし得る | 大文字小文字を除いて綴りが違うので存在しない。作成を計画する |
| Environment `production` の作成を承認し、更新直前の一覧に `Production` があった | 同じ Environment が存在し、関係する現在値の変化。作成の更新要求を送らず未実行 | 綴りが違うので存在しない。作成の要求を送り、既存の `Production` の方式を上書きする |
| 同じ名前のブランチの個別パターンとタグの個別パターンがある | 別の要素。種類と名前の組で区別する | 名前が同じなので一つの要素 |
| `main` のブランチの個別パターンがある `selected` のデプロイ元制限に、`MAIN` のブランチの個別パターンを宣言した | 別の要素。`MAIN` が加わり `main` がなくなる、一つの管理設定の不一致 | 大文字小文字だけの違いなので、同じ要素で一致 |
| デプロイ元制限の方式が `selected` でない Environment を観測し、宣言も同じ方式だった | 値は方式だけで、一致。個別パターンは値に含まれない | 個別パターンを取得できないので比較不能 |
| 存在しない Environment を、`selected` と個別パターンのデプロイ元制限とともに宣言した | 変更予定は作成とデプロイ元制限の二つで、論理的な設定変更は一つ。方式を本文に含む `PUT` のあとに個別パターンを加える | 作成を本文なしで送り、方式を後続の `PUT` で変える |
| 方式を `selected` に変えたあと、最初の個別パターンを加える直前の一覧が空でなかった | 承認時に予定した値（パターンが一つもない）と違い、関係する現在値の変化 | 宣言に含まれるパターンなら、予定どおりの変化 |
| 個別パターンを加える要求に `303` が返った | 更新操作の成否不明。再送せず、後続を開始しない | 同じパターンがすでにあるので成功、または変更なし |
| Environment の方式の変更のあと、方式は期待値と同じで、その Environment の `wait_timer` が変わっていた | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 期待状態が一致したので完了 |
| 個別パターンだけを変える変更のあと、その Environment の `wait_timer` が変わっていた | 比べない。個別パターンの更新エンドポイントで reco が観測するのは個別パターンの集合だけである | Environment の保護設定なので、変更対象外の設定の予期しない変化 |
| 存在しない Environment `staging` に、`variables` だけを書いた | Environment は不一致で作成、変数は属する先が存在しないことを理由とする比較不能で作成。論理的な設定変更は Environment の作成と変数ごとに一つずつ | `variables` だけでは Environment の存在を宣言しないので適用不能。または、作成と変数の作成を一つにまとめる |
| GitHub 上に担当者のいない保護ルールがある Environment に、`reviewers: []` を宣言した | 不一致。保護ルールをなくす変更予定になる | 担当者が1人もいないので、空集合と同じで一致 |
| 担当者の集合の観測が `[User 1, User 2]` で、宣言が `[User 2, User 1]` だった | 一致。順序は値に含めない | 並びが違うので不一致 |
| GitHub 上の担当者に `type: Team` の要素があり、宣言は利用者だけだった | 不一致。Team の担当者を外す変更予定になる | 宣言で受け付けない種類なので比べない |
| 存在しない Environment `preview` に、`reviewers: []` を書いた | Environment は不一致で作成、担当者の集合は属する先が存在しないことを理由とする比較不能。担当者の集合は作成の期待状態に加わり、更新操作を持たない | `reviewers: []` を送る更新操作を作成のあとに並べる |
| 担当者が1人で `prevent_self_review: true` の Environment に、`prevent_self_review` を書かずに `reviewers: []` を宣言した | 担当者を外す変更予定に `Side effect:` で `prevent_self_review` が失われることを示し、承認すれば実行する。更新後に `prevent_self_review` がなくなっても予期しない変化ではない | 宣言していない設定が失われるので適用不能。または、更新後に値がなくなったので予期しない変化で失敗 |
| 担当者が1人で `prevent_self_review: false` の Environment の担当者を外す変更を承認したあと、外部主体が `prevent_self_review` を `true` にした | 副作用を示さなかった設定の変化で、変更前提は不成立。未実行 | 変えるのは担当者の集合だけなので、変更前提は成立する |
| 担当者を `[User 1]` から `[User 2]` に変える更新のあと、`prevent_self_review` が `true` から `false` になっていた | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 同じ保護ルールの項目なので、担当者の更新の結果として比べない |
| 担当者のいない Environment に担当者を設定する更新のあと、`prevent_self_review: false` の保護ルールが作られた | 予期しない変化ではない。更新直前は保護ルールがなく `false` と読み、更新後も `false` である | 更新直前は値がなかったので、`false` になったことは予期しない変化 |
| 担当者のいない Environment に担当者を設定する更新のあと、`prevent_self_review: true` の保護ルールが作られた | 変更対象外の設定の予期しない変化で、変更結果は失敗 | 更新直前に保護ルールがなく比べる値がないので、比べない |
| 担当者が1人で `prevent_self_review: true` の Environment に、`reviewers: []` と `prevent_self_review: false` を宣言した | 担当者の集合と自己承認の禁止の二つの変更予定からなる、一つの論理的な設定変更。`reviewers: []` の `PUT` だけを送り、`Side effect:` は添えない | 自己承認の禁止を `false` にする `PUT` を別に送る。または、宣言していても副作用として示す |
| 保護ルールのない Environment に、`reviewers` を書かずに `prevent_self_review: true` を宣言した | 自己承認の禁止の変更予定が適用不能。担当者がいない状態では GitHub が拒む | 入力検証で拒否する。または、`PUT` を送って `422` を待つ |
| 保護ルールのない Environment に、`reviewers` を書かずに `prevent_self_review: false` を宣言した | 一致。保護ルールがない観測を `false` と読む | 値を観測できないので比較不能、または `PUT` を送る |
| `prevent_self_review: true` の担当者のいない保護ルールがある Environment に、`reviewers` を書かずに `prevent_self_review: false` を宣言した | 不一致で、適用不能。GitHub はこの保護ルールの `true` を返し、担当者がいない状態では変えられない | 担当者がいないので `false` とみなし、一致 |
| 保護ルールのない Environment に、担当者1人と `prevent_self_review: true` を宣言した | 担当者の集合、自己承認の禁止の順に、別々の論理的な設定変更と `PUT` | 一つの `PUT` にまとめる。または、自己承認の禁止を先に送る |
| 待機時間の保護ルールのない Environment に `wait_timer: 0` を宣言した | 一致。`PUT` を送らない | 値を観測できないので比較不能、または `wait_timer: 0` を送る |
| 存在しない Environment に担当者を1人宣言し、作成は成功して担当者の `PUT` が `422` で拒否された | 作成は完了、担当者の設定は失敗。担当者のいない Environment が残るが、存在しなかった状態より保護を弱めない | 作成と担当者を一つの `PUT` にして、作成ごと拒否させる |
| 存在する Environment の変数 `REGION` の削除を承認したあと、Environment が同じ名前で作り直され、同じ名前と値の変数が作られていた | Environment の ID が違い、関係する現在値の変化。未実行で、作り直された Environment の変数を消さない | 変数の名前と値が承認時と同じなので削除する |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [アーキテクチャ設計](../architecture.md)
- [GitHub API 通信の設計](../github-access.md)
- [Actions の設計](actions.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)
- [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md)
- [GitHub Actions 変数 API 仕様](../../research/actions-variables.md)
