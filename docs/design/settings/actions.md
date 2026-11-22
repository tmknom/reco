# Actions の設計

設定宣言の `actions`（[YAML 設定仕様](../../spec/configuration.md) 4〜6章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Actions の実行許可 | Actions Permissions | Actions の有効状態、Actions 許可ポリシー、選択方式の Actions 許可条件など、Actions と再利用可能なワークフローを実行できる範囲に関する設定の領域 |
| Actions の有効状態 | Actions Enabled | リポジトリで GitHub Actions を有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない |
| Actions 許可ポリシー | Allowed Actions Policy | 許可する Actions と再利用可能なワークフローの範囲を定める上位設定 |
| 選択方式の Actions 許可条件 | Selected Actions Criteria | 上位の許可ポリシーが `selected` のときに適用される詳細な許可条件 |
| ワークフロー権限 | Workflow Permissions | ワークフローの `GITHUB_TOKEN` の既定権限と、GitHub Actions による Pull Request の作成と承認の許可からなる設定の領域 |
| リポジトリ外のワークフローからのアクセス | Actions Access | リポジトリの外のワークフローが、このリポジトリの Actions と再利用可能なワークフローを使える範囲を定める設定の領域。設定は `access_level` の一つで、非公開リポジトリにだけ適用される |

Selected Actions Criteria は、詳細な許可条件をまとめて指す呼称で、GitHub 公式の用語ではありません。

`sha_pinning_required` は単独の管理の単位で、Actions 許可ポリシーと同じ更新エンドポイントで更新しますが、設定グループにはしません。一方を変える更新がもう一方を変えず、期待値を別々に確かめられるためです。

Actions 許可ポリシーの期待値は、そのポリシーで Actions を実行できること、つまり Actions が有効であることを含みます（[YAML 設定仕様](../../spec/configuration.md) 4章）。そのため、Actions 許可ポリシーを宣言したリポジトリで Actions を有効にすることは、Actions 許可ポリシーという管理設定の変更であり、未管理設定の変更には当たりません。このリポジトリで Actions の有効状態を有効と宣言しても、期待状態は増えません。Actions の有効状態は管理設定にならず、有効にする変更予定は Actions 許可ポリシーの一つだけです。同じ状態の変化を二つの管理設定の変更予定として数えないためです。

Actions の有効状態が管理設定になるのは、Actions 許可ポリシーを宣言せずに有効状態を宣言したリポジトリです。無効を宣言できるのも、この場合に限ります（YAML 設定仕様 4.1）。無効のあいだは Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件を観測できず、それらの期待値を確かめられないためです。どちらも宣言していないリポジトリでは、Actions の有効状態は未管理設定です。

許可パターンが同じ文字列を重ねて含む場合の扱いは、YAML 設定仕様 4.2 に従います。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 観測

Actions の実行許可は、二つの読み取り要求で読みます。`GET /repos/{owner}/{repo}/actions/permissions` は `GET /repos/{owner}/{repo}` が成功したときだけ送ります。`GET /repos/{owner}/{repo}/actions/permissions/selected-actions` は、Actions 許可ポリシーを `selected` と観測したときだけ送ります。それ以外のときは GitHub が取得を拒否するためです（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 4章）。

ワークフロー権限の応答はリポジトリ ID を含まないため（GitHub Actions 権限 API 仕様 5.1）、ワークフロー権限は別の読み取り要求で読み、その要求は `GET /repos/{owner}/{repo}` が成功したときだけ送ります。

リポジトリ外のワークフローからのアクセスは、`GET /repos/{owner}/{repo}/actions/permissions/access` で読みます。この要求は、`GET /repos/{owner}/{repo}` の応答の `private` が真のときだけ送ります。公開リポジトリには設定が適用されず、GitHub が取得を拒否するためです（GitHub Actions 権限 API 仕様 6章）。送るかどうかは `private` の観測値で決め、取得の `422` から公開であることを推測しません。権限のない公開リポジトリには `403` が返り、`422` が公開範囲だけを理由に返るとは確かめられていないためです。個人アカウント所有のリポジトリは internal の公開範囲を持たないため、`private` の真偽で足ります（[GitHub リポジトリ API 仕様](../../research/repositories.md) 3章）。`private` の取得契約は [General の設計](general.md) の観測が定めます。

