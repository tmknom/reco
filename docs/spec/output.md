# 表示仕様

## 1. 目的と適用範囲

本ドキュメントは、`plan`、`apply`、`show` が stdout へ表示する内容の外部契約を定めます。コマンド、オプション、承認の入力、出力先、エラーの形、終了コードは [CLI 仕様](cli.md) が定めます。

外部契約とするのは、表示の構造、固定の語（行のラベル、結果区分、`absent` と `present`、全体の結論を示す行）、各行が伝えなければならない情報と示してはならない情報、並びと数え方です。理由文、注記、原因の候補などの文言と、値の細かい表記（単位の書き方、字下げ、項目の並びなど）は外部契約にせず、コードとテストを正本とします。本ドキュメントの表示例も形を示すためのもので、例の文言と表記は外部契約ではありません。この線引きの理由は [ドキュメント規約](../contributing/docs-policy.md) 1章が定めます。

本ドキュメントは、どの設定にも当てはまる表示の規則だけを定めます。どの変更予定が適用不能になり、どの副作用を伴い、どの順に実行し、変更前提で何を比べるか、どの設定がどの理由で観測不能になるか、その設定の行が何を示すかは、[設定ごとの設計](../design/settings/README.md) が定めます。

## 2. 表示の原則

表示には、reco の実行が終わったあとも残る影響と、利用者が判断や行動を変えられる情報だけを示します。更新のあいだだけ生じて実行が完了すれば残らない状態や、更新を何回に分けて送るかといった内部の手順は示しません。利用者が何もできない情報は、判断に必要な情報を埋もれさせ、残り続ける影響だと読み違えさせるためです。実行が完了しなかった場合は、残った状態を確かめるのに要る範囲で、どの更新まで済んだかを示します（4.3節）。

すべての表示に、次の規則が当てはまります。

- **設定の名前**：管理設定は設定宣言のキーを `.` でつないだ名前（`general.allow_auto_merge`）で、リポジトリリソースとその中の設定は `<種類のキー>.<名前>` と `<種類のキー>.<名前>.<設定>` の形で示す。名前は設定宣言に書いた綴りのまま示す。GitHub が大文字小文字の違う綴りで返しても、設定ファイルの行と対応づけられるようにするため
- **文字列の値**：リポジトリリソースの名前、変数の値、パターンなど、利用者か GitHub が決める文字列は、二重引用符で囲んで制御文字をエスケープし、1行で示す。長い値も省略しない。引用符で囲むことで、値が固定の語（`absent` など）と見分けられ、前後の空白や末尾の改行だけの違いも見える。省略した値は、何に変わるかを利用者が確かめられないため
- **秘密情報**：認証情報は示さない（[CLI 仕様](cli.md) 3章）。Repository Variables と Environment Variables の値は秘密情報として扱わず、そのまま示す。GitHub は Variables を機密でない設定情報の保存先としており、値はワークフローの出力でもマスクされないため（[GitHub Actions 変数 API 仕様](../research/actions-variables.md) 3章）
- **reco の知らない値**：GitHub が reco の知らない列挙値を返したときは、その値そのものを示さず、知らない値であることを示す。GitHub が返した、reco の検証していない文字列を表示に入れないため。値は GitHub の設定画面で確かめられる
- **GitHub の並び**：GitHub から観測した集合や一覧は、GitHub が返した順に、重複も残して示す。観測した値を並べ替えたり補ったりすると、推測した値を観測値として示すことになるため

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `QR-001` | Quality Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Actions 変数 API 仕様](../research/actions-variables.md) | Repository File |

## 3. `plan`：変更計画

`plan` は、選択したリポジトリの変更計画を表示します。変更計画は、`apply` が承認の前に示すものと同じです（4.1節）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-007` | Functional Requirement |

### 3.1 全体の構成

変更計画は、次の順に示します。

1. 読んだ設定ファイルを示す `Configuration:` の行
2. 変更予定のあるリポジトリごとの `Repository:` の節。設定宣言の `repositories` の並びで示し、節のあいだを空行で区切る。変更不要のリポジトリは節を持たない
3. 集計の `Plan:` の行と `Repositories checked:` の行
4. 適用不能な変更予定が一つでもあれば、空行を挟んで `Plan cannot be applied.` を一度だけ

変更の必要がないと確認できた場合に限り、1から3の代わりに `No changes.` と `Repositories checked:` の行だけを示します。

```text
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Repository: example/project-c
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true
    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)

