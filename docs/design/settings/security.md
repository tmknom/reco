# Security の設計

設定宣言の `security`（[YAML 設定仕様](../../spec/configuration.md) 9章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Dependabot | Dependabot | Dependabot alerts と Dependabot security updates からなる設定の領域。一方の設定の変更がもう一方の値に左右されるため（[適用可能性と副作用](#適用可能性と副作用)）、一回の観測で両方を読み取る |
| Dependabot alerts | Dependabot Alerts | 依存関係の脆弱性を知らせる Dependabot alerts を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない |
| Dependabot security updates | Dependabot Security Updates | 脆弱性を修正する Pull Request を Dependabot が作る Dependabot security updates を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない。有効のときは一時停止を伴うことがある。期待値は有効か無効で、一時停止は期待値にならない |
| 一時停止 | Paused | 有効な Dependabot security updates について、GitHub が更新を止めている状態。GitHub が自動で入れ、利用者が選ぶ値ではない。設定の値としては有効と同じ値で、値の表現が一時停止中かどうかを保つ |
| Secret scanning | Secret Scanning | Secret scanning の有効状態とリポジトリの push protection からなる設定の領域。どちらもリポジトリの取得の応答に含まれ、この領域のための読み取り要求はない。一方の設定の変更がもう一方の値に左右される（[適用可能性と副作用](#適用可能性と副作用)） |
| Secret scanning の有効状態 | Secret Scanning Enabled | リポジトリに push されたシークレットを検出する Secret scanning を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかである。非公開リポジトリでは観測不能で、理由はリポジトリが非公開であることである。無効にする更新に伴って、GitHub がリポジトリの push protection も無効にする |
| リポジトリの push protection | Repository Push Protection | シークレットを含む push を拒否する push protection を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかである。利用者が自分のアカウントに設定する push protection とは別の設定である。非公開リポジトリでは観測不能で、理由は Secret scanning の有効状態と同じである。Secret scanning の有効状態が無効のあいだ、GitHub は有効にする更新を反映しない |
| Private vulnerability reporting | Private Vulnerability Reporting | Private vulnerability reporting の有効状態だけからなる設定の領域。この領域のための読み取り要求を一つ持つ。Dependabot の領域にも Secret scanning の領域にも含めないのは、この設定を宣言していないリポジトリの変更計画の確認で、この読み取り要求を送らないためである |
| Private vulnerability reporting の有効状態 | Private Vulnerability Reporting Enabled | 見つけた脆弱性を、公開せずにメンテナーへ報告できる Private vulnerability reporting を、リポジトリで有効にしているかを表す設定。有効か無効のいずれかである。非公開リポジトリでは観測不能で、理由は Secret scanning の有効状態と同じである。ほかの設定を前提にせず、期待値はほかの設定の状態を含まない |
| Dependency graph | Dependency Graph | リポジトリの依存関係を GitHub が解析する機能の有効状態。Dependabot alerts の有効状態を変える更新に伴って、GitHub が変える。reco は観測せず、設定の領域に含めず、値の表示も管理もしない。GitHub の REST API に、その値を返す操作も、それだけを変える操作もないためである（[適用可能性と副作用](#適用可能性と副作用)） |

依存する設定と前提の設定（[ドメインモデル](../domain.md) の GitHub が提供する対象と設定）の組は、Dependabot security updates と Dependabot alerts、リポジトリの push protection と Secret scanning の有効状態の二つです。

Dependabot security updates の一時停止中の値と有効の値は、同じ値と判定されますが、表現は一時停止中かどうかを保ちます。表示は保った表現を使い、照合、変更前提、期待状態の確認、変更対象外の設定の比較は、一時停止の有無で結果を変えません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

### Dependabot

Dependabot は、二つの読み取り要求で読みます。`GET /repos/{owner}/{repo}/vulnerability-alerts` で Dependabot alerts を、`GET /repos/{owner}/{repo}/automated-security-fixes` で Dependabot security updates を、この順に読みます。どちらも `GET /repos/{owner}/{repo}` が成功したときだけ送り、Dependabot の領域を読む観測は必ず両方を送ります。一方の設定の変更予定の副作用と適用可能性が、もう一方の観測値で決まるためです（[適用可能性と副作用](#適用可能性と副作用)）。Dependabot alerts の取得が失敗すれば、Dependabot security updates の要求は送りません。

Dependabot alerts の取得は、値を本文ではなく、要求が成り立ったかどうかで返します（[GitHub リポジトリ API 仕様](../../research/repositories.md) 6章）。有効なら成功の `204` を、無効なら「対象が見つからない」の `404` を返します。`404` は、権限の不足やリポジトリの不存在でも返る GitHub への要求の失敗であり、この応答だけでは無効と判断できません。`404` を無効と読むのは、同じ観測の `GET /repos/{owner}/{repo}` の応答の `permissions.admin` が `true` のとき、つまりトークンの所有者がリポジトリへの admin のアクセスを持つと確かめたときに限ります。公式資料がこの取得に admin read のアクセスを求めており、admin のアクセスを持てば取得そのものは成り立つはずで、残る解釈が無効だけになるためです。本文の `message` は無効のときだけ違いましたが、公式資料の契約ではないため読みません。`permissions.admin` が `false` のときと、`permissions` か `admin` が欠けたときの `404` は、観測操作の失敗（対象が見つからない）のままで、観測不能にもしません。値を確認できない理由を、GitHub の取得条件として根拠づけられないためです。`permissions` の取得契約は [General の設計](general.md) の観測が定めます。`204` と `404` のほかの応答は、`204` 以外の `2xx` を含めて観測操作の失敗です。`404` を無効と読む扱いは、この取得だけのものです。ほかの読み取り要求の `404` は、admin のアクセスを確かめていても観測操作の失敗のままです。

この条件には、確かめていない場合があります。`permissions.admin` が `true` でも、トークン自体の権限がこの取得に足りない場合です。Administration の read を持たない fine-grained personal access token や、`repo` の scope を持たない classic のトークンがこれに当たり得ます。GitHub が権限の不足として `404` を返すなら、reco は有効な Dependabot alerts を無効と読みます。`plan` では `dependabot_alerts: false` の宣言を一致と照合して変更予定にせず、`dependabot_alerts: true` の宣言を不一致として有効にする変更を計画します。後者の更新要求は、有効な Dependabot alerts を有効にするだけで値を下げず、更新後の観測も無効と読むため、完了とは報告しません。この場合の `permissions.admin` の値と取得の応答は、どちらも確かめていません。確かめたのは、`repo` の scope を持つ `gh auth token` のトークンで、トークンの所有者が Admin ロールを持つ場合だけです（GitHub リポジトリ API 仕様 5章）。reco が必要な権限の確認と案内の前提にするのも、このトークンです（[CLI 仕様](../../spec/cli.md) 3章）。

Dependabot security updates の取得も admin read のアクセスを求め、その `404` は観測操作の失敗です。Dependabot alerts のあとに必ずこの取得を送るため、権限の足りないトークンや、二つの取得のあいだにリポジトリが消えるか admin のアクセスを失った場合は、観測の全体が失敗すると見込めます（権限の足りないトークンでの応答は確かめていません）。Dependabot alerts の `404` を無効と読む読み方は、この順序にも支えられています。

Dependabot security updates の取得は、`200` の本文の `enabled` と `paused` を読みます。無効のときも `200` で `enabled: false` が返り、公式資料の「無効なら `404`」とは食い違います（GitHub リポジトリ API 仕様 7章）。そのため `404` は無効と読まず、観測操作の失敗とします。観測値は、無効（`enabled: false`、`paused: false`）、有効（`enabled: true`、`paused: false`）、有効で一時停止中（`enabled: true`、`paused: true`）の三つです。取得契約は、`enabled` と `paused` がそろい、この三つの組のどれかであることです。`enabled: false` と `paused: true` の組は、実際の動作で観測しておらず意味を根拠づけられないため、取得契約に適合しない応答とします。`security_and_analysis.dependabot_security_updates` は値が一致しましたが、一時停止を含まず、非公開リポジトリでは `GET /repos/{owner}/{repo}` が `security_and_analysis` を返さないため、取得元にしません。Dependabot の二つの設定に、取得条件による観測不能はありません。Dependency graph は読みません（[適用可能性と副作用](#適用可能性と副作用)）。

### Secret scanning

Secret scanning の有効状態とリポジトリの push protection は、`GET /repos/{owner}/{repo}` の応答の `security_and_analysis.secret_scanning.status` と `security_and_analysis.secret_scanning_push_protection.status` から読みます。追加の読み取り要求は送りません。`enabled` を有効、`disabled` を無効と読みます。`security_and_analysis` のほかの項目は読みません。

二つの設定は、応答の `private` が真のとき、`security_and_analysis` を読まずに観測不能とします。理由はリポジトリが非公開であることで、二つの設定は同じ理由を持ちます。非公開リポジトリでは、`permissions.admin` が `true` でも応答から `security_and_analysis` が欠け、公式資料も、ユーザーが所有する非公開リポジトリの Secret scanning を Enterprise Managed Users と GitHub Enterprise Server に限っています（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5章）。reco が扱う個人アカウント所有のリポジトリでは、非公開のリポジトリで Secret scanning を利用できないためです。応答が `security_and_analysis` を含んでいても読まないのは、観測不能の根拠が公開範囲であり、フィールドの有無で分けると、非公開リポジトリへの push protection の更新が `200` のまま値を変えないにもかかわらず（同 5章）、その値を比べられる値として扱うことになるためです。

`private` が偽のときは、二つの `status` がそろい、どちらも `enabled` か `disabled` であることを取得契約とします。`security_and_analysis` が欠けた応答、`null` の応答、どちらかの `status` が欠けた応答、定義にない `status` の応答は、取得契約に適合しない応答です。公開リポジトリで `security_and_analysis` が欠けるのは権限の不足であり（第三者の公開リポジトリで確かめた。同 5章）、GitHub の取得条件として根拠づけられないため、General のマージ関連フィールドの欠落と同じく、取得条件による観測不能にしません。片方の設定だけの観測結果も作りません。この判定は、ほかの設定の領域の読み取り要求を送る前に行います。失敗が決まった観測のために、要求を送らないためです。

`security_and_analysis` を取得契約に含めるのは、Secret scanning の領域を読む観測だけです。この領域を読まない観測は `security_and_analysis` を解釈せず、欠けていても、`null` でも、型が合わなくても失敗しません。General だけを宣言したリポジトリの `plan` を、宣言していない設定の応答の形で失敗させないためです。

General の設定を変える更新操作と、Secret scanning の2設定を変える更新操作は同じ更新エンドポイントへ送るため、どちらの更新直前と更新後の観測も、General と Secret scanning の両方の領域を読み取ります。同じ更新エンドポイントの設定として比べるためです（[更新方式](#更新方式)）。そのため、公開リポジトリの General の更新は、応答が `security_and_analysis` を含まなければ、更新要求を送らずに止まります。

### Private vulnerability reporting

Private vulnerability reporting の有効状態は、`GET /repos/{owner}/{repo}/private-vulnerability-reporting` の本文の `enabled` で読みます。この要求は、`GET /repos/{owner}/{repo}` が成功し、応答の `private` が偽のときだけ送ります。`private` が真のときは送らず、リポジトリが非公開であることを理由とする観測不能とします。理由は Secret scanning の2設定と同じです。要求を送らないことの意味は、公開リポジトリの `access_level` と同じです（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。GitHub は非公開リポジトリへのこの取得に `404` を返し、その応答は存在しないリポジトリへの応答と区別できないためです（[GitHub リポジトリ API 仕様](../../research/repositories.md) 8章）。応答から観測不能を根拠づけられず、送れば観測操作の失敗になって、同じ観測のほかの設定も確認できなくなります。公開と観測したあとの取得の失敗は、`404` と `422` を含めて観測操作の失敗とし、観測不能にも無効にも読み替えません。取得契約は、`enabled` が boolean であることです。この取得は admin のアクセスを求めず、権限のない第三者の公開リポジトリでも値を返しました（同 8章）。

### 取得条件による観測不能

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| リポジトリが非公開（`private: true`） | Secret scanning の有効状態、リポジトリの push protection、Private vulnerability reporting の有効状態 | Secret scanning の2設定は、公式資料が、ユーザーが所有する非公開リポジトリの Secret scanning を Enterprise Managed Users と GitHub Enterprise Server に限り、非公開リポジトリの応答は `security_and_analysis` を含まない（GitHub リポジトリ API 仕様 5章）。Private vulnerability reporting は、公式資料が公開リポジトリの設定として記載し、非公開リポジトリへの取得は存在しないリポジトリと区別できない `404` を返す（同 8章） |

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

Dependabot alerts、Dependabot security updates、Secret scanning の有効状態、リポジトリの push protection には、一つの設定の更新が別の設定を変える依存があります。次の表は、その依存の扱いを定めます。

| 宣言した変更 | GitHub の動作（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5〜7章） | 扱い |
|---|---|---|
| Dependabot alerts を無効にする | `DELETE` が Dependency graph も無効にする（公式資料に記載がある）。有効だった Dependabot security updates も無効にする（公式資料に記載がなく、実際の動作で確かめた） | 適用可能。Dependency graph が無効になることと、Dependabot security updates を宣言していなければその計画時の観測値からの変化を、変更に伴う副作用として示す |
| Dependabot security updates を有効にする | Dependabot alerts が無効なら `422` で拒否する | Dependabot alerts が無効で、有効にする宣言がなければ適用不能 |
| push protection を有効にする | Secret scanning が無効なら `200` を返して値を変えない | Secret scanning が無効で、有効にする宣言がなければ適用不能 |
| Secret scanning を無効にする | 有効だった push protection も無効にする | 適用可能。push protection を宣言していなければ、その計画時の観測値からの変化を変更に伴う副作用として示す |

Dependabot alerts の無効化を適用可能にしたのは、実際の動作を確かめたうえで、人間が判断したためです。調査の段階では、Dependency graph への影響を伴う更新を、未管理設定を変えない保証と両立させる方法が決まるまで適用しない案でした。判断の理由は次のとおりです。

- Dependency graph と Dependabot security updates の変化は、利用者が宣言した無効化に対して GitHub が起こす影響で、reco が宣言していない設定を自分で選んで変えるものではない。Actions 許可ポリシーを `selected` から変えて選択方式の Actions 許可条件が失われる遷移と同じ性質であり、同じく変更に伴う副作用として承認の前に示せば、INV-001 が禁じる暗黙の変更に当たらない
- Dependabot alerts を無効のまま Dependency graph を残す手段は、設定画面の操作だけで、REST API にはない（GitHub リポジトリ API 仕様 6章）。reco が影響を避ける更新方式はない。無効化を適用不能にすれば、利用者は reco で Dependabot alerts を無効にできなくなるだけで、影響を承認の前に知る手段は増えない
- 逆に、Dependabot security updates の有効化と push protection の有効化で前提の設定が無効なときに適用不能とするのは、GitHub が前提の設定を自動では有効にしないため、reco が宣言されていない設定を変えなければ実行できないからである

### Dependabot

Dependency graph は、reco が観測も管理もしない設定です。REST API に、Dependency graph の有効状態を返す操作も、Dependency graph だけを有効・無効にする操作もないためです（GitHub リポジトリ API 仕様 6章）。実際の動作の確認では SBOM の取得の成否で代わりに確かめましたが、SBOM の取得は有効状態を返す操作ではありません。そのため Dependency graph の副作用は観測値を伴わず、無効になることだけを示し、更新後にも確かめません（[ドメインモデル](../domain.md) の変更に伴う副作用）。Dependabot alerts を無効にすると GitHub が Dependency graph を無効にすることは、公式資料の記載で確定しており、可能性ではなく起きる変化として添えます。Dependabot alerts を有効にすると GitHub が Dependency graph を有効にすることも、同じく起きる変化として添えます。Dependency graph がすでに有効なら実際には何も変わりませんが、reco はその値を観測しないため、変化の有無で添えるかどうかを分けられません。Dependabot security updates と push protection の副作用は、計画時の観測値とともに示し、副作用を生む更新操作の変更前提に含めます。

Dependabot alerts の変更予定は、どちらの向きも適用可能です。Dependabot security updates の Mismatch は、Dependabot alerts の計画時の観測値が有効か、同じ対象リポジトリで Dependabot alerts を有効と宣言したときに限り適用可能です。どちらでもなければ適用不能で、理由は Dependabot alerts が無効であることです。GitHub は、Dependabot alerts が無効のあいだ、Dependabot security updates の `PUT` も `DELETE` も `422` で拒否し（GitHub リポジトリ API 仕様 7章）、reco が Dependabot alerts を有効にすれば、利用者が宣言していない設定を変えることになるためです。有効にする向きも無効にする向きも同じです。

| Dependabot alerts の計画時の観測 | `dependabot_alerts` の宣言 | Dependabot security updates の変更予定の適用可能性 |
|---|---|---|
| 有効 | 問わない | 適用可能 |
| 無効 | `true` | 適用可能。Dependabot alerts を有効にする変更のあとに更新する |
| 無効 | 省略か `false` | 適用不能 |

`dependabot_alerts: false` と `dependabot_security_updates: true` の同時の宣言は、入力検証で拒否します（[YAML 設定仕様](../../spec/configuration.md) 9章）。期待値どうしは互いの状態を含まないため、Dependabot security updates の `true` は Dependabot alerts を有効にすることを含みません。Dependabot alerts が無効で、有効にする宣言がなければ、利用者が宣言していない Dependabot alerts を reco が有効にしなければ実行できない遷移に当たります。

Dependabot alerts の変更予定に添える変更に伴う副作用と、副作用を生む更新操作の変更前提に含める設定は、次のとおりです。

| Dependabot alerts の変更予定 | `dependabot_security_updates` の宣言 | Dependabot security updates の計画時の観測 | 変更に伴う副作用 | 変更前提に含める Dependabot security updates（計画時に無効なら、副作用を示さなかった設定） |
|---|---|---|---|---|
| 有効にする | 問わない | 問わない | Dependency graph が有効になる | 含めない |
| 無効にする | 省略 | 有効か一時停止中 | Dependency graph が無効になる。Dependabot security updates が計画時の観測値から無効になる | 計画時の観測値 |
| 無効にする | 省略 | 無効 | Dependency graph が無効になる | 計画時の観測値（無効） |
| 無効にする | 宣言する | 問わない | Dependency graph が無効になる | 含めない |

Dependabot alerts の `PUT` が Dependency graph を有効にすることは、公式資料の記載を根拠にします（GitHub リポジトリ API 仕様 6章）。`dependabot_security_updates` を宣言していれば、その変化は宣言した値への変更として同じ変更計画が扱い、先行する Dependabot security updates の変更が無効にしているため、副作用として示さず、変更前提にも含めません。宣言しておらず計画時に無効だった場合も、変更前提には含めます。承認のあとに外部主体が有効にしていれば、Dependabot alerts の `DELETE` が承認の前に示していない変化を起こすためです。

Dependabot security updates の変更の変更前提に、Dependabot alerts が有効であることは含めません。承認のあとに Dependabot alerts が無効にされていれば、GitHub が `422` で拒否し、Dependabot security updates の値は変わらないためです。

### Secret scanning

非公開リポジトリのために Not Comparable になった Secret scanning の有効状態とリポジトリの push protection は、常に適用不能で、理由はリポジトリが非公開であることです。非公開リポジトリへの push protection の有効化は `200` を返して値を変えず（GitHub リポジトリ API 仕様 5章）、更新のあとに期待状態を確かめる手段がなく、公開範囲は管理設定ではないため、同じ対象リポジトリのほかの宣言では解消しません。公開リポジトリでは、Secret scanning の変更予定はどちらの向きも適用可能です。push protection の変更予定の適用可能性は、向きで決まります。リポジトリの push protection を有効にする Mismatch は、Secret scanning の有効状態の計画時の観測値が有効か、同じ対象リポジトリで Secret scanning の有効状態を有効と宣言したときに限り適用可能です。どちらでもなければ適用不能で、理由は Secret scanning が無効であることです。GitHub は成功を応答しても値を変えず、更新のあとに期待状態を確かめられないためです。リポジトリの push protection を無効にする Mismatch は、どちらの場合も適用可能です。

| Secret scanning の計画時の観測 | `secret_scanning` の宣言 | push protection を有効にする変更予定 | push protection を無効にする変更予定 |
|---|---|---|---|
| 有効 | 問わない | 適用可能 | 適用可能 |
| 無効 | `true` | 適用可能。Secret scanning を有効にする変更のあとに更新する | 適用可能 |
| 無効 | 省略 | 適用不能 | 適用可能 |

`secret_scanning: false` と `secret_scanning_push_protection: true` の同時の宣言は、入力検証で拒否します（YAML 設定仕様 9.2）。push protection を有効にする向きを適用不能にする根拠は、Dependabot security updates と異なり、GitHub が拒否せず `200` を返して値を変えないことです。送っても更新後の観測で不一致になるだけで、期待状態に至る手段がありません。無効にする向きは、Secret scanning が無効のときに反映されない動作を確かめておらず、確かめていない条件で止めないため、適用可能とします。反映されなければ、更新後の観測が不一致として検出します。

Secret scanning の変更予定に添える変更に伴う副作用と、副作用を生む更新操作の変更前提に含める設定は、次のとおりです。

| Secret scanning の変更予定 | `secret_scanning_push_protection` の宣言 | push protection の計画時の観測 | 変更に伴う副作用 | 変更前提に含める push protection（計画時に無効なら、副作用を示さなかった設定） |
|---|---|---|---|---|
| 有効にする | 問わない | 問わない | なし | 含めない |
| 無効にする | 省略 | 有効 | push protection が計画時の観測値から無効になる | 計画時の観測値 |
| 無効にする | 省略 | 無効 | なし | 計画時の観測値（無効） |
| 無効にする | 宣言する | 問わない | なし | 含めない |

push protection は Secret scanning と同じ更新エンドポイントの設定で、更新後に比べるかは [アーキテクチャ設計](../architecture.md) 5.3節の比較の規則に従います。

push protection を有効にする変更の変更前提には、Secret scanning が有効であることを含めます。承認時に予定した値は有効で、計画時に無効だった場合は、先行する Secret scanning の変更が有効にすることが予定どおりの変化です。Dependabot security updates と扱いを変えるのは、承認のあとに Secret scanning が無効にされていたとき、GitHub が `200` を返して値を変えず、更新操作の結果からは反映されなかったことが分からないためです。push protection を無効にする変更の変更前提には含めません。

### Private vulnerability reporting

非公開リポジトリのために Not Comparable になった Private vulnerability reporting の有効状態は、Secret scanning と同じ理由で常に適用不能です。GitHub は非公開リポジトリへの更新に `404` を返し、更新のあとに期待状態を確かめる手段もないためです（GitHub リポジトリ API 仕様 8章）。公開リポジトリでは、Mismatch は、どちらの向きも、同じ対象リポジトリのほかの宣言と観測によらず適用可能です。前提の設定を持たないためです。依存する設定も持たず、変更に伴う副作用もありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 論理的な設定変更と更新操作

Dependabot の2設定、Secret scanning の2設定、Private vulnerability reporting の有効状態は、それぞれ一つの更新操作からなる一つの論理的な設定変更です。依存する設定と前提の設定をまとめないのは、片方の期待状態だけが一致しなかったときに、どちらの変更が完了したかを分けて報告できるようにするためです（[Actions の設計](actions.md) の `sha_pinning_required` と同じ理由）。

依存する2つの設定をともに宣言した場合は、有効にするときは前提の設定（Dependabot alerts、Secret scanning）を先に、無効にするときは依存する設定（Dependabot security updates、push protection）を先に更新します。無効にするときに前提の設定を先に更新すると、依存する設定が GitHub によって先に無効になり、その変更予定の変更前提が成り立たなくなるためです。前提の設定を無効にし、依存する設定を有効にする宣言は、GitHub がその状態を受け付けないため実現できません。

論理的な設定変更の実行順は、YAML 設定仕様の表の並び（Dependabot alerts、Dependabot security updates）を基本とし、Dependabot alerts を無効にする変更予定があるときだけ、同じ対象リポジトリの Dependabot security updates の変更予定をその直前に実行します。変更計画の表示は表の並びのままです。

| Dependabot alerts の変更予定 | Dependabot security updates の変更予定 | 実行順 |
|---|---|---|
| 有効にする | あり | Dependabot alerts、Dependabot security updates |
| 無効にする | あり（無効にする） | Dependabot security updates、Dependabot alerts |
| なし | あり | Dependabot security updates |
| あり | なし | Dependabot alerts |

Dependabot alerts を無効にする変更予定と、Dependabot security updates を有効にする変更予定が並ぶことはありません。入力検証で拒否する宣言の組み合わせだからです。二つの論理的な設定変更のあいだには更新要求の間隔（[アーキテクチャ設計](../architecture.md) 6.2節）が入ります。

Secret scanning の実行順も Dependabot と同じ形です。YAML 設定仕様の表の並び（Secret scanning、push protection）を基本とし、Secret scanning を無効にする変更予定があるときだけ、同じ対象リポジトリの push protection の変更予定をその直前に実行します。Secret scanning を無効にする変更予定と、push protection を有効にする変更予定が並ぶことはありません。入力検証で拒否する宣言の組み合わせだからです。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 更新方式

| 更新の対象 | 更新方式で守ること |
|---|---|
| Secret scanning の有効状態、リポジトリの push protection | `security_and_analysis` の中の、変える1項目の `status` だけを送り、省略した中の項目とトップレベルのフィールドの保持に依拠して部分更新する |
| Dependabot alerts | 有効にするときは本文のない `PUT`、無効にするときは本文のない `DELETE` を送る。この更新エンドポイントで reco が観測する設定はほかになく、保全を確かめる設定がない |
| Dependabot security updates | Dependabot alerts と同じく、本文のない `PUT` と `DELETE` を送る。この更新エンドポイントで reco が観測する設定はほかになく、保全を確かめる設定がない |
| Private vulnerability reporting の有効状態 | Dependabot alerts と同じく、本文のない `PUT` と `DELETE` を送る。この更新エンドポイントで reco が観測する設定はほかになく、保全を確かめる設定がない |

Secret scanning の有効状態とリポジトリの push protection は、General と同じ `PATCH /repos/{owner}/{repo}` に、`{"security_and_analysis":{"secret_scanning":{"status":"enabled"}}}` のように、変える1項目の `status` だけを送ります。値は有効なら `enabled`、無効なら `disabled` です。正常応答は `200` で、成功の判定は General と同じく本文が取得契約を満たすことです。`200` の本文が `security_and_analysis` を含まない場合を考え、本文でこの項目の値を確かめません（[アーキテクチャ設計](../architecture.md) 6.1節）。1項目だけを送る `PATCH` が、もう一方の項目、`security_and_analysis` のほかの項目、トップレベルの一般設定を変えなかったことを、公開リポジトリで実際の動作により確かめています（[GitHub リポジトリ API 仕様](../../research/repositories.md) 5章）。これが部分更新の根拠です。更新後に再取得して更新直前の値と比べるのは、General の2設定と、Secret scanning の2設定のうちもう一方です。副作用として示した設定の扱いは、アーキテクチャ設計 5.3節の比較の規則に従います。

Dependabot alerts は `/repos/{owner}/{repo}/vulnerability-alerts`、Dependabot security updates は `/repos/{owner}/{repo}/automated-security-fixes` を更新エンドポイントとし、期待値が `true` なら本文のない `PUT`、`false` なら本文のない `DELETE` を送ります。正常応答はどれも本文のない `204` で、ほかの `2xx` は更新の契約の外として成否不明にします（アーキテクチャ設計 6.1節）。すでに同じ状態でも `204` が返り、直後の取得が更新後の値を返すことは、実際の動作で確かめています（GitHub リポジトリ API 仕様 6・7章）。それぞれの更新エンドポイントで reco が観測する設定はその設定だけのため、更新後に更新直前の値と比べる設定はありません。Dependabot alerts の `PUT` と、Dependabot security updates の `PUT` と `DELETE` が、もう一方の設定を変えないことは実際の動作で確かめており（同 6・7章）、これを保全の根拠とします。Dependabot alerts の `DELETE` が Dependabot security updates と Dependency graph を無効にする影響は、変更に伴う副作用として [適用可能性と副作用](#適用可能性と副作用) で扱います。

Private vulnerability reporting の有効状態は、`/repos/{owner}/{repo}/private-vulnerability-reporting` を更新エンドポイントとし、Dependabot と同じく、期待値が `true` なら本文のない `PUT`、`false` なら本文のない `DELETE` を送ります。正常応答は本文のない `204` で、ほかの `2xx` は成否不明、`422` は GitHub が要求を拒否した応答として更新操作の失敗です（アーキテクチャ設計 6.1節）。すでに同じ状態でも `204` が返り、直後の取得が更新後の値を返すことは、実際の動作で確かめています（GitHub リポジトリ API 仕様 8章）。この更新エンドポイントで reco が観測する設定はこの設定だけのため、更新後に更新直前の値と比べる設定はありません。公開リポジトリで `PUT` と `DELETE` を送った前後で、reco が観測するほかの設定が変わらなかったことを実際の動作で確かめており（同 8章）、これを保全の根拠とします。

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
| 非公開リポジトリで Secret scanning の領域を読み取り、応答に `security_and_analysis` がなかった | Secret scanning の有効状態とリポジトリの push protection は観測不能。理由はリポジトリが非公開であること | 権限が足りず観測操作が失敗した。または、値がないので無効 |
| 非公開リポジトリで Secret scanning の領域を読み取り、応答が二つの設定の値を含んでいた | 観測不能。観測不能かどうかは公開範囲で決め、応答の値を観測値にしない | 応答にあるので観測値とする |
| 公開リポジトリで Secret scanning の領域を読み取り、応答に `security_and_analysis` がなかった | 観測操作の失敗。公開リポジトリで値が返らない理由は権限の不足である | 取得条件による観測不能として表示へ進む。または、無効とみなす |
| General だけを読み取る観測で、応答に `security_and_analysis` がなかった | 観測は成り立つ。Secret scanning の領域は読み取らなかった設定の領域である | 必要なフィールドが欠けたので観測操作の失敗 |
| Dependabot security updates を `true` と宣言し、一時停止中だった | 一時停止中の値は有効と同じ値で、Match。変更予定にならない | 一時停止を解くまで差分が消えないので Mismatch |
| Dependabot security updates を宣言せずに Dependabot alerts を無効にすると宣言し、Dependabot security updates が有効だった | Dependabot alerts の Mismatch で適用可能。Dependency graph と Dependabot security updates が無効になることを副作用として添え、Dependabot security updates の計画時の観測値を変更前提に含める | 宣言していない設定を GitHub が変えるので適用不能 |
| Dependabot alerts が無効で宣言されていないリポジトリで、Dependabot security updates を `true` と宣言した | Mismatch で適用不能。理由は Dependabot alerts が無効であること | 期待値が Dependabot alerts の有効を含むので、有効にしてから更新する |
| Dependabot alerts と Dependabot security updates の両方を無効にすると宣言し、どちらも有効だった | 二つの論理的な設定変更。Dependabot security updates、Dependabot alerts の順に実行する。Dependabot alerts の変更に Dependabot security updates の副作用は添えない | 管理設定の並びのまま Dependabot alerts を先に無効にする |
| Dependabot alerts を無効にする変更を承認し、副作用として示した Dependabot security updates が、更新直前に無効になっていた | 関係する現在値の変化で、変更前提は不成立。変更結果は未実行 | 無効にする予定の設定がすでに無効なので、変更前提は成立する |
| 非公開リポジトリで `secret_scanning` を宣言した | Not Comparable で適用不能。理由はリポジトリが非公開であること | 値がないので無効とみなし、`false` の宣言と Match |
| Secret scanning が無効で宣言されていないリポジトリで、リポジトリの push protection を `true` と宣言した | Mismatch で適用不能。理由は Secret scanning が無効であること | GitHub が成功を応答するので適用可能。または、reco が Secret scanning を有効にしてから更新する |
| リポジトリの push protection を宣言せずに Secret scanning を無効にすると宣言し、リポジトリの push protection が有効だった | Secret scanning の有効状態の Mismatch で適用可能。リポジトリの push protection が無効になることを副作用として添え、計画時の観測値を変更前提に含める。更新後には比べない | 同じ更新エンドポイントの設定が変わったので、変更対象外の設定の予期しない変化 |
| 同じ宣言で、リポジトリの push protection が計画時に無効で、更新後に有効だった | 変更対象外の設定の予期しない変化。副作用を示さなかった設定は更新後にも比べる | 副作用に関わる設定なので比べない |
| Secret scanning の有効状態とリポジトリの push protection の両方を無効にすると宣言し、どちらも有効だった | 二つの論理的な設定変更。リポジトリの push protection、Secret scanning の有効状態の順に実行する | 管理設定の並びのまま Secret scanning を先に無効にする |
| General の設定だけを変える更新の前後で、Secret scanning の有効状態が変わった | 変更対象外の設定の予期しない変化で、変更結果は失敗 | Secret scanning を宣言していないので比べない |
| リポジトリの push protection を有効にする変更を承認し、更新直前に Secret scanning が無効になっていた | 関係する現在値の変化で、変更前提は不成立。変更結果は未実行 | 変えるのはリポジトリの push protection だけなので、変更前提は成立する |
| 非公開リポジトリで Private vulnerability reporting の領域を読み取った | 観測不能。理由はリポジトリが非公開であること。読み取り要求は送らない | 取得を送り、「対象が見つからない」を無効と読む。または観測操作の失敗として、ほかの設定の表示も止める |
| 公開と観測した直後に、Private vulnerability reporting の有効状態の取得に GitHub が「対象が見つからない」と応答した | 観測操作の失敗。二つの読み取りのあいだに公開範囲が変わった可能性がある | 非公開と推測して観測不能とする。または、admin のアクセスを確かめて無効と読む |
| Dependabot alerts だけを宣言したリポジトリの変更計画の確認 | Private vulnerability reporting の領域は読み取らなかった設定の領域で、読み取り要求を送らない | 同じ `security` の設定なので読み取る |
| 非公開リポジトリで `private_vulnerability_reporting` を宣言した | Not Comparable で適用不能。理由はリポジトリが非公開であること。Secret scanning の2設定と同じ理由である | 値がないので無効とみなし、`false` の宣言と Match |
| Private vulnerability reporting の有効状態の変更を承認し、更新直前の観測でリポジトリが非公開になっていた | 観測値だった設定が観測不能になっており、関係する現在値の変化。更新要求を送らず、変更結果は未実行 | 比べる値がないので、変更前提は成立する |
| Private vulnerability reporting の有効状態の更新要求を、GitHub が処理できない要求として拒否した | 更新操作の結果は失敗で、変更結果も失敗 | 非公開と推測して、適用不能に読み替える |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [アーキテクチャ設計](../architecture.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [CLI 仕様](../../spec/cli.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
