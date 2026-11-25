# Security の設計

設定宣言の `security`（[YAML 設定仕様](../../spec/configuration.md) 11章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Dependabot | Dependabot | Dependabot alerts と Dependabot security updates からなる設定の領域。一方の変更予定の適用可能性と副作用がもう一方の値で決まるため、一回の観測で両方を読み取る |
| Dependabot alerts | Dependabot Alerts | 依存関係の脆弱性を知らせる Dependabot alerts を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない |
| Dependabot security updates | Dependabot Security Updates | 脆弱性を修正する Pull Request を Dependabot が作る Dependabot security updates を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない。有効のときは一時停止を伴うことがある |
| 一時停止 | Paused | 有効な Dependabot security updates について、GitHub が更新を止めている状態。GitHub が自動で入れ、利用者が選ぶ値ではない。期待値にならない |
| Secret scanning | Secret Scanning | Secret scanning の有効状態とリポジトリの push protection からなる設定の領域。どちらもリポジトリの取得の応答に含まれ、この領域のための読み取り要求はない |
| Secret scanning の有効状態 | Secret Scanning Enabled | リポジトリに push されたシークレットを検出する Secret scanning を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかである |
| リポジトリの push protection | Repository Push Protection | シークレットを含む push を拒否する push protection を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかである。利用者が自分のアカウントに設定する push protection とは別の設定である |
| Private vulnerability reporting | Private Vulnerability Reporting | Private vulnerability reporting の有効状態だけからなる設定の領域。Dependabot や Secret scanning の領域に含めないのは、この設定を宣言していないリポジトリの変更計画の確認で、この領域の読み取り要求を送らないためである |
| Private vulnerability reporting の有効状態 | Private Vulnerability Reporting Enabled | 見つけた脆弱性を、公開せずにメンテナーへ報告できる Private vulnerability reporting を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかで、前提の設定を持たない |
| Dependency graph | Dependency Graph | リポジトリの依存関係を GitHub が解析する機能の有効状態。Dependabot alerts を変える更新に伴って GitHub が変える。GitHub の REST API に、その値を返す操作も、それだけを変える操作もないため（[GitHub リポジトリ API 仕様](../../research/repositories.md) 6章）、reco は観測も表示も管理もしない |

依存する設定と前提の設定（[ドメインモデル](../domain.md) の GitHub が提供する対象と設定）の組は、Dependabot security updates と Dependabot alerts、リポジトリの push protection と Secret scanning の有効状態の二つです。

Dependabot security updates の一時停止中の値は、有効と同じ値と判定し、表現は一時停止中かどうかを保ちます（[ドメインモデル](../domain.md) の管理の単位と管理設定）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

### Dependabot

Dependabot は、`GET /repos/{owner}/{repo}/vulnerability-alerts` で Dependabot alerts を、`GET /repos/{owner}/{repo}/automated-security-fixes` で Dependabot security updates を、この順に読みます。Dependabot の領域を読む観測は必ず両方を送ります。

Dependabot alerts の取得は、値を本文ではなく、要求が成り立ったかどうかで返します。有効なら `204` を、無効なら `404` を返します（[GitHub リポジトリ API 仕様](../../research/repositories.md) 6章）。`404` を無効と読むのは、同じ観測の `GET /repos/{owner}/{repo}` の応答の `permissions.admin` が `true` のときに限ります（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。公式資料がこの取得に admin read のアクセスを求めており、admin のアクセスを持てば取得そのものは成り立つはずで、残る解釈が無効だけになるためです。本文の `message` は無効のときだけ違いましたが、公式資料の契約ではないため読みません。`permissions.admin` が `false` のときと、`permissions` か `admin` が欠けたときの `404`、`204` と `404` のほかの応答は、観測操作の失敗です。