Plan: 2 repositories to update, 2 changes
Repositories checked: 3

Plan cannot be applied.
```

```text
No changes.
Repositories checked: 2
```

### 3.2 表示区分

変更予定は、次の区分で示します。

| 表示区分 | 意味 | 利用者に示す情報 |
|---|---|---|
| 確認済みの差分 | 比較可能な現在値と期待値が異なる | リポジトリ、設定、現在値、期待値 |
| 比較不能な設定予定 | 現在値が観測不能で比較できず、期待状態の適用を検討する | リポジトリ、設定、観測不能の理由から分かるリポジトリの状態、期待値 |
| 適用不能 | 安全な変更に必要な条件を確認できない | 対象と理由。差分・比較不能の区分とは別に示す |
| 変更不要 | 管理範囲内の設定が期待状態と一致する | 確認したリポジトリの数 |

照合結果と適用可能性は別に示します。不一致でも適用不能な場合があり、比較不能でも条件を確認できれば適用可能な変更予定となります。観測操作そのものの失敗は比較不能に含めず、エラーとして報告します（[CLI 仕様](cli.md) 5章）。

### 3.3 変更予定の行

変更予定は、`<設定の名前>: <現在値> -> <期待値>` の形で示します。値が1行に収まらない設定（項目を並べた設定グループ、集合を値とする設定など）は、設定の行の下に現在値と、`->` に続く期待値を1行ずつ示します。どの設定がどちらの形になるかは外部契約にしません。

現在値の位置には、観測した値か、次の語を置きます。

- **`absent`**：リポジトリリソースが存在しないと確認したこと。確かめた事実であり、比較不能の印である括弧には入れない。リポジトリリソースの作成は、現在値の位置に `absent` を置き、期待値の位置には、値を持たない種類では `present` を、値を持つ種類では宣言した値を置く
- **括弧で囲んだ状態**：現在値が観測不能で比較できないこと。観測不能の理由から分かるリポジトリの状態を示す（下の表）。確認済みの差分とは、現在値が括弧で示されることで区別する

リポジトリリソースの削除は、期待値の位置に `absent` を置き、現在値の位置に失われる値を示します。値が1行に収まらないリポジトリリソースの作成と削除は、値の位置に `present` を置き、宣言した値か失われる値の全体を続く行に示します。値を持たず中の設定を持つ種類（Environment）の削除も、`present -> absent` の行の下に、削除で失われる値の全体（[ドメインモデル](../design/domain.md) の削除で失われる値と副作用）を続く行に示します。失われる設定がない項目も省かず、その状態を値として示します。利用者が、reco が確かめたうえで失われるものがないことと、reco が示していないことを区別できるようにするためです。この種類で失われる値の全体を示せない削除は、続く行を示さず、適用不能の行を添えます（3.4節）。存在しないリポジトリリソースに存在しないことを宣言した場合は一致であり、行を示しません。宣言に名前のないリポジトリリソースは、変更計画に示しません。

括弧の状態は、観測したリポジトリの状態から決まり、推測した値ではありません。GitHub が保存している値を示すものでもありません。利用者が知りたいのは承認すると何が起きるかであり、その判断には、いま効いている状態と宣言した値があれば足ります。状態の種類は次のとおりで、どの設定がどの状態になるかは設定ごとの設計の「観測」が定めます。

| 状態の種類 | 表示例 | 意味 |
|---|---|---|
| 上位の設定が無効 | `(GitHub Actions disabled)` | 上位の設定が無効で、この設定は効いていない |
| 上位の設定が別の方式 | `(not in use)` | この設定を使う方式が選ばれていない |
| 公開範囲 | `(public repository)`、`(private repository)` | この設定はリポジトリの公開範囲に適用されないか、利用できない |
| 属する先がない | `(environment absent)` | 属する先のリポジトリリソースがないため、この設定もない |
| 存在を判定できない | `(existence unknown)` | 宣言した名前と同じかどうかを判定できない名前のリポジトリリソースがあり、その名前のリポジトリリソースが存在するかを確かめられない |
| 解釈しない部分 | `(uninterpreted parts)` | リポジトリリソースは存在するが、reco はその値の全体を読み取れない |
| 知らない値 | `(unknown value)` | GitHub は値を返したが、reco はその値の意味を知らない |
| 値が返らない | `(not returned)` | GitHub がこの設定の値を応答に含めず、reco は現在の値を確かめられない |

属する先がない設定とリポジトリリソース（存在しない Environment の中の設定と変数など）の変更予定は、属する先の作成とは別の変更予定として示します。

値の変更が利用者に読み取りにくい場合は、次のように差分を添えます。

- **集合の差分**：現在値と期待値がどちらも集合である確認済みの差分には、値の行の下に、なくなる要素を `... to remove:`、加わる要素を `... to add:` の行で、この順に添える。なくなる要素は現在値の順、加わる要素は期待値の順に並べ、該当する要素がない行は示さない。二つの一覧の並びが違っても、利用者が差を目で突き合わせなくて済むようにするため。集合として比べるため、順序だけの違いと、GitHub 上の重複した要素は示さない
- **全体を値とする設定の変更**：一覧の全体を値とするリポジトリリソース（Ruleset）の値の変更は、設定の行に値を置かず、変わる部分だけを続く行に示す。宣言が値の全体であり、宣言にない要素も外すため、何が外れるかを全体を見比べずに読めるようにするため
- **期待値に伴う上位の設定の変化**：期待値が、宣言した設定のほかに上位の設定の状態も変える場合は、期待値に括弧でそのことを添える。どの場合に添えるかは設定ごとの設計が定める

```text
Repository: example/project-a
  environments.production: absent -> present
  environments.production.deployment_branch_policy: (environment absent) -> protected_branches
  environments.preview.deployment_branch_policy:
    selected [branch "feature/*", branch "main"]
    -> selected [branch "main", branch "release/*"]
    Patterns to remove: [branch "feature/*"]
    Patterns to add: [branch "release/*"]
  environments.legacy: present -> absent
    deployment_branch_policy: selected [branch "main"]
    reviewers: [User 583231 (octocat)]
    prevent_self_review: true
    wait_timer: 30 minutes
    can_admins_bypass: true
  actions.variables.GO_VERSION: "1.26" -> "1.27"
  actions.variables.LEGACY_FLAG: "enabled\n" -> absent

