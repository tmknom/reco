# Environments の設計

設定宣言の `environments`（[YAML 設定仕様](../../spec/configuration.md) 9章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。関連リソースに共通する規則は [ドメインモデル](../domain.md) の関連リソースと存在に従い、ここでは繰り返しません。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Environments | Environments | Environment と、その中の設定からなる設定の領域 |
| Environment | Environment | デプロイ先を表す、リポジトリに属する関連リソース。値を持たない種類で、GitHub が ID を付ける。中の設定として、デプロイ元制限と Environment の保護設定を持つ |
| デプロイ元制限 | Deployment Branch Policy | Environment にデプロイできるブランチとタグを限る設定グループ。方式（`all`、`protected_branches`、`selected`）と、方式が `selected` のときに限って持つ個別パターンの集合からなる |
| 個別パターン | Deployment Pattern | デプロイ元制限の値の要素。種類（ブランチかタグか）と名前の組で、名前は大文字と小文字を区別する |
| Environment の保護設定 | Environment Protection | デプロイを待たせる分数（`wait_timer`）、承認を求めるレビュアーと自己承認の禁止（`required_reviewers`）、管理者による迂回の可否（`can_admins_bypass`）。reco は観測するが、宣言を受け付けない |
| Environment Variables | Environment Variables | Environment に属する Environment Variable からなる設定の領域。Environments とは別の設定の領域 |
| Environment Variable | Environment Variable | Environment に属し、名前と値を持つ関連リソース。GitHub は ID を付けないため、名前だけで識別する種類として扱う |

Environment は、一覧と個別取得の応答が ID を含み、同じ名前で作り直すと ID が変わることを確かめた種類です（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md)）。そのため、GitHub が ID を付ける種類として扱います（[ドメインモデル](../domain.md) の関連リソースの同一性）。

Environment Variable の管理の単位は、属する先の Environment の名前と変数の名前の組です。Environment の名前は Environment と同じ規則で、変数の名前と値は Repository Variable と同じ規則で比べます（[Actions の設計](actions.md) の設定と管理の単位）。

