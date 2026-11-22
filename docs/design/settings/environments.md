# Environments の設計

設定宣言の `environments`（[YAML 設定仕様](../../spec/configuration.md) 8章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。関連リソースに共通する概念は [ドメインモデル](../domain.md) の関連リソースと存在に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Environments | Environments | Environment と、その中の設定からなる設定の領域 |
| Environment | Environment | デプロイ先を表す、リポジトリに属する関連リソース。GitHub が ID を付ける。中の設定として、デプロイ元制限と Environment の保護設定を持つ |
| デプロイ元制限 | Deployment Branch Policy | Environment にデプロイできるブランチとタグを限る設定グループ。方式（`all`、`protected_branches`、`selected`）と、個別パターンで選ぶ方式（`selected`）のときに限って持つ個別パターンの集合からなる |
| 個別パターン | Deployment Pattern | デプロイ元制限の値の要素。種類（ブランチかタグか）と名前の組 |
| Environment の保護設定 | Environment Protection | デプロイを待たせる分数（`wait_timer`）、承認を求めるレビュアーと自己承認の禁止（`required_reviewers`）、管理者による迂回の可否（`can_admins_bypass`）。reco は観測するが、宣言を受け付けない |

Environment は値を持たない種類の関連リソースです。存在することの期待値は、存在することだけを表します。

デプロイ元制限は、ほかの項目の値によって持つかどうかが決まる集合を含む、一つの設定グループです。個別パターンが同じかどうかは、種類（ブランチかタグか）と名前の組で決まり、名前は大文字と小文字を区別します。方式が `selected` でないときの値は方式だけです。GitHub は方式が `selected` のあいだだけ個別パターンを受け付け、方式を `selected` から変えると個別パターンを残しません。そのため、このときの値は観測不能ではなく、方式だけの値として確かめられます。

期待値も同じ形をとり、方式が `selected` のときに限って、個別パターンの集合（空集合を含む）を宣言します。期待値と観測値の違いは、方式の違いと、加わる個別パターン（期待値にあって観測値にないもの）となくなる個別パターン（観測値にあって期待値にないもの）からなります。加わる個別パターンとなくなる個別パターンは、デプロイ元制限の一つの変更予定の内訳であり、それぞれが管理の単位、照合結果、変更予定を持つわけではありません。

Environment は、一覧と個別取得の応答が ID を含み、同じ名前で作り直すと ID が変わることを確かめた種類です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)）。そのため、GitHub が ID を付ける種類として扱い（[ドメインモデル](../domain.md) の関連リソースの同一性）、Change Execution が更新直前の再確認に Environment の ID を含めます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |

## 観測

Environment では、一覧について次を実際の動作で確かめています（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)）。一覧の成功応答は、確かめた件数の範囲で、作成と削除の直後も含めて、存在する Environment をすべて含みました。GitHub は Environment の名前を、ASCII の英字の大文字と小文字を区別せずに同じ名前とみなし、応答の名前には作成したときの綴りを返します。GitHub は、アクセントの有無、ASCII 以外の文字の大文字と小文字、全角と半角、末尾に付けた空白や制御文字の違いも同じ名前とみなしました。一方、先頭や内部に空白を加えた名前と、印字可能な ASCII 文字の記号を加えた名前は、確かめた範囲で別の名前でした。ページを分けたときの応答（`total_count` と、`Link` ヘッダーが示す URL の形）も確かめています。

GitHub の同一判定のすべてを reco は再現できないため、宣言の名前は同一性を判定できる名前（印字可能な ASCII 文字だけからなり、前後に空白を持たない名前）に限ります（[YAML 設定仕様](../../spec/configuration.md) 8.1節）。宣言の名前と一覧の名前の突き合わせと、それによる存在の観測結果は、[ドメインモデル](../domain.md) の同一性を判定できる名前と存在の観測に従います。この判定は、一覧から名前で Environment を引く一つの関数に閉じ、呼び出し側が一覧を綴りで探しません。一覧に、大文字小文字を区別せずに同じ名前が二つ以上あれば、取得契約に適合しない応答とします。GitHub は二つを同じ Environment とみなすはずであり、どちらを照合し更新するかが決まらないためです。個別パターンの重複と同じ扱いで、`show` も観測操作の失敗で止まります。