Plan: 1 repository to update, 6 changes
Repositories checked: 1
```

### 3.4 変更予定に添える行

変更予定に添える行には次の四つがあり、変更予定の値の行と差分の行のあとに、この順に示します。どれも変更計画の一部として stdout へ表示し、`apply` では承認プロンプトの前に表示します。どの変更予定にどの行を添え、何を示すかは、設定ごとの設計の「適用可能性と副作用」が定めます。

- **`Applicability: not applicable (<理由>)`**：適用不能な変更予定に添える。理由は、何が成り立っていないかと、設定ファイルの直し方を示す。宣言を消す直し方では、消すキーを示す。マッピングの唯一のキーである設定は、そのマッピングのキーを示す。設定のキーだけを消すと値のないマッピングが残り、入力検証で拒否されるため（[YAML 設定仕様](configuration.md) 14.1節）。理由がリポジトリリソースの値か中の設定の、reco が解釈しない部分であるときは、この行の下に、解釈しなかったものを `show` と同じ形で1行に一つずつ示す（5.3節）。利用者が、既存のリポジトリリソースのどこを reco が扱えないかを、承認を求められる前に読めるようにするため
- **`Note:`**：変更予定の注記（[ドメインモデル](../design/domain.md) の計画）を示す。宣言した変更が GitHub 上で何を意味するかの事実を述べ、reco による設定の良し悪しの評価は含めない。適用可能性を変えず、承認とは別の確認も求めない。宣言の外で起きる変化ではないため、`Side effect:` とは別の行にする。変更結果の末尾の注記（4.3節）とは、字下げと示す場面で区別される
- **`Previous settings:`**：再開する設定（[ドメインモデル](../design/domain.md) の計画）を示す。値を示せないときは、残っていた値で再開することと、値を示せない理由を示す。値が失われるわけではないため、`Side effect:` とは別の行にし、`Side effect:` の前に示す
- **`Side effect:`**：宣言した変更に伴って GitHub が管理範囲外の設定を削除するか変える場合に、その設定と、reco が観測する設定なら現在値を示す。reco が観測しない設定は、起きる変化だけを示す。値を取得できず失われるかどうかが分からないときは、失われる可能性と値を示せない理由を示す。リポジトリリソースの削除に伴って、それに属するリポジトリリソースが消える場合は、種類ごとに1行で、消えるリポジトリリソースの名前と値を示す。GitHub が値を返さない種類では、名前と、値を示せないことを示す。消えるリポジトリリソースが一つもない種類の行は示さない

`Side effect:` を添えた変更予定は適用可能であり、利用者が承認すれば実行します。`Side effect:` は、変更が完了したあとも残る影響だけを示します。一つの変更の更新操作のあいだだけ生じる中間状態（[アーキテクチャ設計](../design/architecture.md) 5.4）は示しません。完了すれば残らず、利用者が承認の前に避ける手段もないためです。中間状態が残るのは変更が完了しなかった場合で、そのときは変更結果の `failed` と、変更結果の注記で示します（4.3節）。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)
    Previous settings: GitHub Actions resumes with sha_pinning_required kept from before it was disabled (reco cannot read it while GitHub Actions is disabled)
    Side effect: GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (reco cannot read them while GitHub Actions is disabled)
  environments.old-preview: present -> absent
    deployment_branch_policy: all
    reviewers: []
    prevent_self_review: false
    wait_timer: 0 minutes
    can_admins_bypass: true
    Side effect: GitHub also deletes the environment variables "REGION": "ap-northeast-1", "TIER": "1"
    Side effect: GitHub also deletes the environment secrets "DEPLOY_KEY", "TOKEN" (reco cannot read their values)
  security.dependabot_alerts: true -> false
    Side effect: GitHub also disables the dependency graph
```