取得条件による観測不能として扱う場合は次のとおりです。

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| Actions が無効（`enabled: false`） | Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件 | 応答に `allowed_actions` が含まれず、`sha_pinning_required` は保存された値によらず `false` で返る（GitHub Actions 権限 API 仕様 3.1） |
| Actions 許可ポリシーが `selected` でない | 選択方式の Actions 許可条件 | 取得が `409` を返し、値は返らない（GitHub Actions 権限 API 仕様 4章） |
| リポジトリが公開（`private: false`） | `access_level` | 公式資料が非公開リポジトリにだけ適用されると記載し、公開リポジトリへの取得は `422` を返して値を返さない（GitHub Actions 権限 API 仕様 6章） |

Actions の有効状態は取得条件を持たず、`GET /repos/{owner}/{repo}/actions/permissions` の応答が取得契約に適合すれば必ず観測値になります。Actions の有効状態だけを管理設定とするリポジトリでも、計画時の観測は Actions の実行許可を読み取ります。

Actions が有効のときに `allowed_actions` または `sha_pinning_required` が欠けた応答と、3項目のいずれかが欠けた Selected actions の応答は、取得契約に適合しない応答です。`sha_pinning_required` と Selected actions の3項目は公式ドキュメント上は任意のフィールドですが、欠落の理由を確認できないため、既定値で補いません。`selected` と観測したあとの Selected actions の取得が `409` を返した場合も、観測不能にせず観測操作の失敗とします（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。

`access_level` の取得条件は公開範囲だけです。Actions が無効のリポジトリでも取得は値を返すため（GitHub Actions 権限 API 仕様 6章）、Actions の有効状態は取得条件になりません。定義にない `access_level` の値と、`access_level` の欠けた応答は、取得契約に適合しない応答です。非公開と観測したあとの取得が `422` を返した場合も、観測不能にせず観測操作の失敗とします。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 適用可能性と副作用

Actions が無効のために Not Comparable になった Actions 許可ポリシーは、宣言したポリシーで Actions を有効にする変更として適用可能です。Actions 許可ポリシーが `selected` でないか Actions が無効であるために Not Comparable になった選択方式の Actions 許可条件も、適用可能です。この設定グループを宣言できるのは Actions 許可ポリシーを `selected` と宣言したときに限られ（[YAML 設定仕様](../../spec/configuration.md) 4.1）、同じ変更計画がポリシーを `selected` にするためです。Actions の有効状態は取得条件を持たないため Not Comparable にならず、その Mismatch は、有効にする向きも無効にする向きも適用可能です。

Actions が無効のために Not Comparable になった `sha_pinning_required` は、同じ対象リポジトリの期待状態が Actions の有効を含むときに限り適用可能です。期待状態が Actions の有効を含むのは、Actions 許可ポリシーを宣言したときと、Actions の有効状態を有効と宣言したときです。このとき、同じ変更計画が先に Actions を有効にし、そのあとで値を観測して期待状態を確かめられます。含まないときは適用不能で、理由は Actions が無効のままであることです。無効のままでは更新のあとに期待状態を確かめられず、reco が Actions を有効にすれば、利用者が宣言していない設定を変えることになるためです。Actions が有効で Mismatch になった `sha_pinning_required` は、ほかの宣言によらず適用可能です。

| 観測 | 同じ対象リポジトリの宣言 | 照合結果 | 適用可能性 |
|---|---|---|---|
| 有効で、宣言と同じ値 | 問わない | 一致 | なし |
| 有効で、宣言と異なる値 | 問わない | 不一致 | 適用可能 |
| 無効 | `enabled: true` か Actions 許可ポリシー | 比較不能 | 適用可能。Actions を有効にする変更のあとに更新する |
| 無効 | どちらもない | 比較不能 | 適用不能 |

Actions が無効のあいだ、`sha_pinning_required` の取得は保存された値を示さず（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 3.1）、無効のまま送った値は成功の応答が返っても保存されません（GitHub Actions 権限 API 仕様 3.2）。更新して期待状態を確かめるには Actions が有効である必要があり、有効にする宣言がなければ、利用者が宣言していない Actions の有効状態を reco が変えなければ実行できない遷移に当たります。