Environments は、`GET /repos/{owner}/{repo}/environments` の全ページで読みます。読み方は [アーキテクチャ設計](../architecture.md) 3.2節の一覧の読み方に従います。一覧の要素は個別取得の本文と全フィールドが一致するため（GitHub デプロイ環境 API 仕様 3.1）、Environment ごとの個別取得は送りません。デプロイ元制限の方式は `deployment_branch_policy` から決めます。`null` は `all`、`protected_branches` だけが真なら `protected_branches`、`custom_branch_policies` だけが真なら `selected` です。両方が同じ値の組は GitHub が更新で受け付けないため（同 1章）、取得で受けたら取得契約に適合しない応答とします。

個別パターンは、`GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` の全ページで読みます。この要求は、方式を `selected` と観測した Environment にだけ、一覧の並びで送ります。`selected` でないときは GitHub が `404` を返すためです（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 1章）。`selected` と観測したあとの失敗は、`404` を含めて観測操作の失敗とします。Environment の名前は、パスの一つの要素としてエスケープして送ります。`.` と `..` はエスケープしても別の経路を指すため、その名前の Environment の個別パターンは読まず、観測操作の失敗とします。

`plan` と `apply` も、`show` と同じく、宣言に名前のない Environment を含めて、方式を `selected` と観測したすべての Environment の個別パターンを読みます。そのどれかの読み取りに失敗すれば、宣言した Environment の計画も観測操作の失敗で止まります。宣言した Environment に限ると、観測結果に「方式は `selected` だが個別パターンを読んでいない」状態が要り、その状態を空集合と取り違えると、存在する個別パターンを加わる側に数えた計画を作り得るためです。

Environments に、取得条件による観測不能はありません。非公開リポジトリでも一覧と個別パターンを取得できたためです（GitHub デプロイ環境 API 仕様 1.1）。Environments の領域を読み取った観測結果では、宣言した名前の Environment は、一覧に同じ名前があれば存在し、なければ存在しません。例外は、同じ名前がなく、同一性を判定できる名前でない名前が一覧にあるときです。このとき、その Environment は名前の同一性を判定できないことを理由とする観測不能です。GitHub が宣言の名前をその名前と同じとみなすかを、reco は確かめられないためです。同じ名前が一覧にあれば、ほかに同一性を判定できる名前でない名前があっても存在します。

Environment の取得契約は、`id`、`name`、`deployment_branch_policy`、`protection_rules`、`can_admins_bypass` がそろうことです。`can_admins_bypass` は公式のスキーマにありませんが、常に返ることを確かめています（GitHub デプロイ環境 API 仕様 3.2）。保護ルールは `type` で読み分け、`wait_timer` と `required_reviewers` の値を読みます。それ以外の `type` の要素は読みません。個別パターンの取得契約は、`id`、`name`、`type` がそろい、`type` が `branch` か `tag` であることです。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |

## 適用可能性と副作用

名前の同一性を判定できない Environment の作成と、その中の設定の変更予定は、適用不能です。作成の要求が、GitHub が同じとみなす既存の Environment を変え得るためです（[ドメインモデル](../domain.md) の照合と変更予定）。それ以外の Environment とデプロイ元制限の変更予定は、[論理的な設定変更と更新操作](#論理的な設定変更と更新操作) の表のとおり適用可能です。

方式を `selected` から変える宣言によって個別パターンがなくなることは、デプロイ元制限という管理設定の値の変更であり、変更に伴う副作用には当たりません。個別パターンは同じ管理設定の値の一部で、未管理設定ではないためです。なくなる個別パターンは、変更予定の計画時の観測値に含まれて承認の対象になり、その更新操作の関係する現在値として更新直前に確かめます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 論理的な設定変更と更新操作

Environment とその `deployment_branch_policy` の変更予定は、次の表に従います。`PUT /repos/{owner}/{repo}/environments/{environment_name}` は作成と更新を兼ね、作成のときも本文の値を設定するためです（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)）。

