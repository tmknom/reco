# Actions の設計

設定宣言の `actions`（[YAML 設定仕様](../../spec/configuration.md) 4〜8章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| Actions の実行許可 | Actions Permissions | Actions の有効状態、Actions 許可ポリシー、選択方式の Actions 許可条件、`sha_pinning_required` など、Actions と再利用可能なワークフローを実行できる範囲に関する設定の領域 |
| Actions の有効状態 | Actions Enabled | リポジトリで GitHub Actions を有効にしているかを表す設定。有効か無効のいずれかで、取得条件を持たない |
| Actions 許可ポリシー | Allowed Actions Policy | 許可する Actions と再利用可能なワークフローの範囲を定める上位設定 |
| 選択方式の Actions 許可条件 | Selected Actions Criteria | 上位の許可ポリシーが `selected` のときに適用される詳細な許可条件。3項目からなる設定グループ。GitHub 公式の用語ではない |
| ワークフロー権限 | Workflow Permissions | ワークフローの `GITHUB_TOKEN` の既定権限と、GitHub Actions による Pull Request の作成と承認の許可からなる設定の領域 |
| リポジトリ外のワークフローからのアクセス | Actions Access | リポジトリの外のワークフローが、このリポジトリの Actions と再利用可能なワークフローを使える範囲を定める設定の領域。設定は `access_level` の一つで、非公開リポジトリにだけ適用される |
| Fork Pull Request ワークフローの承認 | Fork Pull Request Approval | 公開リポジトリで、Fork からの Pull Request のワークフローを実行する前に承認を求める範囲を定める設定の領域。設定は承認ポリシー（`approval_policy`）の一つで、公開リポジトリにだけ適用される |
| 承認ポリシー | Approval Policy | Fork Pull Request ワークフローの承認の領域の設定。ワークフローの実行に承認が要るユーザーの範囲を、3つの値のどれかで表す |
| 非公開リポジトリの Fork Pull Request ワークフロー | Private Fork Pull Request Workflows | 非公開リポジトリで、Fork からの Pull Request のワークフローを実行できるかと、その条件を定める設定の領域。設定は4項目からなる実行条件の一つで、非公開リポジトリにだけ適用される |
| 実行条件 | Private Fork Pull Request Workflow Conditions | 非公開リポジトリの Fork Pull Request ワークフローの領域の設定。実行の許可、書き込みトークンの送信、Secrets と Variables の送信、承認の要求の4項目からなる設定グループ |
| 保持期間 | Retention | チェック、ワークフローの実行、コミットのステータス、Artifact、Log を保持する期間を定める設定の領域。設定は保持日数（`days`）の一つ。GitHub は同じ取得で、保持日数の上限（`maximum_allowed_days`）を返す。上限は設定できる値の範囲（[ドメインモデル](../domain.md) の観測と GitHub への要求）に当たる |
| 保持日数 | Retention Days | 保持期間の領域の設定。日数の整数 |
| Repository Variables | Repository Variables | リポジトリに属する Repository Variable からなる設定の領域 |
| Repository Variable | Repository Variable | リポジトリに属し、名前と値を持つ関連リソース。GitHub は ID を付けないため、名前だけで識別する種類として扱う |

承認ポリシーは単独の管理の単位です。実行条件は4項目を一つの値とする設定グループで、照合結果、変更予定、適用可能性を4項目の組に対して一つだけ持ちます。実行を許可しないと GitHub がほかの3項目を `false` にするため、項目ごとの期待状態は互いに独立に成り立たないためです（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 8.2）。保持期間の設定は、`show` が示すだけで、宣言はまだ受け付けません。承認ポリシーと実行条件を一つの領域にしないのは、更新エンドポイントが別であることに加え、公開範囲に合わない側だけを宣言したリポジトリで、宣言していない側の取得を送らないためです。一つの領域にすると、その取得の失敗が、宣言していない設定のために変更計画の確認を止めます。

`sha_pinning_required` は、Actions 許可ポリシーと同じ更新エンドポイントで更新しますが、単独の管理の単位です。一方を変える更新がもう一方を変えず、期待値を別々に確かめられるためです。

Actions 許可ポリシーの期待値は、Actions が有効であることを含みます（[YAML 設定仕様](../../spec/configuration.md) 4章、[ドメインモデル](../domain.md) の管理の単位と管理設定）。Actions 許可ポリシーを宣言したリポジトリでは、Actions の有効状態は管理設定にならず、有効にする変更予定は Actions 許可ポリシーの一つだけです。Actions の有効状態が管理設定になるのは、Actions 許可ポリシーを宣言せずに有効状態を宣言したリポジトリだけで、無効を宣言できるのもこの場合に限ります（YAML 設定仕様 4.1）。

許可パターンは文字列の集合です。GitHub から取得した値が同じ文字列を複数含む場合も一つの要素として扱い、重複の有無だけが違う二つの値は同じ値とします。ワイルドカードやパターンの包含関係は解釈せず、文字列の一致で比べます。Repository Variable の値は、文字列の完全一致で比べます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |

## 観測