この読み方には、確かめていない場合が残ります。`permissions.admin` が `true` でも、トークン自体の権限が足りない場合（Administration の read を持たない fine-grained personal access token、`repo` の scope を持たない classic のトークン）です。GitHub がこのとき `404` を返すなら、reco は有効な Dependabot alerts を無効と読みます。`dependabot_alerts: false` の宣言は一致と照合され、`dependabot_alerts: true` の宣言は有効にする変更を計画しますが、その更新要求は値を下げず、更新後の観測も無効と読むため完了とは報告しません。確かめたのは、`repo` の scope を持つ `gh auth token` のトークンで、所有者が Admin ロールを持つ場合だけです（GitHub リポジトリ API 仕様 5章）。この読み方は、Dependabot alerts のあとに必ず Dependabot security updates を読む順序にも支えられています。後者も admin read のアクセスを求め、その `404` は観測操作の失敗のため、権限の足りないトークンや、二つの取得のあいだにリポジトリが消えるか admin のアクセスを失った場合は、観測の全体が失敗すると見込めます（権限の足りないトークンでの応答は確かめていません）。

Dependabot security updates の取得は、`200` の本文の `enabled` と `paused` を読みます。無効のときも `200` で `enabled: false` が返り、公式資料の「無効なら `404`」とは食い違うため（GitHub リポジトリ API 仕様 7章）、`404` は無効と読まず観測操作の失敗とします。取得契約は、`enabled` と `paused` がそろい、無効（`false`、`false`）、有効（`true`、`false`）、一時停止中（`true`、`true`）のどれかであることです。`enabled: false` と `paused: true` の組は、実際の動作で観測しておらず意味を根拠づけられないため、取得契約に適合しない応答とします。`security_and_analysis.dependabot_security_updates` は、一時停止を含まず、非公開リポジトリでは返らないため、取得元にしません。

### Secret scanning

Secret scanning の2設定は、`GET /repos/{owner}/{repo}` の応答の `security_and_analysis.secret_scanning.status` と `security_and_analysis.secret_scanning_push_protection.status` から、`enabled` を有効、`disabled` を無効と読みます。

応答の `private` が真のときは、`security_and_analysis` を読まずに、二つの設定をリポジトリが非公開であることを理由とする観測不能とします。応答が値を含んでいても読みません。非公開リポジトリへの push protection の更新は `200` のまま値を変えず（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5章）、そこで返る値は比べられる値として扱えないためです。

`private` が偽のときは、二つの `status` がそろい、どちらも `enabled` か `disabled` であることを取得契約とします。`security_and_analysis` が欠けるか `null` の応答、`status` が欠けるか定義にない値の応答は、取得契約に適合しない応答です。公開リポジトリで `security_and_analysis` が欠けるのは権限の不足であり（第三者の公開リポジトリで確かめた。同 5章）、取得条件として根拠づけられないためです。この判定は、ほかの設定の領域の読み取り要求を送る前に行い、失敗が決まった観測のために要求を送りません。Secret scanning の領域を読まない観測は `security_and_analysis` を解釈せず、その形で失敗しません。General だけを宣言したリポジトリの `plan` を、宣言していない設定の応答の形で失敗させないためです。

### Private vulnerability reporting

Private vulnerability reporting の有効状態は、`GET /repos/{owner}/{repo}/private-vulnerability-reporting` の本文の `enabled`（boolean）で読みます。この要求は、応答の `private` が偽のときだけ送り、真のときはリポジトリが非公開であることを理由とする観測不能とします。GitHub は非公開リポジトリへのこの取得に、存在しないリポジトリへの応答と区別できない `404` を返すため（[GitHub リポジトリ API 仕様](../../research/repositories.md) 8章）、送れば観測操作の失敗になり、同じ観測のほかの設定も確認できなくなるためです。公開と観測したあとの取得の失敗は、`404` と `422` を含めて観測操作の失敗です。この取得は admin のアクセスを求めず、権限のない第三者の公開リポジトリでも値を返しました（同 8章）。

### 取得条件による観測不能

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| リポジトリが非公開（`private: true`） | Secret scanning の有効状態、リポジトリの push protection、Private vulnerability reporting の有効状態 | Secret scanning は、公式資料が、ユーザーが所有する非公開リポジトリでの利用を Enterprise Managed Users と GitHub Enterprise Server に限り、非公開リポジトリの応答は `admin` が `true` でも `security_and_analysis` を含まない（GitHub リポジトリ API 仕様 5章）。Private vulnerability reporting は、公式資料が公開リポジトリの設定として記載する（同 8章） |

Dependabot の2設定に、取得条件による観測不能はありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

Dependabot と Secret scanning には、一つの設定の更新が別の設定を変える依存があります。次の表は、その依存の扱いを定めます。