### 3.5 並びと数え方

一つのリポジトリの変更予定は、[YAML 設定仕様](configuration.md) の章と表の並びで示し、設定宣言での記述順には従いません。リポジトリリソースの変更予定は、同じ章の中で、設定宣言でそのリポジトリリソースを書いた順に示します。名前の並びは YAML 設定仕様の表から決まらないためです。リポジトリリソースの中の設定と、それに属するリポジトリリソースの変更予定は、属するリポジトリリソースの変更予定の直後に、設定宣言に書いた順で示します。

`Plan:` の行は、変更予定のあるリポジトリの数と変更予定の数を示します。変更予定のあるリポジトリの数は、そのすべてが適用可能であることを意味しません。変更予定の数は次のように数えます。

- 確認済みの差分と比較不能な設定予定を合わせて、管理設定の単位で数える。二つを区別するのは変更予定の行の現在値であり、集計では区別しない。比較不能な設定予定も、宣言した値へ変える変更として利用者が承認するものだからである
- 設定グループは、項目の数によらず一つと数える。リポジトリリソースの値（Ruleset の値など）は、リポジトリリソース一つを一つと数える
- リポジトリリソースの作成と削除は、確認済みの差分として数える。存在しないと確認できたことは、比較できる事実だからである。属する先の作成と、その中の設定の変更予定は、それぞれ数える。削除は、失われる値の項目の数と、副作用として示した属するリポジトリリソースの数によらず、一つと数える。副作用は管理設定の変更予定ではないためである
- 適用不能な変更予定と、適用不能な変更予定だけを持つリポジトリも数える
- 更新要求の数や論理的な設定変更の数とは同一視しない