Actions の実行許可は、`GET /repos/{owner}/{repo}/actions/permissions` で読み、Actions 許可ポリシーを `selected` と観測したときだけ `GET /repos/{owner}/{repo}/actions/permissions/selected-actions` を続けて送ります。それ以外のときは GitHub が取得を `409` で拒否するためです（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 4章）。ワークフロー権限は `GET /repos/{owner}/{repo}/actions/permissions/workflow` で読みます。

リポジトリ外のワークフローからのアクセスは `GET /repos/{owner}/{repo}/actions/permissions/access` で読み、`GET /repos/{owner}/{repo}` の応答の `private` が真のときだけ送ります。送るかどうかを、取得の `422` から推測しません。権限のない公開リポジトリには `403` が返り、`422` が公開範囲だけを理由に返るとは確かめられていないためです。個人アカウント所有のリポジトリは internal の公開範囲を持たないため、`private` の真偽で足ります（[GitHub リポジトリ API 仕様](../../research/repositories.md) 3章）。

Fork Pull Request ワークフローの承認ポリシーは `GET /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval` で読み、`private` が偽のときだけ送ります。非公開リポジトリの Fork Pull Request ワークフローの実行条件は `GET /repos/{owner}/{repo}/actions/permissions/fork-pr-workflows-private-repos` で読み、`private` が真のときだけ送ります。どちらも、送るかどうかを取得の `422` から推測しない理由は `access_level` と同じです。保持日数とその上限は `GET /repos/{owner}/{repo}/actions/permissions/artifact-and-log-retention` で読み、公開範囲によらず送ります。承認ポリシーは、YAML 設定仕様の章の並びに従い、リポジトリ外のワークフローからのアクセスのあと、Repository Variables の前に読みます。実行条件と保持期間は YAML 設定仕様に章がないため、読み取りの順をここで定めます。承認ポリシーのあと、Repository Variables の前に、実行条件、保持期間の順で読みます。

| 取得条件 | 観測不能になる設定 | 根拠 |
|---|---|---|
| Actions が無効（`enabled: false`） | Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件 | 応答に `allowed_actions` が含まれず、`sha_pinning_required` は保存された値によらず `false` で返る（GitHub Actions 権限 API 仕様 3.1） |
| Actions 許可ポリシーが `selected` でない | 選択方式の Actions 許可条件 | 取得が `409` を返し、値は返らない（GitHub Actions 権限 API 仕様 4章） |
| リポジトリが公開（`private: false`） | `access_level`、非公開リポジトリの Fork Pull Request ワークフローの実行条件 | 公式資料が非公開リポジトリにだけ適用されると記載し、公開リポジトリへの取得は `422` を返して値を返さない（GitHub Actions 権限 API 仕様 6章、8章） |
| リポジトリが非公開（`private: true`） | Fork Pull Request ワークフローの承認ポリシー | 公式資料が公開リポジトリの設定として説明し、非公開リポジトリへの取得は `422` を返して値を返さない（GitHub Actions 権限 API 仕様 7章） |

ワークフロー権限、保持日数、Repository Variables は取得条件を持たず、`access_level`、承認ポリシー、実行条件の取得条件は公開範囲だけです。ワークフロー権限、`access_level`、承認ポリシー、実行条件、保持日数は Actions が無効でも取得でき（GitHub Actions 権限 API 仕様 5.1・6〜9章）、Repository Variables は非公開リポジトリでも一覧を読めます（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 1.1）。`selected` と観測したあとの Selected actions の取得の `409` と、公開範囲が合うと観測したあとの `access_level`、承認ポリシー、実行条件の取得の `422` は、観測不能にせず観測操作の失敗とします（[ドメインモデル](../domain.md) の現在状態・観測結果・観測失敗）。

Actions が有効のときに `allowed_actions` か `sha_pinning_required` が欠けた応答、3項目のいずれかが欠けた Selected actions の応答、定義にない値か欠けた `access_level` の応答は、取得契約に適合しない応答です。`sha_pinning_required` と Selected actions の3項目は公式ドキュメント上は任意のフィールドですが、欠落の理由を確認できないため、既定値で補いません。

定義にない値か欠けた `approval_policy` の応答、4項目のいずれかが欠けた実行条件の応答、`days` か `maximum_allowed_days` が欠けたか整数でない保持期間の応答も、取得契約に適合しない応答です。`run_workflows_from_fork_pull_requests` が `false` のとき、GitHub はほかの3項目を `false` で返しますが（GitHub Actions 権限 API 仕様 8.2）、reco はこの関係を取得契約にせず、返った4項目をそのまま観測値とし、宣言した4項目の組とそのまま比べます。保持日数は、上限を超えていても、`1` 未満でも、観測値としてそのまま持ちます（ドメインモデルの現在状態・観測結果・観測失敗）。上限は公開範囲と、管理する Organization か Enterprise で決まります（同 9章）。

Repository Variables は、`GET /repos/{owner}/{repo}/actions/variables` の一覧で読みます。一覧の要素は個別取得と全フィールドが一致するため（GitHub Actions 変数 API 仕様 4.1）、個別取得は送りません。個別取得は名前の末尾に空白をつけても `200` を返すため、存在の確認にも使えません。