デプロイ元制限は、方式の値によって個別パターンの集合を持つかどうかが決まる設定グループです（[ドメインモデル](../domain.md) の一覧の全体を宣言する設定との違い）。GitHub は方式が `selected` のあいだだけ個別パターンを受け付け、方式を `selected` から変えると個別パターンを残しません。そのため、方式が `selected` でないときの値は観測不能ではなく、方式だけの値です。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `grounding` | [GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 観測

Environments は、`GET /repos/{owner}/{repo}/environments` の全ページで読みます（[GitHub API 通信の設計](../github-access.md) 2.3節）。一覧の成功応答が、作成と削除の直後も含めて存在する Environment をすべて含むことと、GitHub が ASCII の英字の大文字小文字だけが違う名前を同じ Environment とみなすことを、実際の動作で確かめています（[GitHub デプロイ環境 API 仕様](../../research/deployment-environments.md) 2章・3.1）。GitHub はほかの違い（アクセントの有無など）も同じとみなすため、宣言の名前は同一性を判定できる名前に限り（YAML 設定仕様 9.1節）、判定できない名前の扱いはドメインモデルの同一性を判定できる名前に従います。一覧に、Unicode の単純な大文字小文字の畳み込みで同じになる名前が二つ以上あれば、取得契約に適合しない応答とします。GitHub は二つを同じ Environment とみなすはずで、どちらを照合し更新するかが決まらないためです。一覧の要素は個別取得の本文と全フィールドが一致するため（同 3.1）、個別取得は送りません。

Environment の取得契約は、`id`、`name`、`deployment_branch_policy`、`protection_rules`、`can_admins_bypass` がそろうことです。`can_admins_bypass` は公式のスキーマにありませんが、常に返ることを確かめています（同 3.2）。方式は `deployment_branch_policy` から、`null` を `all`、`protected_branches` だけが真を `protected_branches`、`custom_branch_policies` だけが真を `selected` と読みます。二つが同じ値の組は GitHub が更新で受け付けないため（同 1章）、取得で受けたら取得契約に適合しない応答とします。保護ルールは `type` で読み分け、`wait_timer` と `required_reviewers` の値だけを読みます。

個別パターンは、`GET /repos/{owner}/{repo}/environments/{environment_name}/deployment-branch-policies` の全ページで読み、方式を `selected` と観測した Environment にだけ送ります。`selected` でないときは GitHub が `404` を返すためです（[GitHub デプロイ元パターン API 仕様](../../research/deployment-branch-policies.md) 1章）。`selected` と観測したあとの `404` は観測操作の失敗です。個別パターンの取得契約は、`id`、`name`、`type` がそろい、`type` が `branch` か `tag` であることです。

Environment Variables は、`GET /repos/{owner}/{repo}/environments/{environment_name}/variables` の全ページで、すべての個別パターンを読んだあとに読みます。`per_page`、1ページの本文の上限、要素の取得契約、名前の重複の扱いは、Repository Variables と同じです（[Actions の設計](actions.md) の観測）。重複は一つの Environment の一覧の中で判定します。ページ数の上限は、GitHub が一つの Environment に保存できる100件（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 3章）を1ページ30件で読み切る4ページです。一覧にあった Environment への `404` は、二つの読み取りのあいだに Environment がなくなった可能性があるため、観測操作の失敗とします。

個別パターンと Environment Variables は、Environment ごとに送る読み取り要求で、範囲は [GitHub API 通信の設計](../github-access.md) 2.4節に従います。Environment の名前は、パスの一つの要素として宛先に入れます（同 4章）。

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

Environments と Environment Variables に固有の適用不能の条件はありません。名前の同一性を判定できないことと、属する先が存在しないことによる適用可能性は、ドメインモデルの照合と変更予定に従います。変数を書いた Environment は `environments[]` に書いた Environment で、存在することを宣言しているため（YAML 設定仕様 9.3節）、属する先が宣言されていないことによる適用不能は生じません。非公開リポジトリでも Environment Variables の一覧、作成、更新を利用できたため（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 1.1）、公開範囲による適用不能も設けません。一つの Environment の変数の件数の上限（100件）と値の長さの上限は、計画の時点で確かめません。

Environment と Environment Variable の変更予定に、変更に伴う副作用と再開する設定はありません。方式を `selected` から変えて個別パターンがなくなることは、デプロイ元制限という管理設定の値の変更です（ドメインモデルの一覧の全体を宣言する設定との違い）。

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

変数の更新操作の変更前提で比べる関係する現在値は、Environment そのもの、変数の順です。計画時に存在した Environment では、Environment の ID も比べます。作り直された Environment の変数の一覧は空になるため（同 4.3）、変数だけを比べると、変数の作成を承認したあとに Environment が作り直されたことを検出できないためです。同じ変更計画が作成する Environment では ID が定まらず、変数が存在しないと観測できたことで Environment の存在もあわせて確かめます（[ドメインモデル](../domain.md) の関連リソースの同一性）。

論理的な設定変更は、Environment ごとに、存在（デプロイ元制限をまとめた作成を含む）、デプロイ元制限、変数（宣言の順）の順に並べ、Environment をまたいでは宣言の順に並べます。設定の領域の順（全 Environment のあとに全 Environment Variable）にしないのは、変更計画の行を設定ファイルの行と同じ順に並べ、作成から変数の更新直前の観測までのあいだを短くするためです。

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

更新後に比べるのは、宛先の Environment の保護設定（`wait_timer`、`required_reviewers`、`can_admins_bypass`）です。reco が種類を知らない保護ルールは観測しないため比べません。

宛先の名前は、存在する Environment では変更前提で ID を確かめた更新直前の観測の綴りにし、作成では設定宣言の綴りにします。作成の変更前提の「存在しないこと」は、大文字小文字を区別せずに一覧と比べます。比べ方を誤ると、存在する Environment に作成の要求を送り、宣言した方式で上書きするためです。

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