`Repositories checked:` の行は、変更予定の有無によらず、選択したリポジトリの数を示します。

## 4. `apply`：承認前の提示と変更結果

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-008` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `INV-005` | Invariant |

### 4.1 承認前の提示

`apply` は、承認の前に、変更計画を3章の `plan` と同じ形で示します。確認済みの差分と比較不能な設定予定は、`plan` と同じく現在値の示し方で区別できます。適用不能な変更予定があれば、変更計画を最後まで示し、承認を求めずに止まります。すべて変更不要と確認できた場合は、`plan` と同じく `No changes.` と `Repositories checked:` の行を示します。承認の受け付け方は [CLI 仕様](cli.md) 4章が定めます。

### 4.2 変更結果の構成

変更結果は、`Applying changes...` の行に続けて、選択したすべてのリポジトリを、設定宣言の `repositories` の並びで示します。結果は、リポジトリごとに次のとおり示します。

- **変更不要だった**：リポジトリの行に `no change` を示す。先行するリポジトリが完了しなかった場合も同じである
- **すべての論理的な設定変更が完了した**：リポジトリの行に `updated` を示す
- **論理的な設定変更が1件で、完了しなかった（先行するリポジトリが完了しなかった場合を除く）**：リポジトリの行に `failed` か `not attempted` を示し、理由の行（4.3節）を添える
- **論理的な設定変更が2件以上で、完了しなかったものを含む（先行するリポジトリが完了しなかった場合を除く）**：リポジトリの行に結果を示さず、論理的な設定変更ごとに結果を実行順に示し、完了しなかったものに理由の行を添える
- **先行するリポジトリが完了しなかったため、どの論理的な設定変更も開始しなかった**：論理的な設定変更の数によらず、リポジトリの行に `not attempted` と理由の行を示す

すべてのリポジトリが `updated` か `no change` のときに限り、`Apply complete.` と、`updated` のリポジトリの数を示す `Repositories updated:` の行を示します。`no change` のリポジトリは数に含めません。

結果区分は次のとおりです。

| 結果区分 | 意味 |
|---|---|
| `updated` | 必要な更新操作の成功と、論理的な設定変更全体の期待状態の一致を確認でき、変更対象外の設定の予期しない変化を検出しなかった |
| `failed` | 更新操作の明示的な拒否、更新要求の成否不明（HTTP リダイレクト応答を含む）、更新開始後の前提不成立、成功応答後の状態確認失敗・不一致、変更対象外の設定の予期しない変化などによって変更が完了しなかった。副作用がなかったことは意味しない |
| `not attempted` | 当該の論理的な設定変更について、更新操作を一度も開始していない。開始直前の前提不成立で開始できなかった場合と、先行する異常によって実行を止めた場合を含む |
| `no change` | 管理範囲内の設定が期待状態と一致することを確認でき、更新が不要だった |

論理的な設定変更ごとに結果を示すときは、論理的な設定変更を、それが変える管理設定の名前で示します。二つ以上の管理設定を変える論理的な設定変更は、変更計画に示す順に ` + ` でつないで示します（`general.allow_squash_merge + general.allow_merge_commit`）。同じ管理設定を変える更新操作が複数あっても、その管理設定の名前は一度だけ示します。設定グループの項目は、個別の変更として示しません。変更結果を一つにまとめても、承認の前に提示する変更予定は管理設定ごとの行のままです。どの変更予定を一つの論理的な設定変更にまとめるかは、設定ごとの設計の「論理的な設定変更と更新操作」が定めます。

リポジトリリソースの作成と削除も、完了すれば `updated` と示します。何を作成し、何を削除したかは、承認の前に提示した変更予定が示します。リポジトリリソースの変更結果はリポジトリリソースの名前だけを示し、値と更新要求の本文を示しません。削除でも、失われた値を示しません。値は承認の前に提示しており、変更結果で繰り返すと、長い値が結果を埋もれさせるためです。

### 4.3 理由の行と変更結果の注記

完了しなかった論理的な設定変更には、次の理由の行を添えます。理由の行が示さなければならない情報は次のとおりで、文言は外部契約ではありません。

- **`Update request`**：更新操作ごとの結果を、`succeeded`、`rejected`、`unknown`、開始しなかったことを示す `not started` で示す。複数の更新操作からなる論理的な設定変更では、更新操作を開始したあとで完了しなかった場合に、`Update request (<対象>): <結果>` の形で実行順に示す。`<対象>` は更新操作が変える管理設定で、管理設定の一部の要素だけを変える更新操作（個別パターンの追加など）はその要素で示す。続けて並ぶ同じ種類と結果の更新操作は1行にまとめてよく、そのため1行が複数の更新要求を表すことがある。`not started` は更新操作についての表示で、論理的な設定変更の `not attempted` とは区別する
- **`Precondition:`**：変更前提の不成立で更新操作を開始できなかったときに、変わっていた設定と、それが承認のあとの変化か、先行する更新操作のあとの値が予定どおりでなかったのかを示す。リポジトリリソースそのものの変化はリポジトリリソースの名前で示す。原因（GitHub の反映の遅れか、外部主体の変更か）は断定しない。複数の設定が変わっていた場合は一つだけ示し、リポジトリリソースそのものと、その中の設定か属するリポジトリリソースが両方変わっていればリポジトリリソースそのものを、それ以外では論理的な設定変更が変える設定を優先する。リポジトリリソースの削除で、承認の前に示した失われる値か、副作用として示した属するリポジトリリソースが変わっていた場合は、リポジトリリソースの名前で示す。どれが変わっていても、削除が承認の前に示した内容と違うことに変わりはないためである。GitHub の取得条件や知らない値のために確認できなかった値を確認できるようになった場合と、その逆も、変化として示す。承認した先行する変更による予定どおりの変化は示さない。先行する更新操作が成功しなかったために開始しなかった場合は、この行を足さない
- **`Stopped:`**：先行する異常で実行を止めたときに、止めた理由が同じリポジトリの先行する論理的な設定変更か、先行するリポジトリかを区別して示す
- **`Desired state:`**：更新後に期待状態と比べた結果を、下の表の区分で示す
- **`Other settings:`**：変更対象外の設定の予期しない変化を、管理設定と同じ形の名前と、更新直前の値と更新後の値で示す。reco に起因するとは断定しない。変更対象外の設定には、同じリポジトリのほかの管理設定も含む。一方で GitHub が知らない値を返し、もう一方で値を返した設定は変化として示し、知らない値の側を `(unknown value)` と示す。設定グループの省略した項目の変化は、`<管理設定の名前>.<項目のキー>` の名前で示す。どの設定を比べるかは、設定ごとの設計の「更新方式」が定める
- **`Possible cause:`**：更新操作が成功したのに期待状態と一致しないときに、設定ごとの設計が原因の候補を挙げていれば、可能性の形で示す。reco は原因を断定しない。更新操作の成否が不明なときは示さない

| `Desired state:` の区分 | 意味 |
|---|---|
| `matches` | 期待状態と一致した。更新操作の成否が不明なら、そのことを添え、`updated` にはしない |
| `does not match` | 期待値と異なる値か、存在しないはずのリポジトリリソースの存在を観測した。二つ以上の管理設定を変える論理的な設定変更では、期待値と異なる値を観測した管理設定を変更計画に示す順に添える。期待値と比べる設定が観測不能だった管理設定は、値を確認できていないため添えない |
| `not in effect` | 期待値と比べる設定が、GitHub の取得条件か属する先の不在のため観測不能だった。理由を添え、利用者が確かめるべきリポジトリの状態を示す。値そのものは確認できていないため、`does not match` とは示さない |
| `not confirmed` | 再観測の失敗、リポジトリ ID の変化、存在を判定できない名前、解釈しない部分、知らない値、値が返らないことのために、値を期待値と比べられなかった。理由を添える。設定が効いていないことは意味しない |
| `not checked` | 最初の更新操作が拒否され、更新後の確認を行わなかった |

先行する更新操作が成功していれば、後続を開始できなかった場合も後続が失敗した場合も、更新後の確認の結果と変更対象外の設定の予期しない変化を示します。この確認で期待状態との一致を確認しても、必要な更新操作の成功を確認していないため、`updated` にしません。更新要求の応答を確認できない場合は成否不明とし、その後の再観測で期待状態が一致しても `updated` にしません。成否不明になった原因は、理由の行では区別せず、エラーとして示します（[CLI 仕様](cli.md) 5章）。

いずれかの論理的な設定変更で更新操作を開始しており、完了しなかった論理的な設定変更があるときは、変更結果の末尾に **変更結果の注記**（`Note:`）を添えます。開始した更新操作に成否不明があれば自動で再送していないことを、なければ自動ロールバックを行っていないことを示し、どちらにも `reco plan` で残りの変更を確かめられることを添えます。一つの変更の途中で止まった場合も、`plan` は先行する更新操作のあとの現在値を変更計画に示すため、利用者は残った中間状態と残りの変更を同じ表示で確かめられます。この判定は、選択したすべてのリポジトリの論理的な設定変更について行い、先に完了したリポジトリのあとで別のリポジトリが完了しなかった場合も添えます。更新操作を一度も開始していなければ、適用済みの変更が残らないため添えません。

異常時の表示では、確認済みの操作結果、成否が不明な範囲、未実行の変更を区別します。

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    actions.permissions.allowed_actions + actions.permissions.selected_actions: failed
      Update request (actions.permissions.allowed_actions): succeeded
      Update request (actions.permissions.selected_actions): rejected
      Desired state: does not match (actions.permissions.selected_actions)
    actions.workflow_permissions.default_workflow_permissions: not attempted
      Stopped: a preceding change did not complete
  example/project-b: not attempted
    Stopped: a preceding repository did not complete

Note: Some changes may already have been applied. No automatic rollback was performed. Run reco plan to see the remaining changes.
```