| 宣言した変更 | GitHub の動作（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5〜7章） | 扱い |
|---|---|---|
| Dependabot alerts を無効にする | `DELETE` が Dependency graph も無効にする（公式資料に記載がある）。有効だった Dependabot security updates も無効にする（公式資料に記載がなく、実際の動作で確かめた） | 適用可能。Dependency graph が無効になることと、Dependabot security updates を宣言していなければその計画時の観測値からの変化を、変更に伴う副作用として示す |
| Dependabot security updates を変える（どちらの向きも） | Dependabot alerts が無効なら `PUT` も `DELETE` も `422` で拒否する | Dependabot alerts が無効で、有効にする宣言がなければ適用不能 |
| push protection を有効にする | Secret scanning が無効なら `200` を返して値を変えない | Secret scanning が無効で、有効にする宣言がなければ適用不能 |
| Secret scanning を無効にする | 有効だった push protection も無効にする | 適用可能。push protection を宣言していなければ、その計画時の観測値からの変化を変更に伴う副作用として示す |

前提の設定を無効にする変更を適用可能とし、前提の設定が無効なときの依存する設定の変更を適用不能とする理由は、[アーキテクチャ設計](../architecture.md) 5.4節の方針の適用です。前者で依存する設定と Dependency graph が変わるのは、利用者が宣言した無効化を GitHub が処理した結果で、reco が避ける更新方式はありません。Dependabot alerts を無効のまま Dependency graph を残す手段は設定画面の操作だけで、REST API にはありません（GitHub リポジトリ API 仕様 6章）。後者は、GitHub が前提の設定を自動では有効にしないため、reco が宣言されていない前提の設定を変えなければ実行できません。

### Dependabot

| Dependabot alerts の計画時の観測 | `dependabot_alerts` の宣言 | Dependabot security updates の変更予定の適用可能性 |
|---|---|---|
| 有効 | 問わない | 適用可能 |
| 無効 | `true` | 適用可能。Dependabot alerts を有効にする変更のあとに更新する |
| 無効 | 省略か `false` | 適用不能。理由は Dependabot alerts が無効であること |

Dependabot alerts の変更予定は、どちらの向きも適用可能です。Dependabot alerts の変更予定に添える副作用と、副作用を生む更新操作の変更前提に含める設定は、次のとおりです。

| Dependabot alerts の変更予定 | `dependabot_security_updates` の宣言 | Dependabot security updates の計画時の観測 | 変更に伴う副作用 | 変更前提に含める Dependabot security updates |
|---|---|---|---|---|
| 有効にする | 問わない | 問わない | Dependency graph が有効になる | 含めない |
| 無効にする | 省略 | 有効か一時停止中 | Dependency graph が無効になる。Dependabot security updates が計画時の観測値から無効になる | 計画時の観測値 |
| 無効にする | 省略 | 無効 | Dependency graph が無効になる | 計画時の観測値（副作用を示さなかった設定） |
| 無効にする | 宣言する | 問わない | Dependency graph が無効になる | 含めない。先行する Dependabot security updates の変更が無効にする |

Dependency graph の副作用は、reco が観測しない設定への副作用のため、計画時の値を示さず、変更前提でも更新後にも確かめません。Dependabot alerts の `DELETE` が無効にし、`PUT` が有効にすることは公式資料の記載で確定しているため（GitHub リポジトリ API 仕様 6章）、可能性ではなく起きる変化として添えます。すでに同じ状態なら実際には何も変わりませんが、reco は値を観測しないため、添えるかどうかを変化の有無で分けられません。

Dependabot security updates の変更の変更前提に、Dependabot alerts が有効であることは含めません。承認のあとに Dependabot alerts が無効にされていれば、GitHub が `422` で拒否して値を変えないためです。

### Secret scanning

非公開リポジトリのために比較不能になった Secret scanning の2設定は、常に適用不能で、理由はリポジトリが非公開であることです。非公開リポジトリへの更新は `200` のまま値を変えず（GitHub リポジトリ API 仕様 5章）、更新のあとに期待状態を確かめる手段がなく、公開範囲は管理設定ではないため、ほかの宣言では解消しません。公開リポジトリでは、Secret scanning の変更予定はどちらの向きも適用可能です。