この一覧は `per_page` に GitHub の上限の `30` を送り、1ページの本文の上限を 9 MiB、ページ数の上限を17ページとします（[GitHub API 通信の設計](../github-access.md) 2.1節の、上限を導く一覧）。GitHub は `30` を超える `per_page` を `30` に丸めますが（GitHub Actions 変数 API 仕様 4.1）、本文の上限の導出が1ページ30件を前提にするため、丸めに頼りません。

- **本文の上限**：値は 48,000 バイト、名前は300文字までで（GitHub Actions 変数 API 仕様 2章、3章）、値の1バイトは JSON の文字列で最長6バイト（`\uXXXX`）になる。1要素は約 290,000 バイト、30件で約 8,700,000 バイトで、9 MiB はこれに空白などの余裕を加えた大きさである。1 MiB の上限では、48,000 バイトの値を28件含むページ（1,347,400 バイト）を読めない（同 4.1）
- **ページ数の上限**：GitHub がリポジトリに保存できる500件（同 3章）を、1ページ30件で読み切る17ページ
- **代償**：全ページを読み終えるまで要素を持つため、最大で約 153 MiB をメモリに持つ。要求の時間切れは本文の読み取りを含むため、大きな値を持つリポジトリを遅い回線で読むと通信の失敗で止まり得る。どちらも観測操作の失敗で止まる側であり、受け入れる

一覧の要素は、`name` と `value` が文字列のときだけ取得契約に適合します。`value` の空文字列は値であり、存在する変数として観測します。空の `name`、`name` か `value` の欠落と `null` は取得契約に適合しない応答で、`value` の欠落を空文字列で補いません。値は GitHub が返したとおりに持ち、前後の空白や改行を変えません。GitHub は名前を ASCII の英字の大文字小文字を区別せずに比べるため（GitHub Actions 変数 API 仕様 2章）、宣言の名前と一覧の名前も同じ規則で突き合わせ、一覧に大文字小文字だけが違う名前が二つ以上あれば取得契約に適合しない応答とします。名前に使える文字が ASCII の英数字と `_` に限られるため、同一性を判定できない名前は生じません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 適用可能性と副作用

Actions が無効のために比較不能になった Actions 許可ポリシーは、宣言したポリシーで Actions を有効にする変更として適用可能です。選択方式の Actions 許可条件の比較不能も適用可能です。この設定グループを宣言できるのは Actions 許可ポリシーを `selected` と宣言したときに限られ（[YAML 設定仕様](../../spec/configuration.md) 4.1）、同じ変更計画がポリシーを `selected` にするためです。Actions の有効状態の不一致は、どちらの向きも適用可能です。

`sha_pinning_required` の適用可能性は、次の表に従います。

| 観測 | 同じ対象リポジトリの宣言 | 照合結果 | 適用可能性 |
|---|---|---|---|
| 有効で、宣言と異なる値 | 問わない | 不一致 | 適用可能 |
| 無効 | `enabled: true` か Actions 許可ポリシー | 比較不能 | 適用可能。Actions を有効にする変更のあとに更新する |
| 無効 | どちらもない | 比較不能 | 適用不能。理由は Actions が無効のままであること |

Actions が無効のあいだ、`sha_pinning_required` の取得は保存された値を示さず（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 3.1）、送った値は成功の応答が返っても保存されません（同 3.2）。有効にする宣言がなければ、更新のあとに期待状態を確かめられず、reco が Actions を有効にすれば宣言していない設定を変えることになります。

`access_level` の適用可能性は、公開範囲だけで決まります。非公開リポジトリの不一致（宣言できない `organization` の観測値を含む）は、Actions の有効状態によらず適用可能です。公開リポジトリのために比較不能になった `access_level` は、常に適用不能で、理由はリポジトリが公開であることです。公開リポジトリには `access_level` が適用されず、取得は値を返さず（GitHub Actions 権限 API 仕様 6章）、更新のあとに期待状態を確かめる手段がないためです。公開範囲は管理設定ではないため、`sha_pinning_required` と違い、ほかの宣言では解消しません。

承認ポリシーの適用可能性も、公開範囲だけで決まります。公開リポジトリの不一致は、Actions の有効状態によらず適用可能です。非公開リポジトリのために比較不能になった承認ポリシーは、常に適用不能で、理由はリポジトリが非公開であることです。非公開リポジトリには承認ポリシーが適用されず、取得も更新も `422` を返し（GitHub Actions 権限 API 仕様 7章）、更新のあとに期待状態を確かめる手段がないためです。承認ポリシーの変更予定は、副作用も再開する設定も持ちません。

実行条件の適用可能性も、公開範囲だけで決まります。非公開リポジトリの不一致は、Actions の有効状態によらず適用可能です。公開リポジトリのために比較不能になった実行条件は、常に適用不能で、理由はリポジトリが公開であることです。公開リポジトリへの取得と更新は `422` を返し、公式資料も非公開リポジトリにだけ利用できるとしているためです（GitHub Actions 権限 API 仕様 8章）。