`access_level` の変更予定の適用可能性は、公開範囲だけで決まります。Actions の有効状態と、同じ対象リポジトリのほかの宣言は関係しません。

| 観測 | 照合結果 | 適用可能性 |
|---|---|---|
| 非公開で、宣言と同じ値 | 一致 | なし |
| 非公開で、宣言と異なる値（宣言できない `organization` を含む） | 不一致 | 適用可能 |
| 公開 | 比較不能 | 適用不能 |

公開リポジトリのために Not Comparable になった `access_level` は、常に適用不能で、理由はリポジトリが公開であることです。公開リポジトリには `access_level` が適用されず、取得は値を返さず（GitHub Actions 権限 API 仕様 6章）、更新のあとに期待状態を確かめる手段がないためです。公開範囲は管理設定ではなく、reco が変える設定でもないため、同じ対象リポジトリのほかの宣言では解消しない点が `sha_pinning_required` と異なります。非公開リポジトリで Mismatch になった `access_level` は、Actions の有効状態によらず適用可能です。Actions が無効でも、`access_level` の取得と更新はどちらも成り立つためです。

Actions 許可ポリシーを `selected` から `all` / `local_only` へ変える変更予定は、既存の選択方式の Actions 許可条件を失わせます。Actions が無効のリポジトリで `selected` 以外のポリシーを宣言した場合は、無効にする前のポリシーが `selected` なら失われ得ることを、可能性として添えます（[ドメインモデル](../domain.md) の照合結果と適用可能性）。一方、遷移先が `selected` の変更で選択方式の Actions 許可条件が宣言した値に置き換わることは、管理設定の変更であり、副作用ではありません。どの遷移にどの副作用を示すかは、[論理的な設定変更と更新操作](#論理的な設定変更と更新操作) の遷移表が定めます。

Actions を無効から有効にする変更予定は、宣言していない設定の値を変えずに、その効力を戻します。GitHub は無効にする前の Actions 許可ポリシー、選択方式の Actions 許可条件、`sha_pinning_required` を残しており、有効に戻すとそれらの値で Actions を実行できるようになるためです。このうち未管理設定であるものが再開する設定であり、有効にする変更予定に添えます。値が失われる副作用とは起きることが異なるため、副作用には数えません。管理設定を除くのは、同じ変更計画が宣言した値に変えるためです。副作用として示す設定も、値が失われ得るため、再開する設定に数えません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |

## 論理的な設定変更と更新操作

選択方式の Actions 許可条件の取得・更新には、Actions 許可ポリシー（`allowed_actions`）が `selected` であるという前提があります。`all` / `local_only` のときと Actions が無効のとき、GitHub は選択方式の Actions 許可条件の取得も更新も `409` で拒否します（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 4章）。そのため `all` / `local_only` から `selected` へ変更する際は、Actions 許可ポリシーを先に更新するしかなく、二つの更新操作の間に中間状態が生じます。

Actions 許可ポリシーの遷移は次の表に従います。遷移元は観測した現在値、遷移先は宣言した値です。Actions が無効のときは遷移元のポリシーが応答に含まれないため（GitHub Actions 権限 API 仕様 3.1）、遷移元を「無効」として扱い、照合結果は比較不能とします。

| 遷移元 | 遷移先 | 更新操作の順序 | 中間状態で取得できる値と取得できない値 | 適用不能になる条件 |
|---|---|---|---|---|
| `all` / `local_only` | もう一方 | (1) Actions 許可ポリシー | 中間状態はない | なし |
| `all` / `local_only` | `selected` | (1) Actions 許可ポリシー、(2) 選択方式の Actions 許可条件 | (1) の前は選択方式の Actions 許可条件を取得できない。(1) の後は取得でき、GitHub が戻した値（`github_owned_allowed: true`、`verified_allowed: false`、`patterns_allowed: []`）を返す | なし |
| `selected` | `selected`（3項目だけが異なる） | (1) 選択方式の Actions 許可条件 | 中間状態はない | なし |
| `selected` | `all` / `local_only` | (1) Actions 許可ポリシー | 中間状態はない。(1) の後は選択方式の Actions 許可条件を取得できない | なし。既存の選択方式の Actions 許可条件が失われることを変更計画に示す |
| 無効 | `all` / `local_only` | (1) Actions の有効化と Actions 許可ポリシー（一つの更新操作） | 中間状態はない。(1) の前は Actions 許可ポリシーと選択方式の Actions 許可条件を取得できない | なし。無効にする前のポリシーが `selected` なら選択方式の Actions 許可条件が失われ得ることを変更計画に示す。`sha_pinning_required` を宣言していなければ、無効にする前の値で再開することも示す |
| 無効 | `selected` | (1) Actions の有効化と Actions 許可ポリシー（一つの更新操作）、(2) 選択方式の Actions 許可条件 | (1) の前は Actions 許可ポリシーと選択方式の Actions 許可条件を取得できない。(1) の後は選択方式の Actions 許可条件を取得できる | なし。`sha_pinning_required` を宣言していなければ、無効にする前の値で再開することを変更計画に示す |

`all` / `local_only` / 無効から `selected` への遷移では、Actions 許可ポリシーと選択方式の Actions 許可条件の変更予定を、(1) と (2) をこの順に行う一つの論理的な設定変更にまとめます。完了を判断する期待状態は、Actions 許可ポリシーが `selected` で、選択方式の Actions 許可条件が宣言した値であることです。(2) の直前の変更前提に、Actions 許可ポリシーが `selected` であること（(1) による予定どおりの変化）を含めます。(1) の直後の取得が (1) の前のポリシーを返した場合の扱いは [アーキテクチャ設計](../architecture.md) 6.2節で定めます。中間状態の3項目は (2) ですべて上書きするため、変更前提に含めません。(2) の直前の観測は設定の領域の単位で読むため、選択方式の Actions 許可条件も読み取ります。この読み取りが失敗した場合は、3項目を変更前提に使わなくても観測操作の失敗であり、(2) を開始しません。

遷移元が `local_only` のとき、中間状態は `github_owned_allowed: true` によって GitHub 所有の Actions を許可します。宣言した `github_owned_allowed` が `false` なら、遷移元も宣言した状態も許可しない Actions を (1) から (2) までの間だけ許可することになります。(2) を開始できなかった場合と (2) が失敗した場合は、この中間状態が残ります。reco は (1) を取り消さず、どの更新操作まで終えたかを変更結果に示します。この中間状態は、変更に伴う副作用として変更計画に示しません。論理的な設定変更が完了すれば残らないためです（[ドメインモデル](../domain.md) の変更に伴う副作用）。(1) の更新要求と (2) の直前の観測のあいだには更新要求の間隔（アーキテクチャ設計 6.2節）が入るため、中間状態は少なくとも1秒残ります。

遷移元が無効の遷移では、(1) の直前の変更前提に、Actions が無効で Actions 許可ポリシーを取得できないままであることを含めます。承認時に予定した値が観測不能であるためです（[ドメインモデル](../domain.md) の承認と変更前提）。

`selected` のまま3項目だけを変える場合は、Actions 許可ポリシーの更新を送りません。現在の値と同じ `selected` を送っても、GitHub は `github_owned_allowed` を `true` に変えるためです（GitHub Actions 権限 API 仕様 3.2）。

`selected` から `all` / `local_only` への切替によって、GitHub は選択方式の Actions 許可条件を残さず、`selected` に戻したときには上記の値に戻っています（GitHub Actions 権限 API 仕様 4章）。遷移先が `all` / `local_only` のとき選択方式の Actions 許可条件は宣言できないため（[YAML 設定仕様](../../spec/configuration.md) 4.1）、その既存値は管理範囲外の設定であり、reco はこの消失を防げません。Change Planning は、この遷移を適用可能としたうえで、既存の選択方式の Actions 許可条件が失われることを観測した値とともに変更計画へ含めます（[CLI 仕様](../../spec/cli.md) 4章）。消失を承認前に明示するのは、INV-001 が禁じる管理範囲外の設定の暗黙の削除に当たらないようにするためです。そのため、(1) の直前の変更前提に、変更計画に示した選択方式の Actions 許可条件の値を含めます。更新後は選択方式の Actions 許可条件を取得できないため、その値を保全済みとはみなさず、消失したものとして扱います。

Actions 許可ポリシーを宣言せずに Actions の有効状態を宣言した場合（YAML 設定仕様 4.1）、有効状態の変更予定は、次の表のどちらの向きも適用可能で、一つの更新操作からなる一つの論理的な設定変更になります。

| 遷移元 | 宣言 | 更新操作 | 更新後に取得できない値 |
|---|---|---|---|
| 無効 | `enabled: true` | (1) Actions の有効状態（`enabled: true` だけを送る） | なし。無効にする前の Actions 許可ポリシー、選択方式の Actions 許可条件、`sha_pinning_required` で有効になり、そのことを変更計画に示す（CLI 仕様 4章） |
| 有効 | `enabled: false` | (1) Actions の有効状態（`enabled: false` だけを送る） | Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件 |

変更前提で比べる関係する現在値は、Actions の有効状態だけです。無効にしても、GitHub は Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件の値を保持するため（GitHub Actions 権限 API 仕様 3.2・4章）、変更計画に示す消失はなく、これらの値を変更前提に含めません。Actions 許可ポリシーとともに有効を宣言した場合は、有効状態が管理設定にならないため（[設定と管理の単位](#設定と管理の単位)）、Actions 許可ポリシーの遷移表の遷移元が「無効」の行だけが当たります。無効と Actions 許可ポリシーは同時に宣言できないため（YAML 設定仕様 4.1）、有効状態の変更予定と Actions 許可ポリシーの変更予定が、同じ対象リポジトリの変更計画に並ぶことはありません。

`sha_pinning_required` の変更予定は、一つの更新操作からなる一つの論理的な設定変更です。管理の単位の並び（YAML 設定仕様 4章）により、同じ対象リポジトリの Actions の有効状態、Actions 許可ポリシー、選択方式の Actions 許可条件の論理的な設定変更がすべて完了したあとに実行します。Actions 許可ポリシーと `sha_pinning_required` の両方に変更予定があるときも、一つの更新要求にまとめません。片方の期待状態だけが一致しなかったときに、どちらの変更が完了したかを分けて報告できなくなるためです。変更前提で比べる関係する現在値は、計画時の観測で決まります。

| 計画時の観測 | 関係する現在値と、承認時に予定した値 |
|---|---|
| 有効で、宣言と異なる値 | `sha_pinning_required` が計画時の観測値であること、Actions の有効状態が有効であること |
| 無効（先行する変更が有効にする） | Actions の有効状態が有効であること |

計画時に無効だったリポジトリでは、先行する論理的な設定変更が Actions を有効にすることが予定どおりの変化で、そのあとに観測できる `sha_pinning_required` の値は変更前提に含めません（[ドメインモデル](../domain.md) の承認と変更前提）。更新直前に Actions が無効であれば、変更前提の不成立として更新要求を送りません。このとき観測し直しはしません（アーキテクチャ設計 6.2節の観測し直しは、同じ論理的な設定変更の先行する更新操作に限ります）。Actions を有効にした先行する論理的な設定変更は、更新後の観測で有効を確かめて完了しています。そのあとに無効を観測した原因が外部主体の変更か取得の遅れかは断定できず、Actions を有効にする要求を誤って送るより、止めるほうを選ぶためです。先行する Actions 許可ポリシーの変更が変えた `allowed_actions` の値は、変更前提に含めません。

無効のリポジトリでは、Actions を有効にする論理的な設定変更の完了から `sha_pinning_required` の更新までのあいだ、Actions は無効にする前の `sha_pinning_required` で動きます。更新要求の間隔（アーキテクチャ設計 6.2節）が入るため、この状態は少なくとも1秒残ります。`sha_pinning_required` の論理的な設定変更が完了しなければ、この状態が残ります。この状態は変更計画に示しません。宣言した値に変わるまでの短いあいだの状態で、完了しなかったことは変更結果が示すためです。

`access_level` の変更予定は、一つの更新操作からなる一つの論理的な設定変更です。変更前提で比べる関係する現在値は、`access_level` が計画時の観測値であることだけです。更新直前にリポジトリが公開になっていれば `access_level` が観測不能になり、関係する現在値の変化として更新要求を送りません。更新後に公開になっていれば、期待値と比べる `access_level` が観測不能になるため、期待状態の確認失敗として完了にしません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |

## 更新方式

| 更新の対象 | 更新方式で守ること |
|---|---|
| Actions の有効状態 | `enabled` だけを送り、省略した `allowed_actions` と `sha_pinning_required` の保持に依拠して更新する |
| Actions 許可ポリシー | 必須の `enabled` には `true` を送り、`sha_pinning_required` など送らない値を変えないことを確認できる方式で更新する |
| 選択方式の Actions 許可条件 | 3項目を一体として扱い、Actions 許可ポリシーとの依存関係を考慮する |
| `sha_pinning_required` | 必須の `enabled` には、更新直前に有効と確かめたうえで `true` を送り、省略した `allowed_actions` の保持に依拠して更新する |
| ワークフロー権限 | 管理する値だけを送り、省略したもう一方のフィールドの保持に依拠して部分更新する |
| リポジトリ外のワークフローからのアクセス | `access_level` だけを送る。この更新エンドポイントで reco が観測する設定はほかになく、保全を確かめる設定がない |

Actions の有効状態では、`PUT /repos/{owner}/{repo}/actions/permissions` に `enabled` だけを送ります。省略した `allowed_actions` と `sha_pinning_required` が保持され、選択方式の Actions 許可条件も変わらないことを、無効にする更新と有効に戻す更新のどちらでも、公開・非公開のリポジトリで実際の動作により確認しています（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 3.2・4章）。これが、`enabled` だけを送る方式の根拠です。無効にする更新では、更新後に Actions 許可ポリシーと `sha_pinning_required` が取得条件により観測不能になり（GitHub Actions 権限 API 仕様 3.1）、更新直前の値と比べられません。有効にする更新では、更新直前に同じ理由で観測不能で、比べる基準がありません。どちらの場合も、この確認結果を根拠に保全されるとみなします。無効にする更新が反映されず有効のままだった場合は、どちらも観測できるため、更新直前の値と比べます。

Actions 許可ポリシーでは、`PUT /repos/{owner}/{repo}/actions/permissions` で省略した `sha_pinning_required` が保持されることを実際の動作で確認しています（GitHub Actions 権限 API 仕様 3.2）。必須の `enabled` には `true` を送ります。`allowed_actions` の宣言は Actions が有効であることを期待状態に含むためです（[YAML 設定仕様](../../spec/configuration.md) 4章）。`sha_pinning_required` は送りません。この更新要求は Actions の有効状態も書くため、Actions 許可ポリシーを変える論理的な設定変更は有効状態も変える設定に含め（[ドメインモデル](../domain.md) の変更対象外の設定）、更新直前の値と比べません。更新後は `sha_pinning_required` を再取得し、更新直前の値と比べます。Actions が無効のときは `sha_pinning_required` の応答が保存された値を示さないため（GitHub Actions 権限 API 仕様 3.1）、更新直前の値と比べられません。この場合は、無効の状態から有効化と同時にポリシーを変えても `sha_pinning_required` が保持されるという確認結果（GitHub Actions 権限 API 仕様 3.2）を根拠に、保全されるとみなします。更新後に Actions が無効で `sha_pinning_required` を再取得できない場合は、この根拠に当たりません。期待値の Actions 許可ポリシーも同じ取得条件で観測不能になるため、期待状態の確認失敗として完了にしません。

選択方式の Actions 許可条件では、`PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` に3項目をすべて送ります。3項目は一つの設定グループで、どれも期待値を持つためです。省略したフィールドが保持されることは実際の動作で確認していますが（GitHub Actions 権限 API 仕様 4章）、この方式は保持に依拠しません。空の許可パターンは、`null` の扱いが公開ドキュメントにないため、空の配列として送ります。この更新エンドポイントで reco が観測する設定は選択方式の Actions 許可条件だけのため、更新後に更新直前の値と比べる設定はありません。この更新が `allowed_actions` と `sha_pinning_required` を変えないことは、実際の動作で確認しています（GitHub Actions 権限 API 仕様 4.2）。

`sha_pinning_required` では、`PUT /repos/{owner}/{repo}/actions/permissions` に `enabled: true` と `sha_pinning_required` だけを送ります。省略した `allowed_actions` が保持され、選択方式の Actions 許可条件も変わらないことを、`allowed_actions` を `selected` にし、3項目を既定と異なる値にした状態で、公開・非公開のリポジトリで実際の動作により確認しています（GitHub Actions 権限 API 仕様 3.2）。これが、`allowed_actions` を送らない方式の根拠です。`allowed_actions` を併送しないのは、`selected` を送ると GitHub が `github_owned_allowed` を `true` に変えるためです。必須の `enabled` に送る `true` は既存値の併送に当たるため、更新直前の観測で Actions が有効であることを変更前提として確かめます（[論理的な設定変更と更新操作](#論理的な設定変更と更新操作)）。無効のリポジトリへこの更新要求を送ると、利用者が宣言していない有効化を reco が行うことになるためです。`sha_pinning_required` の期待値は Actions が有効であることを含まないため、Actions の有効状態はこの論理的な設定変更が変える設定に含めず、更新後に Actions 許可ポリシーとともに再取得して、更新直前の値と比べます。選択方式の Actions 許可条件は別の更新エンドポイントの設定のため更新後に比べず、上記の確認結果を根拠に保全されるとみなします。

ワークフロー権限では、`PUT /repos/{owner}/{repo}/actions/permissions/workflow` で省略したフィールドが保持されることを、公開・非公開のリポジトリで実際の動作により確認しています（GitHub Actions 権限 API 仕様 5.2）。これが、管理する値だけを送る方式の根拠です。観測したもう一方の値を併送しないのは、観測から更新までのあいだに外部主体がその値を変えた場合に、古い値で上書きしてしまうためです。更新後に再取得して更新直前の値と比べるのは、`default_workflow_permissions` と `can_approve_pull_request_reviews` のうち、その論理的な設定変更が変えないほうです。2フィールドは Actions が無効のときも取得・更新でき（GitHub Actions 権限 API 仕様 5.1・5.2）、Actions の有効状態はこの方式の条件になりません。

リポジトリ外のワークフローからのアクセスでは、`PUT /repos/{owner}/{repo}/actions/permissions/access` に `access_level` だけを送り、`204` を正常応答とします。`access_level` は必須で、この更新エンドポイントで reco が観測する設定は `access_level` だけのため、更新後に更新直前の値と比べる設定はありません。送った値が直後の取得に反映されることと、Actions が無効のときも取得・更新できることは、実際の動作で確認しています（GitHub Actions 権限 API 仕様 6章）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |

## シナリオと反例

以下は Actions の設定に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| Actions が無効のリポジトリで、Actions 許可ポリシーを宣言した | Actions 許可ポリシーの期待値は Actions が有効であることを含む。有効にすることは管理設定の変更である | Actions の有効状態を宣言していないので、有効にできず適用不能 |
| Actions の有効状態を有効、Actions 許可ポリシーを `all` と宣言し、Actions が無効だった | 管理設定は Actions 許可ポリシーの一つで、変更予定も Not Comparable の一つ | 有効状態の Mismatch とポリシーの Not Comparable の、二つの変更予定になる |
| Actions の有効状態だけを有効と宣言し、Actions が無効だった | Mismatch で適用可能。Actions 許可ポリシーは未管理設定のまま。GitHub が残していた無効にする前の値で Actions が再開するため、その値を観測できなくても、再開することを承認の前に示す | ポリシーを観測できないので Not Comparable |
| Actions が無効のリポジトリで、`sha_pinning_required` だけを宣言した | Not Comparable で適用不能。期待状態が Actions の有効を含まず、無効のままでは更新のあとに期待状態を確かめられない | 値を送れば成功の応答が返るので適用可能。または、reco が Actions を有効にして適用する |
| Actions が無効のリポジトリで、Actions の有効状態を有効、`sha_pinning_required` を `true` と宣言した | 有効状態は Mismatch、`sha_pinning_required` は Not Comparable で、どちらも適用可能。同じ変更計画が先に Actions を有効にする | 現在値を観測できないので適用不能 |
| 上と同じ宣言で、有効に戻す変更予定に添える再開する設定 | Actions 許可ポリシーと選択方式の Actions 許可条件。`sha_pinning_required` は同じ変更計画が宣言した値に変えるため挙げない | 残っていた値で再開するので、`sha_pinning_required` も挙げる |
| Actions が無効のリポジトリで、Actions 許可ポリシーだけを宣言した | ポリシーの変更予定に、無効にする前の `sha_pinning_required` で再開することを添える。ポリシーは宣言した値になり、選択方式の Actions 許可条件が失われ得ることは副作用として示すため、それらは挙げない | 有効状態を宣言していないので、再開する設定は示さない |
| Actions 許可ポリシーと `sha_pinning_required` の両方に変更予定がある | 論理的な設定変更は二つで、変更結果も二つ。ポリシーが完了したあと、`sha_pinning_required` の変更前提で比べるのは `sha_pinning_required` の計画時の観測値と Actions の有効状態で、ポリシーの値は比べない | 同じ更新エンドポイントなので、一つの論理的な設定変更にまとめる |
| Actions が有効のリポジトリで、`sha_pinning_required` の観測値が宣言と異なる | Mismatch で適用可能。Actions の有効状態や Actions 許可ポリシーを宣言しているかは問わない | Actions を有効にする宣言がないので適用不能 |
| Actions の有効状態を無効と宣言し、Actions が有効で、Actions 許可ポリシーが `selected` だった | Mismatch で適用可能。GitHub は選択方式の Actions 許可条件を保持するため、副作用はない | 無効にすると許可条件を取得できなくなるので、消失を副作用として示す |
| Actions の有効状態を無効と宣言し、Actions が無効だった | Match。観測値の「無効」が期待値と同じ | 無効のリポジトリでは値を観測できないので Not Comparable |
| Actions が無効のリポジトリを観測し、応答の `sha_pinning_required` が `false` だった | 観測不能。無効のあいだの応答は保存された値を示さない。Actions の有効状態は観測値を持つ | 応答にあるので `false` を観測値とする |
| Actions 許可ポリシーを `selected` と観測した直後に、選択方式の Actions 許可条件の取得を GitHub が競合として拒否した | 観測操作の失敗。二つの読み取りのあいだにポリシーが変わった可能性がある | 取得条件による観測不能として表示や照合へ進む |
| Actions 許可ポリシーを `all` から `local_only` へ変えると宣言した | 副作用はない。失われる選択方式の Actions 許可条件が現在状態にない | ポリシーを変えるので、常に許可条件の消失を副作用として示す |
| Actions が無効のリポジトリで、Actions 許可ポリシーを `all` と宣言した | Not Comparable で適用可能。無効にする前のポリシーが `selected` なら許可条件が失われ得るため、可能性を副作用として添える | 値を観測できないので副作用はない |
| Actions 許可ポリシーを `all` から `selected` へ変え、選択方式の Actions 許可条件を宣言した | ポリシーは Mismatch、許可条件は Not Comparable で、理由はポリシーが `selected` でないこと。どちらも適用可能。許可条件が宣言した値になることは管理設定の変更 | 許可条件を取得できないので適用不能。または、既存の値を上書きするので副作用 |
| Actions が無効のリポジトリで、ワークフロー権限の値が返った | 観測値であり、照合に使う。取得条件は設定ごとに異なる | Selected Actions Criteria と同じく、Actions が無効なら観測不能とする |
| Actions が無効の非公開リポジトリで、`access_level` の値が返った | 観測値。取得条件は公開範囲だけである | Actions が無効なので観測不能とする |
| 公開リポジトリで `access_level` を宣言した | Not Comparable で適用不能。理由はリポジトリが公開であること。ほかの宣言では解消しない | `none` を宣言したので Match。または、値を送れば成功の応答が返るので適用可能 |
| 非公開リポジトリで `access_level` を `organization` と観測し、`none` を宣言した | Mismatch で適用可能 | `organization` は宣言できない値なので Not Comparable |
| Actions が無効の非公開リポジトリで、`access_level` の観測値が宣言と異なる | Mismatch で適用可能。Actions の有効状態は適用可能性に関係しない | Actions が無効なので適用不能 |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [アーキテクチャ設計](../architecture.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [CLI 仕様](../../spec/cli.md)
- [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