| Secret scanning の計画時の観測 | `secret_scanning` の宣言 | push protection を有効にする変更予定 | push protection を無効にする変更予定 |
|---|---|---|---|
| 有効 | 問わない | 適用可能 | 適用可能 |
| 無効 | `true` | 適用可能。Secret scanning を有効にする変更のあとに更新する | 適用可能 |
| 無効 | 省略 | 適用不能。理由は Secret scanning が無効であること | 適用可能 |

push protection を有効にする向きを適用不能にする根拠は、Dependabot security updates と異なり、GitHub が拒否せず `200` を返して値を変えないことです。送っても期待状態に至る手段がありません。無効にする向きは、Secret scanning が無効のときに反映されない動作を確かめておらず、確かめていない条件で止めないため適用可能とし、反映されなければ更新後の観測が不一致として検出します。

| Secret scanning の変更予定 | `secret_scanning_push_protection` の宣言 | push protection の計画時の観測 | 変更に伴う副作用 | 変更前提に含める push protection |
|---|---|---|---|---|
| 有効にする | 問わない | 問わない | なし | 含めない |
| 無効にする | 省略 | 有効 | push protection が計画時の観測値から無効になる | 計画時の観測値 |
| 無効にする | 省略 | 無効 | なし | 計画時の観測値（副作用を示さなかった設定） |
| 無効にする | 宣言する | 問わない | なし | 含めない。先行する push protection の変更が無効にする |

push protection を有効にする変更の変更前提には、Secret scanning が有効であることを含め、承認時に予定した値は有効です。GitHub が `200` を返して値を変えず、更新操作の結果からは反映されなかったことが分からないためです（[ドメインモデル](../domain.md) の承認と変更前提）。計画時に無効だった場合は、先行する Secret scanning の変更が有効にすることが予定どおりの変化です。push protection を無効にする変更の変更前提には含めません。

### Private vulnerability reporting

非公開リポジトリのために比較不能になった Private vulnerability reporting の有効状態は、Secret scanning と同じく常に適用不能です。GitHub は非公開リポジトリへの更新に `404` を返し、更新のあとに期待状態を確かめる手段もないためです（GitHub リポジトリ API 仕様 8章）。公開リポジトリでは、どちらの向きも、ほかの宣言と観測によらず適用可能で、変更に伴う副作用はありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 論理的な設定変更と更新操作

Security の5設定の変更予定は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。依存する設定と前提の設定をまとめないのは、片方の期待状態だけが一致しなかったときに、どちらの変更が完了したかを分けて報告できるようにするためです。

実行順は YAML 設定仕様の表の並び（前提の設定、依存する設定）を基本とし、前提の設定を無効にする変更予定があるときだけ、同じ対象リポジトリの依存する設定の変更予定をその直前に実行します。前提の設定を先に無効にすると、GitHub が依存する設定を先に無効にし、その変更予定の変更前提が成り立たなくなるためです。変更計画の表示は表の並びのままです。前提の設定を無効にし、依存する設定を有効にする宣言は入力検証で拒否するため、二つの向きが逆の変更予定は並びません。

| 前提の設定の変更予定 | 依存する設定の変更予定 | 実行順 |
|---|---|---|
| 有効にする | あり | 前提の設定、依存する設定 |
| 無効にする | あり（無効にする） | 依存する設定、前提の設定 |
| なし | あり | 依存する設定 |
| あり | なし | 前提の設定 |

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 更新方式

Secret scanning の2設定は、General と同じ `PATCH /repos/{owner}/{repo}` に、`security_and_analysis` の中の変える1項目の `status`（`enabled` か `disabled`）だけを送ります。正常応答は `200` で、本文が取得契約を満たせば成功です。`200` の本文が `security_and_analysis` を含まない場合があるため、本文でこの項目の値を確かめません（[GitHub API 通信の設計](../github-access.md) 3.2節）。1項目だけを送る `PATCH` が、もう一方の項目、`security_and_analysis` のほかの項目、トップレベルの一般設定を変えなかったことを公開リポジトリで確かめており（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5章）、これを部分更新の根拠とします。更新後に更新直前の値と比べるのは、General の6設定と、Secret scanning の2設定のうちもう一方（副作用として示した場合を除く）です。