`run_workflows_from_fork_pull_requests` を `false` にする変更では、GitHub がほかの3項目も `false` にし、再び `true` にしても以前の値は戻りません（同 8.2）。消える値はどれも宣言した実行条件の中にあり、入力検証が `false` の宣言の3項目を `false` に限るため（[YAML 設定仕様](../../spec/configuration.md) 7.1）、期待値と GitHub の結果は一致します。そのため、実行条件の変更予定は副作用も再開する設定も持ちません。

実行条件の変更予定のうち、`send_write_tokens_to_workflows` か `send_secrets_and_variables` を、効いていない状態から期待値の `true` に変えるものには、項目ごとに変更予定の注記（[ドメインモデル](../domain.md) の照合結果と適用可能性）を添えます。送信は実行を許可しているときだけ効くため、観測値は `run_workflows_from_fork_pull_requests` と合わせて比べます。GitHub は実行を許可しないあいだ送信を `false` で返しますが、この関係を取得契約にしていないため、送信の観測値だけで比べると、実行を許可して送信が始まる変更を見逃します。注記の文言は、書き込み権限を持つ `GITHUB_TOKEN` が渡ること、Secrets と Variables を使えるようになることの事実だけを述べます（GitHub Actions 権限 API 仕様 8章）。送信が効いたまま変えない項目、`true` から `false` に変える項目、ほかの2項目の変化、比較不能な変更予定には添えません。比較不能では現在値を確かめられず、公開リポジトリでは適用不能のため実行しないためです。

Actions 許可ポリシーを `selected` から `all` / `local_only` へ変える変更予定は、既存の選択方式の Actions 許可条件を失わせます。GitHub は切り替えで許可条件を残さず、`selected` に戻すと既定値（`github_owned_allowed: true`、`verified_allowed: false`、`patterns_allowed: []`）になっているためです（GitHub Actions 権限 API 仕様 4章）。遷移先が `selected` でないとき許可条件は宣言できず、未管理設定のため、この消失を変更に伴う副作用として、観測した値とともに示します。Actions が無効のリポジトリで `selected` 以外のポリシーを宣言した場合は、無効にする前のポリシーが `selected` なら失われ得ることを、可能性として添えます。遷移先が `selected` の変更で許可条件が宣言した値に置き換わることは、管理設定の変更であり、副作用ではありません。