```text
Applying changes...
  example/project-a: updated
  example/project-b: not attempted
    Precondition: general.allow_auto_merge changed since approval
  example/project-c: not attempted
    Stopped: a preceding repository did not complete

Note: Some changes may already have been applied. No automatic rollback was performed. Run reco plan to see the remaining changes.
```

## 5. `show`：現在の状態

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |

### 5.1 設定宣言と同じ構造

`show` は、指定したリポジトリについて GitHub から確認できた管理設定の値とリポジトリリソースを、[YAML 設定仕様](configuration.md) の設定宣言と同じ構造の YAML で示します。ルートの `repositories` に要素を一つ置き、その下に、設定宣言のキーで、YAML 設定仕様の章と表の並びで値を示します。`name` には、GitHub が返す名前ではなく、`--repo` に指定したリポジトリ指定を示します。観測操作の失敗の文言と同じ名前で対象を示すためです。

設定宣言と同じ構造にするのは、表示から設定宣言のどこに何を書けばよいかを読み取れるようにし、表示をそのまま設定宣言の出発点にできるようにするためです。観測した値が設定宣言の入力検証を通る場合、表示を変えずに設定宣言として読むと、管理範囲は表示したすべての管理設定とリポジトリリソースになり、期待値は観測時点の値と一致します。観測した値が入力検証を通らない場合（宣言で受け付けない値や組み合わせを GitHub が返した場合）も、表示を設定宣言に合わせて書き換えません。書き換えると、観測した値を推測・補完して示すことになるためです。