| Environment の観測 | `deployment_branch_policy` の観測と宣言 | 変更予定 | 更新操作 | 適用可能性 |
|---|---|---|---|---|
| 存在する | 宣言しない、または宣言と同じ値（`selected` なら個別パターンの集合を含む） | なし | なし | なし |
| 存在する | 宣言と異なる値で、宣言が `selected` でない | 方式の変更（不一致） | (1) Environment の `PUT` | 適用可能。現在値が `selected` なら、なくなる個別パターンを現在値として示す |
| 存在する | 現在値が `selected` でなく、宣言が `selected` | 方式と個別パターンの変更（不一致） | (1) 方式を `selected` にする Environment の `PUT`、(2) 宣言した個別パターンごとの `POST`（宣言の順） | 適用可能 |
| 存在する | 現在値も宣言も `selected` で、個別パターンの集合が違う | 個別パターンの変更（不一致） | (1) 宣言にない個別パターンごとの `DELETE`（観測の順）、(2) 現在値にない個別パターンごとの `POST`（宣言の順）。`PUT` は送らない | 適用可能 |
| 存在しない | 宣言しない | 作成（不一致） | (1) Environment の `PUT` | 適用可能 |
| 存在しない | 宣言する（属する先が存在しないため比較不能） | 作成（不一致）とデプロイ元制限の変更（比較不能） | (1) 方式を本文に含む Environment の `PUT`、宣言が `selected` なら (2) 宣言した個別パターンごとの `POST`（宣言の順） | 適用可能 |
| 名前の同一性を判定できない | 問わない（同じ理由で比較不能） | 作成（比較不能）と、宣言していればデプロイ元制限の変更（比較不能） | なし | 適用不能。作成の要求が、GitHub が同じとみなす既存の Environment を変え得る |

存在しない Environment の作成と、その `deployment_branch_policy` の変更予定は、方式を本文に含む一つの作成の更新操作で始まる一つの論理的な設定変更にまとめます。加える個別パターンがなければ、更新操作はこの一つです（加える場合は後述）。完了を判断する期待状態は、Environment が存在し、方式が宣言した値であることです。作成と方式を二つの更新操作に分けると、作成のあとの中間状態で、宣言していない方式（作成の既定値）の Environment が残り得るためです。変更予定は管理設定ごとに二つのまま示し、集計でも二つに数えます（[CLI 仕様](../../spec/cli.md) 4章）。`deployment_branch_policy` を本文に含む作成の `PUT` が方式を設定することは、実際の動作で確かめています（GitHub デプロイ環境 API 仕様 4.1）。

期待値の方式が `selected` の変更予定は、表の順に、方式を変える `PUT`（方式が変わる場合だけ。作成ではその作成の `PUT`）、取り除く個別パターンごとの `DELETE`、加える個別パターンごとの `POST` を並べた一つの論理的な設定変更にします。GitHub は個別パターンを一つずつ加える要求と取り除く要求だけを受け付け（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 4章）、方式が `selected` でないあいだはパターンの一覧、取得、作成を `404` で拒否するためです（同 1章）。取り除く要素と加える要素は、計画時の観測値と宣言を種類と名前の組で比べて決め、承認のあとで決め直しません。

`DELETE` を `POST` より先に並べるのは、途中で止まったときに残る状態を、デプロイを許可する範囲が狭い側にするためです。`POST` を先にすると、遷移元も宣言も許可しないパターンの組み合わせ（取り除く前のパターンと加えたパターンの両方）が、中間状態として残ります。`DELETE` を先にすれば、中間状態の個別パターンは、`DELETE` のあいだは遷移元の集合の一部、`POST` のあいだは宣言の集合の一部になります。方式を `selected` にした `PUT` のあとは、個別パターンが一つもない状態から `POST` で宣言の集合に近づきます。reco は先行する更新操作を取り消しません。更新操作のあいだには更新要求の間隔（[アーキテクチャ設計](../architecture.md) 6.2節）が入るため、中間状態は少なくともそのあいだ残ります。

存在しない Environment の作成と個別パターンの追加も、一つの論理的な設定変更にします。二つに分けると、デプロイ元制限の一つの変更予定が二つの論理的な設定変更にまたがり（[ドメインモデル](../domain.md) の変更計画・論理的な設定変更・変更結果）、作成の論理的な設定変更が、宣言していない状態（`selected` でパターンが一つもない Environment）で完了と報告することになるためです。作成を本文なしにして方式を後続に回す案も採りません。作成のあとの中間状態で、すべてのブランチとタグを許可する方式が残り得るためです。作成のあと個別パターンで止まった場合も、変更結果の更新操作ごとの行が、作成が済んだことと止まった更新操作を示します。

各更新操作の変更前提で比べる関係する現在値は、Environment そのもの、デプロイ元制限の値（方式と個別パターンの集合）の順です。承認時に予定した値は次のとおりです。