Actions を無効から有効にする変更予定には、再開する設定を添えます。GitHub は無効にする前の Actions 許可ポリシー、選択方式の Actions 許可条件、`sha_pinning_required` を残しており、有効に戻すとそれらの値で Actions を実行できるようになるためです（GitHub Actions 権限 API 仕様 3.2・4章）。どの遷移にどの副作用と再開する設定を示すかは、[論理的な設定変更と更新操作](#論理的な設定変更と更新操作) の表が定めます。

Repository Variable の変更予定は、どれも適用可能で、副作用も再開する設定も持ちません。変数の数の上限（500件）と値の長さの上限は、計画の時点で確かめません（[GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 3章）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 論理的な設定変更と更新操作

### Actions 許可ポリシーと選択方式の Actions 許可条件

選択方式の Actions 許可条件の取得と更新は、Actions 許可ポリシーが `selected` であることを前提とし、そうでなければ GitHub が `409` で拒否します（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 4章）。そのため `selected` への変更では Actions 許可ポリシーを先に更新するしかなく、二つの更新操作のあいだに中間状態が生じます。

遷移は次の表に従います。遷移元は観測した現在値、遷移先は宣言した値です。Actions が無効のときは遷移元のポリシーが応答に含まれないため、遷移元を「無効」とし、照合結果は比較不能です。

| 遷移元 | 遷移先 | 更新操作の順序 | 変更計画に示すこと |
|---|---|---|---|
| `all` / `local_only` | もう一方 | (1) Actions 許可ポリシー | なし |
| `all` / `local_only` | `selected` | (1) Actions 許可ポリシー、(2) 選択方式の Actions 許可条件 | なし |
| `selected` | `selected`（3項目だけが異なる） | (1) 選択方式の Actions 許可条件 | なし |
| `selected` | `all` / `local_only` | (1) Actions 許可ポリシー | 既存の選択方式の Actions 許可条件が失われる（副作用） |
| 無効 | `all` / `local_only` | (1) Actions の有効化と Actions 許可ポリシー | 無効にする前のポリシーが `selected` なら、選択方式の Actions 許可条件が失われ得る（副作用）。`sha_pinning_required` を宣言していなければ、無効にする前の値で再開する |
| 無効 | `selected` | (1) Actions の有効化と Actions 許可ポリシー、(2) 選択方式の Actions 許可条件 | `sha_pinning_required` を宣言していなければ、無効にする前の値で再開する |

`selected` への遷移では、(1) と (2) をこの順に行う一つの論理的な設定変更にまとめます。期待状態は、Actions 許可ポリシーが `selected` で、選択方式の Actions 許可条件が宣言した値であることです。(2) の変更前提には、Actions 許可ポリシーが `selected` であること（(1) による予定どおりの変化）を含め、(1) のあとに GitHub が戻す3項目は (2) がすべて上書きするため含めません。(2) の直前の観測は設定の領域の単位で選択方式の Actions 許可条件も読み、その読み取りの失敗は観測操作の失敗として (2) を開始しません。

遷移元が `local_only` のとき、(1) のあとの既定値 `github_owned_allowed: true` は GitHub 所有の Actions を許可します。宣言した `github_owned_allowed` が `false` なら、遷移元も宣言も許可しない Actions を (1) から (2) までのあいだだけ許可し、(2) を開始できないか失敗すればこの状態が残ります。論理的な設定変更が完了すれば残らない中間状態のため、副作用として示しません。(1) と (2) のあいだには更新要求の間隔（[変更の実行の設計](../execution.md) 5章）が入るため、この状態は少なくとも1秒残ります。

遷移元が無効の (1) では、Actions が無効で Actions 許可ポリシーを取得できないままであることを変更前提に含めます。遷移元が `selected` で遷移先が `all` / `local_only` の (1) では、変更計画に示した選択方式の Actions 許可条件の値を変更前提に含めます。更新後は許可条件を取得できないため、値は失われたものとして扱います。

`selected` のまま3項目だけを変えるときは、Actions 許可ポリシーの更新を送りません。現在と同じ `selected` を送っても、GitHub は `github_owned_allowed` を `true` に変えるためです（GitHub Actions 権限 API 仕様 3.2）。

### Actions の有効状態

Actions 許可ポリシーを宣言せずに Actions の有効状態を宣言した場合、有効状態の変更予定は、`enabled` だけを送る一つの更新操作からなる一つの論理的な設定変更です。変更前提で比べる関係する現在値は、Actions の有効状態だけです。

| 遷移元 | 宣言 | 更新後に取得できない値 | 変更計画に示すこと |
|---|---|---|---|
| 無効 | `enabled: true` | なし | 無効にする前の Actions 許可ポリシー、選択方式の Actions 許可条件、`sha_pinning_required` のうち、宣言していないものが再開する |
| 有効 | `enabled: false` | Actions 許可ポリシー、`sha_pinning_required`、選択方式の Actions 許可条件 | なし。GitHub はこれらの値を保持するため（GitHub Actions 権限 API 仕様 3.2・4章）、消失はない |

### `sha_pinning_required`

`sha_pinning_required` の変更予定は、一つの更新操作からなる一つの論理的な設定変更で、同じ対象リポジトリの Actions の有効状態、Actions 許可ポリシー、選択方式の Actions 許可条件の論理的な設定変更がすべて完了したあとに実行します。Actions 許可ポリシーと一つの更新要求にまとめないのは、片方の期待状態だけが一致しなかったときに、どちらの変更が完了したかを分けて報告できなくなるためです。

| 計画時の観測 | 関係する現在値と、承認時に予定した値 |
|---|---|
| 有効で、宣言と異なる値 | `sha_pinning_required` が計画時の観測値であること、Actions が有効であること |
| 無効（先行する変更が有効にする） | Actions が有効であること |

Actions が有効であることを含めるのは、更新要求が必須の `enabled` に `true` を送るためです（[更新方式](#更新方式)）。計画時に無効だったリポジトリで、先行する変更のあとに観測できる `sha_pinning_required` の値は含めません。更新直前に Actions が無効なら、変更前提の不成立として観測し直さずに止めます。先行する論理的な設定変更は更新後の観測で有効を確かめて完了しており、そのあとの無効が外部主体の変更か取得の遅れかは断定できず、Actions を有効にする要求を誤って送るより止めるほうを選ぶためです。

計画時に無効だったリポジトリでは、Actions を有効にする変更の完了から `sha_pinning_required` の更新までのあいだ、Actions は無効にする前の `sha_pinning_required` で動きます。この状態は、更新要求の間隔のあいだ少なくとも1秒残り、`sha_pinning_required` の変更が完了しなければ残ります。宣言した値に変わるまでの状態で、完了しなかったことは変更結果が示すため、変更計画には示しません。

### `access_level`、承認ポリシー、実行条件、Repository Variable

`access_level` の変更予定は、一つの更新操作からなる一つの論理的な設定変更で、関係する現在値は `access_level` だけです。承認ポリシーの変更予定も、一つの更新操作からなる一つの論理的な設定変更で、関係する現在値は承認ポリシーだけです。実行条件の変更予定は、`PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-workflows-private-repos` の一つの更新操作からなる一つの論理的な設定変更で、関係する現在値は実行条件（4項目の組）だけです。Actions の有効状態は関係する現在値に含めません。Actions が無効でも取得と更新ができ、有効に戻しても値が残るためです（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 8章）。

Repository Variable の変更予定は、変数ごとに一つの更新操作からなる一つの論理的な設定変更で、作成は `POST /repos/{owner}/{repo}/actions/variables`、値の変更は `PATCH /repos/{owner}/{repo}/actions/variables/{name}`、削除は `DELETE /repos/{owner}/{repo}/actions/variables/{name}` です。作成、値の変更、削除が混ざっても、設定宣言で変数を書いた順に実行します。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `alignment` | `INV-005` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## 更新方式

### Actions の有効状態、Actions 許可ポリシー、`sha_pinning_required`

三つは同じ `PUT /repos/{owner}/{repo}/actions/permissions` で更新し、それぞれ送るフィールドを変えます。

- **Actions の有効状態**：`enabled` だけを送る。省略した `allowed_actions` と `sha_pinning_required` が保持され、選択方式の Actions 許可条件も変わらないことを、無効にする更新と有効に戻す更新で、公開・非公開のリポジトリで確かめている（[GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) 3.2・4章）
- **Actions 許可ポリシー**：必須の `enabled` に `true` と、`allowed_actions` を送る。`sha_pinning_required` は送らず、省略した値が保持されることを確かめている（同 3.2）。`enabled` の `true` は期待値が含む状態のため、Actions の有効状態はこの論理的な設定変更が変える設定に含める
- **`sha_pinning_required`**：`enabled: true` と `sha_pinning_required` だけを送る。省略した `allowed_actions` が保持され、選択方式の Actions 許可条件も変わらないことを、`selected` で3項目を既定と異なる値にした状態で、公開・非公開のリポジトリで確かめている（同 3.2）。`allowed_actions` を併送しないのは、`selected` を送ると GitHub が `github_owned_allowed` を `true` に変えるためである。`enabled` の `true` は既存値の併送で、期待値は Actions の有効を含まないため、Actions の有効状態は変える設定に含めず、変更前提と更新後の比較の両方で確かめる

更新後に更新直前の値と比べるのは、この更新エンドポイントで観測する Actions の有効状態、Actions 許可ポリシー、`sha_pinning_required` のうち、その論理的な設定変更が変えない設定です。選択方式の Actions 許可条件は別の更新エンドポイントの設定のため比べず、上の確認結果を保全の根拠とします。Actions が無効のあいだは Actions 許可ポリシーと `sha_pinning_required` が観測不能で比べられないため、次の確認結果を保全の根拠とします。

- Actions を無効にする更新と有効に戻す更新で、両者が保持された（同 3.2）
- 無効の状態から有効化と同時にポリシーを変えても、`sha_pinning_required` が保持された（同 3.2）

### 選択方式の Actions 許可条件

`PUT /repos/{owner}/{repo}/actions/permissions/selected-actions` に3項目をすべて送ります。3項目は一つの設定グループで、どれも期待値を持つためです。省略したフィールドの保持は確かめていますが（GitHub Actions 権限 API 仕様 4章）、この方式は保持に依拠しません。空の許可パターンは、`null` の扱いが公開ドキュメントにないため、空の配列として送ります。この更新エンドポイントで観測する設定はこの設定グループだけで、更新後に比べる変更対象外の設定はありません。この更新が `allowed_actions` と `sha_pinning_required` を変えないことを確かめています（同 4.2）。

### ワークフロー権限

`PUT /repos/{owner}/{repo}/actions/permissions/workflow` に管理する値だけを送ります。省略したフィールドが保持されることを、公開・非公開のリポジトリで確かめています（GitHub Actions 権限 API 仕様 5.2）。観測したもう一方の値を併送しないのは、観測から更新までのあいだに外部主体がその値を変えた場合に、古い値で上書きするためです。更新後には、`default_workflow_permissions` と `can_approve_pull_request_reviews` のうち、その論理的な設定変更が変えないほうを比べます。

### リポジトリ外のワークフローからのアクセス

`PUT /repos/{owner}/{repo}/actions/permissions/access` に必須の `access_level` だけを送り、`204` を正常応答とします。この更新エンドポイントで観測する設定は `access_level` だけで、更新後に比べる変更対象外の設定はありません。送った値が直後の取得に反映されることと、Actions が無効のときも更新できることを確かめています（GitHub Actions 権限 API 仕様 6章）。

### Fork Pull Request ワークフローの承認ポリシー

`PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval` に必須の `approval_policy` だけを送り、`204` を正常応答とします。この更新エンドポイントで観測する設定は承認ポリシーだけで、更新後に比べる変更対象外の設定はありません。3つの値がどれも反映されることと、Actions が無効のときも更新できることを確かめています（GitHub Actions 権限 API 仕様 7章、7.2）。更新の直後の取得が更新前の値を返すことがあるため（同 7.2）、期待状態は、変更の実行の設計の更新後の観測し直しで確かめます。

### 非公開リポジトリの Fork Pull Request ワークフローの実行条件

`PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-workflows-private-repos` に4項目をすべて送り、`204` を正常応答とします。4項目は一つの設定グループで、どれも期待値を持つためです。省略したフィールドの保持は確かめていますが（GitHub Actions 権限 API 仕様 8.2）、この方式は保持に依拠しません。4項目を項目ごとの更新操作に分けないのは、分けると、宣言した承認の要求が反映される前に、ワークフローの実行や Secrets の送信だけが有効になる状態が、更新操作のあいだに生じ得るためです。この更新エンドポイントで観測する設定はこの設定グループだけで、更新後に比べる変更対象外の設定はありません。4項目を明示した一つの要求が4項目とも反映されることと、Actions が無効のときも更新できることを確かめています（同 8章、8.2）。

`run_workflows_from_fork_pull_requests` を `false` にする更新では、GitHub は送った値によらずほかの3項目を `false` にします（同 8.2）。入力検証が、この場合の3項目の期待値を `false` に限るため（[YAML 設定仕様](../../spec/configuration.md)）、更新後の期待状態は一致します。更新の直後の取得が更新前の値を返すことがあるため（同 8.2）、期待状態は、変更の実行の設計の更新後の観測し直しで確かめます。

### Repository Variable

| 更新操作 | 送るもの | 正常応答 | GitHub の拒否（更新操作の失敗） |
|---|---|---|---|
| 作成 | `name` と `value` | `201`。本文は空のオブジェクトで、解釈しない | 同じ名前（大文字小文字だけが違う名前を含む）があれば `409`、上限を超える値は `422`。どちらも値を変えない |
| 値の変更 | `value` だけ | `204` | 存在しない名前は `404` で、変数は作られない。上限を超える値は `422` |
| 削除 | 本文なし | `204` | 存在しない名前は `404` |

根拠は [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) 1章・3章・5章です。値の変更で `name` を送らないのは、reco が変数の名前を変えないためです。値の変更と削除の宛先の名前は宣言の綴りにします。GitHub は名前を大文字で保存し、宣言の名前は英小文字を含まないため、宣言の綴りが保存された名前と一致します。応答を受け取れなかった作成は再送しません。応答が返らないまま変数が作られていた例があるためです（同 5.1）。

変数の更新エンドポイントで観測する設定はその変数だけで、同じ一覧のほかの変数は比べません。作成、同じ名前の作成、値の変更、削除のどれでも、ほかの変数が変わらなかったことを確かめており（同 5.4）、これを保全の根拠とします。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub Actions 変数 API 仕様](../../research/actions-variables.md) | Repository File |

## シナリオと反例

以下は Actions の設定に固有の事実に依拠する解釈の例です。独立した入力条件や実行規則を定義するものではありません。

| 状況 | 解釈 | 誤った解釈 |
|---|---|---|
| Actions が無効のリポジトリで、Actions 許可ポリシーを宣言した | Actions 許可ポリシーの期待値は Actions が有効であることを含む。有効にすることは管理設定の変更である | Actions の有効状態を宣言していないので、有効にできず適用不能 |
| Actions の有効状態を有効、Actions 許可ポリシーを `all` と宣言し、Actions が無効だった | 管理設定は Actions 許可ポリシーの一つで、変更予定も比較不能の一つ | 有効状態の不一致とポリシーの比較不能の、二つの変更予定になる |
| Actions の有効状態だけを有効と宣言し、Actions が無効だった | 不一致で適用可能。値を観測できなくても、無効にする前の値で再開することを承認の前に示す | ポリシーを観測できないので比較不能 |
| Actions が無効のリポジトリで、`sha_pinning_required` だけを宣言した | 比較不能で適用不能。無効のままでは更新のあとに期待状態を確かめられない | 値を送れば成功の応答が返るので適用可能。または、reco が Actions を有効にして適用する |
| Actions が無効のリポジトリで、Actions の有効状態を有効、`sha_pinning_required` を `true` と宣言した | 再開する設定は Actions 許可ポリシーと選択方式の Actions 許可条件。`sha_pinning_required` は同じ変更計画が宣言した値に変えるため挙げない | 残っていた値で再開するので、`sha_pinning_required` も挙げる |
| Actions の有効状態を無効と宣言し、Actions が有効で、Actions 許可ポリシーが `selected` だった | 不一致で適用可能。GitHub は選択方式の Actions 許可条件を保持するため、副作用はない | 無効にすると許可条件を取得できなくなるので、消失を副作用として示す |
| Actions が無効のリポジトリを観測し、応答の `sha_pinning_required` が `false` だった | 観測不能。無効のあいだの応答は保存された値を示さない | 応答にあるので `false` を観測値とする |
| Actions 許可ポリシーを `all` から `local_only` へ変えると宣言した | 副作用はない。失われる選択方式の Actions 許可条件が現在状態にない | ポリシーを変えるので、常に許可条件の消失を副作用として示す |
| Actions 許可ポリシーを `local_only` から `selected` へ変え、`github_owned_allowed: false` を宣言した。二つの更新操作のあいだ、GitHub 所有の Actions が許可される | 副作用ではない。論理的な設定変更が完了すれば残らない中間状態である。後続の更新操作が完了せずに残った場合は、変更結果が完了しなかったことを示す | 変更前も宣言した状態も許可しない Actions を許可するので、副作用として変更予定に添える |
| Actions が無効のリポジトリで、ワークフロー権限、保持日数か、公開範囲に合う `access_level`、承認ポリシー、実行条件の値が返った | 観測値。取得条件は設定ごとに異なる | Actions が無効なので観測不能とする |
| 非公開リポジトリで `access_level` を `organization` と観測し、`none` を宣言した | 不一致で適用可能 | `organization` は宣言できない値なので比較不能 |
| 非公開リポジトリで承認ポリシーを宣言した | 比較不能で適用不能。理由はリポジトリが非公開であること。同じ変更計画が Actions を有効にしても解消しない | 非公開リポジトリの Fork Pull Request ワークフローの実行条件の変更として扱う。または、更新を送って `422` を受けてから判断する |
| 公開リポジトリで承認ポリシーの変更を承認したあと、更新直前の観測でリポジトリが非公開になっていた | 関係する現在値の変化。承認ポリシーが観測不能になり、承認時に予定した値と同じとは確かめられない。変更結果は未実行 | 観測不能は比べられないので、変更前提は成立する |
| 非公開リポジトリで、実行条件の `run_workflows_from_fork_pull_requests` を `false`、`send_secrets_and_variables` を `true` と宣言した | 入力不正。GitHub は3項目を `false` で保存するため、期待状態が成り立たない | `204` が返るので受け付け、更新後の確認で `failed` にする |
| 実行条件で、`send_write_tokens_to_workflows` を `true` のまま、`require_approval_for_fork_pr_workflows` だけを変える | 変更予定の注記を添えない。書き込みトークンの送信は変わらない | 期待値が `true` なので注記を添える |
| 実行条件を `true` から `false` にし、3項目も `false` になる | 宣言の中の値の変化で、副作用でも再開する設定でもない | 3項目の値が失われるので、副作用として示す |
| 公開リポジトリで実行条件を宣言した | 比較不能で適用不能。理由はリポジトリが公開であること | 現在値が分からないので、送信を有効にする変更予定の注記を添える |
| 非公開リポジトリで、承認ポリシーの領域だけを読む観測 | 読み取り要求を送らずに観測不能（非公開）。実行条件の取得も送らない | 非公開なので、代わりに実行条件を読む |
| 実行条件の変更を承認したあと、更新直前の観測で、宣言が変えない `require_approval_for_fork_pr_workflows` だけが計画時と異なっていた | 関係する現在値の変化。4項目の組が一つの設定の値で、変更結果は未実行 | 変える項目は計画時のままなので、変更前提は成立する |
| 非公開リポジトリで実行条件の変更を承認したあと、更新直前の観測でリポジトリが公開になっていた | 関係する現在値の変化。実行条件が観測不能になり、変更結果は未実行。承認ポリシーの取得は送らない | 公開リポジトリなので、代わりに承認ポリシーを読む |
| Actions が無効の非公開リポジトリで、実行条件の変更を承認した | Actions の有効状態は関係する現在値ではなく、更新操作を開始する | Actions が無効なので開始しない |
| 宣言した値と、観測した Repository Variable の値が、末尾の改行だけ異なる | 異なる値であり不一致。変更予定は値の変更 | 空白の違いなので同じ値とみなし一致 |
| 宣言した `GO_VERSION` に対し、一覧が `go_version` を返した | 同じ変数として照合する。作成を計画しない | 綴りが違うので存在しないとみなし、作成を計画する |
| Actions が無効のリポジトリで Actions を有効にする変更が完了し、`sha_pinning_required` の更新直前の観測で値を観測できた | 先行する変更が取得条件を解消した、予定どおりの変化で、変更前提は成立する | 承認時に予定した値は観測不能なので、関係する現在値の変化 |
| 上と同じ状況で、`sha_pinning_required` の更新直前の観測で Actions が無効だった | 関係する現在値の変化。観測し直さず、変更結果は未実行 | 計画時と同じ無効のままなので、変更前提は成立する |
| Actions が無効のリポジトリを有効にする Actions 許可ポリシーの更新のあと、`sha_pinning_required` を `true` と観測した | 変更対象外の設定の予期しない変化ではない。更新直前は観測不能で、比べる観測値がない | 更新直前の応答の `false` と異なるので、予期しない変化 |
| Actions 許可ポリシーの更新のあと、Actions の有効状態が無効から有効に変わっていた | 予期しない変化ではない。Actions の有効状態は、その論理的な設定変更が変える設定である | 有効状態は宣言していない設定なので、予期しない変化 |
| `sha_pinning_required` を変える更新のあと、Actions の有効状態が有効から無効に変わっていた | 有効状態は変える設定ではなく、変更対象外の設定の予期しない変化。期待値も観測不能で、期待状態の確認結果は確認失敗 | 更新要求が有効状態を送るので、変える設定であり比べない |
| 選択方式の Actions 許可条件だけを更新する論理的な設定変更の前後で、`sha_pinning_required` の値が変わった | 比べない。同じ設定の領域に属するが、更新エンドポイントが異なる | 同じ設定の領域の設定なので比べる |
| Actions 許可ポリシーを `selected` にした更新操作のあと、二つ目の直前の観測で、選択方式の Actions 許可条件が宣言した値と異なっていた | 二つ目がすべて上書きする設定で、関係する現在値ではない。変更前提は成立する | 承認時に予定していない値なので、変更前提は不成立 |
| 二つの更新操作からなる論理的な設定変更で、`sha_pinning_required` が、一つ目の直前とは異なり、二つ目の直前と更新後では同じだった | 変更対象外の設定の予期しない変化。一つ目の更新エンドポイントの設定で、一つ目の直前の値と比べる | 二つ目の直前と更新後が同じなので、変化はない |

## References

- [設定ごとの設計](README.md)
- [ドメインモデル](../domain.md)
- [GitHub API 通信の設計](../github-access.md)
- [変更の実行の設計](../execution.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub Actions 権限 API 仕様](../../research/actions-permissions.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
- [GitHub Actions 変数 API 仕様](../../research/actions-variables.md)