表示するのは、設定宣言に書ける管理設定とリポジトリリソースに限ります。reco が観測しても宣言を受け付けない設定は表示しません。表示を設定宣言として読めなくなり、宣言できる設定とできない設定を利用者が見分けなければならなくなるためです。

次の例は、表示の一部を抜き出したものです。

```text
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
      allow_auto_merge: false
      # web_commit_signoff_required: unavailable (GitHub did not return the value)
    actions:
      permissions:
        enabled: true
        allowed_actions: all
        sha_pinning_required: false
      # fork_pr_contributor_approval:
      #   approval_policy: unavailable (the repository is private)
      artifact_and_log_retention:
        days: 90 # maximum allowed: 400
      # variables: none
    environments:
      - name: "production"
        deployment_branch_policy:
          mode: protected_branches
        reviewers:
          - type: User
            id: 583231 # octocat
        prevent_self_review: true
        wait_timer: 30 # minutes
        variables:
          - name: "REGION"
            value: "ap-northeast-1"
    security:
      dependabot_alerts: true
      dependabot_security_updates: true # paused by GitHub (still enabled)
    # rulesets: none
```

### 5.2 YAML のコメントで示すもの

値として読ませてはならない情報は、YAML のコメントで示します。値の位置に書くと、設定宣言として読んだときに期待値になるか、型が合わずに読めなくなるためです。