| 更新操作 | Environment そのもの | デプロイ元制限の値 |
|---|---|---|
| 方式を `selected` にする `PUT` | 存在し、ID が計画時と同じ | 計画時の観測値 |
| 作成の `PUT` | 存在しない | 比べない（存在しないことの基準が成立すれば、同じ理由で必ず観測不能になるため） |
| 存在する Environment の `DELETE`、`POST` | 存在し、ID が計画時と同じ | 先行する更新操作による予定どおりの変化を反映した値 |
| 作成に続く `POST` | 先行する更新操作が作成して存在する | 先行する更新操作による予定どおりの変化を反映した値 |

予定どおりの変化を反映した値は、起点の値に、先行する `DELETE` が取り除いたパターンと、先行する `POST` が加えたパターンを反映したものです。起点の値は、方式の `PUT` のない変更では計画時の観測値、方式の `PUT` か作成のあとでは `selected` でパターンが一つもない値です。方式を `selected` に変えた直後と、`selected` を本文に含めて作った直後の一覧が空であることは、実際の動作で確かめています（GitHub デプロイ環境 API 仕様 4.1）。直後の一覧が空でなかった場合は、最初の `POST` の変更前提の不成立として止めます。パターンは種類と名前の組の集合として比べ、ID と並びは比べません。そのため、承認のあとに同じ種類と名前で作り直されたパターンは、変更前提を不成立にしません。

作成に続く更新操作では、承認の時点で Environment の ID が定まりません。作成に続く最初の更新操作では存在だけを確かめ、2つ目以降では、その最初の更新操作の更新直前の観測の ID と比べます。作成の `PUT` から、続く最初の更新操作の更新直前の観測までのあいだに、外部主体が Environment を消して同じ方式で作り直した場合は検出できません。作成の `PUT` の応答本文の `id` を基準にすれば検出できますが、更新の成功応答で値を確かめないという [アーキテクチャ設計](../architecture.md) 6.1節の定めに反するため使いません。