Dependabot alerts（`/repos/{owner}/{repo}/vulnerability-alerts`）、Dependabot security updates（`/repos/{owner}/{repo}/automated-security-fixes`）、Private vulnerability reporting（`/repos/{owner}/{repo}/private-vulnerability-reporting`）は、期待値が `true` なら本文のない `PUT`、`false` なら本文のない `DELETE` を送り、本文のない `204` を正常応答とします。すでに同じ状態でも `204` が返り、直後の取得が更新後の値を返すことを確かめています（GitHub リポジトリ API 仕様 6〜8章）。それぞれの更新エンドポイントで reco が観測する設定はその設定だけのため、更新後に比べる変更対象外の設定はありません。保全の根拠は次の確認結果です。

- Dependabot alerts の `PUT` と、Dependabot security updates の `PUT` と `DELETE` は、もう一方の設定を変えなかった（同 6・7章）。Dependabot alerts の `DELETE` の影響は、変更に伴う副作用として扱う
- Private vulnerability reporting の `PUT` と `DELETE` は、公開リポジトリで、reco が観測するほかの設定を変えなかった（同 8章）

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## シナリオと反例

以下は Security の設定に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| 非公開リポジトリで Secret scanning の領域を読み取り、応答が二つの設定の値を含んでいた | 観測不能。観測不能かどうかは公開範囲で決め、応答の値を観測値にしない | 応答にあるので観測値とする |
| 公開リポジトリで Secret scanning の領域を読み取り、応答に `security_and_analysis` がなかった | 観測操作の失敗。公開リポジトリで値が返らない理由は権限の不足である | 取得条件による観測不能として表示へ進む。または、無効とみなす |
| General だけを読み取る観測で、応答に `security_and_analysis` がなかった | 観測は成り立つ。Secret scanning の領域は読み取らなかった設定の領域である | 必要なフィールドが欠けたので観測操作の失敗 |
| Dependabot security updates を `true` と宣言し、一時停止中だった | 一時停止中の値は有効と同じ値で、Match。変更予定にならない | 一時停止を解くまで差分が消えないので Mismatch |
| Dependabot alerts が無効で宣言されていないリポジトリで、Dependabot security updates を `true` と宣言した | Mismatch で適用不能。理由は Dependabot alerts が無効であること | 有効にするには前提が要るので、Dependabot alerts を有効にしてから更新する |
| Dependabot alerts と Dependabot security updates の両方を無効にすると宣言し、どちらも有効だった | Dependabot security updates、Dependabot alerts の順に実行する。Dependabot alerts の変更に Dependabot security updates の副作用は添えない | 表の並びのまま Dependabot alerts を先に無効にする |
| Dependabot alerts を無効にする変更を承認し、副作用として示した Dependabot security updates が、更新直前に無効になっていた | 関係する現在値の変化で、変更前提は不成立。変更結果は未実行 | 無効にする予定の設定がすでに無効なので、変更前提は成立する |
| Secret scanning が無効で宣言されていないリポジトリで、リポジトリの push protection を `true` と宣言した | Mismatch で適用不能。理由は Secret scanning が無効であること | GitHub が成功を応答するので適用可能 |
| push protection を宣言せずに Secret scanning を無効にすると宣言し、push protection が計画時に無効で、更新後に有効だった | 変更対象外の設定の予期しない変化。副作用を示さなかった設定は更新後にも比べる | 副作用に関わる設定なので比べない |
| General の設定だけを変える更新の前後で、Secret scanning の有効状態が変わった | 変更対象外の設定の予期しない変化で、変更結果は失敗 | Secret scanning を宣言していないので比べない |
| リポジトリの push protection を有効にする変更を承認し、更新直前に Secret scanning が無効になっていた | 関係する現在値の変化で、変更前提は不成立。変更結果は未実行 | 変えるのは push protection だけなので、変更前提は成立する |
| 非公開リポジトリで Private vulnerability reporting の領域を読み取った | 観測不能。理由はリポジトリが非公開であること。読み取り要求は送らない | 取得を送り、「対象が見つからない」を無効と読む |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [アーキテクチャ設計](../architecture.md)
- [GitHub API 通信の設計](../github-access.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