- **確認できない設定**：GitHub の取得条件のために確認できない設定は、その取得条件が `show` のほかの行から分かるかどうかで示し方を分ける。上位の設定によって効いていない状態（Actions が無効など）のように、ほかの行から分かる場合は、行ごと示さない。ほかの行から分からない場合は、行を省かず、コメントにして値の位置に `unavailable (<理由>)` と示す。行を省くと、確認できなかった設定と reco が表示しない設定を区別できないためである。GitHub が知らない値を返した設定と、値を返さなかった設定も、コメントにして `unavailable (<理由>)` と示し、ほかの行はいつもどおり示す
- **コメントにする範囲**：設定のキーだけをコメントにすると、値のないマッピングが残る場合（マッピングの唯一のキーなど）は、そのマッピングごとコメントにする。値のないマッピングは設定宣言として読めないためである（[YAML 設定仕様](configuration.md) 14.1節）。値の一部だけを示すと別の宣言として読める場合（方式だけで個別パターンを持たない値、解釈できた部分だけの Ruleset など）は、値の全体をコメントにする
- **リポジトリリソースがない**：リポジトリリソースが一つもないときは、種類のキーを省かず、コメントにして `none` と示す。キーを省くと、存在しないと確かめたことと、reco がその種類を表示しないことを区別できないためである。空のシーケンスは設定宣言として読めないため、コメントにする
- **値に添える補足**：設定宣言の値に書けないが、利用者が値を読むか設定宣言を書くのに要る情報（単位、GitHub が許す上限、ID に対応する login、GitHub による一時停止など）は、行末のコメントで示す

どの設定がどの理由で確認できず、どこまでをコメントにするかは、設定ごとの設計の「観測」が定めます。

### 5.3 リポジトリリソースと解釈しない部分

リポジトリリソースは、種類ごとに、存在するものをすべて、GitHub が一覧で返した順に示します。reco は並べ替えません。

リポジトリリソースの値が reco の解釈しない部分を含むときは、`name` だけを示し、続く行をコメントにして、`unavailable` と、解釈しなかったものを1行に一つずつ示します。解釈できた部分だけを値として示すことはしません。一部だけを設定宣言に書き写すと、書かなかった部分を外す宣言になるためです。解釈しなかったものの名前は GitHub が返した文字列で、二重引用符で囲んでエスケープして示し、どの部分か（ルール型、パラメータ、キーなど）を添えます。どの部分を解釈するかは、設定ごとの設計の「観測」が定めます。

```text
    rulesets:
      - name: "strict-review"
        # unavailable (the ruleset contains parts that reco does not interpret)
        #   pull_request parameter: "required_reviewers"
        #   rule type: "creation"
```

権限の不足により GitHub が値を応答に含めない場合は、取得条件による観測不能ではなく観測操作の失敗です（[CLI 仕様](cli.md) 3章）。行を省かず、欠けた項目を示してエラーとします。

## 6. References

- [CLI 仕様](cli.md)
- [YAML 設定仕様](configuration.md)
- [要求定義](requirements.md)
- [ドメインモデル](../design/domain.md)
- [アーキテクチャ設計](../design/architecture.md)
- [設定ごとの設計](../design/settings/README.md)
- [GitHub Actions 変数 API 仕様](../research/actions-variables.md)