期待状態は、Environment が存在し、方式が `selected` で、個別パターンの集合が宣言と同じことです。最後の更新操作のあとに観測して確かめます。変更対象外の設定として比べる範囲は [更新方式](#更新方式) に従います。

作成の変更前提で比べる関係する現在値は、その名前の Environment が存在しないことだけです。更新直前に同じ名前の Environment が存在するか、同じかどうかを判定できない名前の Environment があれば、方式が宣言と同じでも関係する現在値の変化として更新要求を送りません。承認したのは作成であり、存在する Environment の設定を上書きする変更ではないためです。方式の変更の変更前提で比べる関係する現在値は、Environment が存在し ID が計画時と同じこと、方式が計画時の観測値と同じこと（`selected` なら個別パターンの集合を含む）です。この順に並べ、Environment そのものの変化を先に示します。期待状態は、作成では Environment が存在し、方式を宣言していれば方式が宣言した値であることです。方式の変更では、方式が宣言した値であることです。ID は期待状態で比べません。作成のあとに存在しないと確かめた場合は不一致、方式の変更のあとに存在しない場合は方式が観測不能、同じかどうかを判定できない名前が現れた場合は観測不能として、どれも完了にしません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |

## 更新方式

| 更新の対象 | 更新方式で守ること |
|---|---|
| Environment | `deployment_branch_policy` だけを送り、省略した保護設定の保持に依拠して更新する。方式を宣言していない作成は本文を送らない |
| Environment の個別パターン | 加える要求は種類と名前だけを送り、取り除く要求は更新直前の観測の ID を宛先にする。どちらも要素一つずつで、一覧の全体を置き換えない |

Environment では、`PUT /repos/{owner}/{repo}/environments/{environment_name}` に `deployment_branch_policy` だけを送ります。`all` は `null`、`protected_branches` は `protected_branches: true` と `custom_branch_policies: false`、`selected` は `protected_branches: false` と `custom_branch_policies: true` で表します。方式を宣言していない作成は本文を送りません。正常応答は作成も更新も `200` で、本文は個別取得と同じ形です。本文が Environment の取得契約を満たさない `200` と、ほかの `2xx`（`201` など）は、更新の契約の外として成否不明にします（[アーキテクチャ設計](../architecture.md) 6.1節）。方式を送る根拠は、次の実際の動作の確認です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 4.1）。

- `deployment_branch_policy` だけの `PUT` で、`wait_timer`、`reviewers`、`prevent_self_review`、`can_admins_bypass` と、それらの保護ルールの `id` が変わらなかった
- 存在する名前への本文のない `PUT` は、何も変えなかった
- 方式の両方 `true` と両方 `false` は `422` で、値は変わらなかった

更新後に再取得して更新直前の値と比べるのは、その名前の Environment の保護設定（`wait_timer`、`required_reviewers`、`can_admins_bypass`）です。作成では更新直前に Environment が存在せず、比べる値がありません。reco が種類を知らない保護ルールは観測結果に含めず、比べません。別の名前の Environment は比べません。別の名前の Environment を作成、更新、削除した前後で、ほかの Environment が `updated_at` を含む全フィールドと個別パターンまで一致したこと（GitHub デプロイ環境 API 仕様 4.1）を根拠に、保全されるとみなします。

更新要求の宛先は、存在する Environment では、変更前提で ID を確かめた更新直前の観測の綴りにし、作成では設定宣言の綴りにします。GitHub は ASCII の英字の大文字小文字だけが違う名前を同じ Environment とみなすため（GitHub デプロイ環境 API 仕様 2章）、作成の変更前提の「存在しないこと」も、大文字小文字を区別せずに一覧と比べます。比べ方を誤ると、存在する Environment に作成の要求を送り、宣言した方式で上書きします。名前は、パスの一つの要素としてエスケープしてから宛先に入れます。エスケープしない `/` は別の経路への要求になるためです（GitHub デプロイ環境 API 仕様 2章）。

Environment の個別パターンは、`POST /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` に `name` と `type` を送って一つ加え、`DELETE .../deployment-branch-policies/{branch_policy_id}` で一つ取り除きます。`type` は GitHub が省略を `branch` と読むため（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 4.1）、宣言した種類を必ず送ります。取り除く要求の宛先の ID は、その更新操作の更新直前の観測から、種類と名前で引きます。計画時の ID は使いません。承認のあとに同じ種類と名前で作り直されたパターンは ID が変わり（同 4.1）、計画時の ID では `404` になるためです。正常応答は、加える要求が `200` で本文は作ったパターン（本文が取得契約を満たさない `200` とほかの `2xx` は成否不明）、取り除く要求が本文のない `204`（ほかの `2xx` は成否不明）です。宛先の名前は Environment の `PUT` と同じくパスの一つの要素としてエスケープします。`..` と `/` を含む名前（`a/../b` など）でも、宣言した名前の Environment にだけ作用し、正規化した別の Environment には届かないことを実際の動作で確かめています（同 1章）。

この二つの更新エンドポイントで reco が観測する設定は個別パターンの集合だけで、それはこの論理的な設定変更が変える設定のため、更新後に更新直前の値と比べる設定はありません。保護設定を更新後に比べるのは、Environment の `PUT`（作成と方式の変更）を送った場合だけです。`wait_timer` の保護ルールを持ち `can_admins_bypass` が `false` の Environment で、個別パターンを加えても取り除いても、`updated_at` と保護ルールの `id` を含む個別取得の本文が変わらなかったことを、実際の動作で確かめています（同 4.2）。

個別パターンを加える要求に GitHub が返す `303`（同じ種類と名前のパターンがすでにある場合の応答で、パターンは作られない）も、追従せず成否不明とします（GitHub デプロイ元パターン API 仕様 4.1、[アーキテクチャ設計](../architecture.md) 5.2節）。成功や変更なしとは読み替えません。更新直前の観測で加えるパターンがないことを確かめてから送るため、この応答は、観測と要求のあいだに外部主体が同じパターンを加えた場合に限って起きます。エラーの示し方は [CLI 仕様](../../spec/cli.md) 7章に従います。

更新直前の観測と更新要求のあいだに、外部主体が同じ名前の Environment を作った場合は防げません。本文のない作成の要求は、存在する名前の Environment を何も変えません。方式を本文に含む作成の要求は、その Environment の方式を宣言した方式にします。外部主体が作った Environment の方式が `selected` で、宣言した方式が `selected` でない場合、この要求は個別パターンを失わせます。同じあいだに外部主体が、加えようとするパターンを先に加えた場合は `303` で成否不明に、取り除こうとするパターンを先に消した場合は `404` で更新操作の失敗になります。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) | Repository File |

## シナリオと反例

以下は Environment に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| Environments の領域を読み取り、Environment が一つもなかった | どの名前の Environment も存在しないと確かめた | 読み取らなかった領域と同じで、存在するかは分からない |
| デプロイ元制限の方式を `selected` と観測した直後に、個別パターンの取得を GitHub が見つからないと応答した | 観測操作の失敗。二つの読み取りのあいだに方式が変わった可能性がある | 個別パターンが空、または観測不能とする |
| 同じ名前のブランチの個別パターンとタグの個別パターンがある | 別の要素。種類と名前の組で区別する | 名前が同じなので一つの要素 |
| Environment `cafe` を宣言し、GitHub 上には `café` だけがあった | 名前の同一性を判定できないことを理由とする観測不能。Not Comparable で適用不能。GitHub は二つを同じ Environment とみなし得る | 大文字小文字を除いて綴りが違うので存在しない。作成を計画する |
| Environment `staging` を宣言し、GitHub 上に `staging` と `café` があった | `staging` は存在する。同じ名前が一覧にあれば、判定できない名前があっても存在は確かめられる | 判定できない名前があるので観測不能 |
| 存在しない Environment を、`deployment_branch_policy` とともに宣言した | Environment は Mismatch で作成、`deployment_branch_policy` は属する先が存在しないことを理由とする Not Comparable。どちらも適用可能で、変更予定は二つ、論理的な設定変更は一つ | 作成なので、変更予定は一つにまとまる。または、方式を観測できないので適用不能 |
| デプロイ元制限の方式が `selected` でない Environment を観測し、宣言も同じ方式だった | 値は方式だけで、Match。個別パターンは値に含まれない | 個別パターンを取得できないので Not Comparable |
| 個別パターンを持つ `selected` のデプロイ元制限に、別の方式を宣言した | 一つの管理設定の Mismatch で適用可能。個別パターンがなくなることは値の変更で、なくなる個別パターンは計画時の観測値として承認の対象になる | 個別パターンは宣言していないので、副作用として添える。または、未管理設定が失われるので適用不能 |
| `selected` のデプロイ元制限で、宣言した個別パターンと観測した個別パターンが、順序だけ違っていた | 同じ値で、Match | 並びが違うので Mismatch |
| `main` のブランチの個別パターンがある `selected` のデプロイ元制限に、`MAIN` のブランチの個別パターンを宣言した | 別の要素。`MAIN` が加わり `main` がなくなる、一つの管理設定の Mismatch | 大文字小文字だけの違いなので、同じ要素で Match |
| 存在しない Environment を、`selected` と個別パターンのデプロイ元制限とともに宣言した | 方式だけを宣言した場合と同じく、Environment は Mismatch で作成、デプロイ元制限は属する先が存在しないことを理由とする Not Comparable。どちらも適用可能で、変更予定は二つ | 個別パターンを観測できないので適用不能。または、個別パターンごとに変更予定がある |
| Environment `production` の作成を承認し、更新直前の一覧に `Production` があった | 同じ Environment が存在し、関係する現在値の変化。作成の更新要求を送らず未実行 | 綴りが違うので存在しない。作成の要求を送り、既存の `Production` の方式を上書きする |
| Environment `production` の作成を承認し、更新直前の一覧に `production` はなく、判定できない名前の `prodüction` が現れた | 存在しないことを確かめられず、関係する現在値の変化。未実行 | 同じ名前がないので存在しない |
| Environment の方式の変更のあと、方式は期待値と同じで、その Environment の `wait_timer` が変わっていた | 変更対象外の設定の予期しない変化で、変更結果は失敗。原因は断定しない | 期待状態が一致したので完了 |
| 方式を `selected` に変えたあと、最初の個別パターンを加える直前の一覧が空でなかった | 承認時に予定した値（パターンが一つもない）と違い、関係する現在値の変化 | 宣言に含まれるパターンなら、予定どおりの変化 |
| 個別パターンを加える要求に `303` が返った | 更新操作の成否不明。再送せず、後続を開始しない | 同じパターンがすでにあるので成功、または変更なし |
| 個別パターンだけを変える変更のあと、その Environment の `wait_timer` が変わっていた | 変更対象外の設定ではなく、予期しない変化として扱わない。個別パターンの更新エンドポイントで reco が観測するのは個別パターンの集合だけである | Environment の保護設定なので、変更対象外の設定の予期しない変化 |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [アーキテクチャ設計](../architecture.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [CLI 仕様](../../spec/cli.md)
- [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)
- [GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md)
