# CLI 仕様

## 1. 目的と適用範囲

本ドキュメントは、reco のコマンド、オプション、認証情報の選択、対話承認、表示内容、終了コードを定めます。対象は利用者から観察できる操作とその結果です。設定の照合や更新を実現する内部構成・API 操作順序は規定しません。

reco は GitHub.com の個人アカウント所有リポジトリを対象とします。利用者が対象を指定し、必要な権限を持つ認証情報を用意します。所有者と認証主体の一致検査は行いません。

## 2. コマンドとオプション

### 2.1 コマンド

reco は次のコマンドを提供します。

```text
reco [--log-level LEVEL] plan  [--config PATH] [--repo OWNER/REPO]
reco [--log-level LEVEL] apply [--config PATH] [--repo OWNER/REPO]
reco [--log-level LEVEL] show  --repo OWNER/REPO
```

| コマンド | 利用者が行うこと | 設定ファイル | GitHub 設定の更新 |
|---|---|---|---|
| `plan` | 期待状態との差、変更予定、適用可否を確認 | 使用 | しない |
| `apply` | 変更予定を確認・承認し、実行結果を確認 | 使用 | 承認後に実施 |
| `show` | 指定リポジトリの現在の設定を確認 | 不使用 | しない |

`apply` は `plan` の結果を引き継がず、実行時の設定宣言と観測結果から変更計画を作成します。

### 2.2 オプション

| オプション | 使用可能なコマンド | 省略時・指定条件 |
|---|---|---|
| `--config PATH` | `plan`、`apply` | `./reco.yaml`。相対パスは実行時の作業ディレクトリを基準に解釈 |
| `--repo OWNER/REPO` | 全コマンド | `plan` / `apply` は省略時に宣言された全リポジトリを選択。`show` では必須 |
| `--log-level LEVEL` | 全コマンド | `info`。`debug`、`info`、`warn`、`error` など、Go の `log/slog` が解釈できる値 |

`--log-level` はコマンド名の前後に指定できます。ログレベルは診断ログの出力を制御するものであり、コマンドの結果、承認に必要な情報、エラー理由を省略するためには使用しません。

`--help` (`-h`) は `reco` と各コマンドに、`--version` (`-v`) はコマンドを伴わない `reco` に指定でき、使い方またはバージョンを表示して終了します。コマンドを指定せずに `reco` だけを実行した場合も使い方を表示します。これらと `help`、`completion` コマンドは [cobra](https://github.com/spf13/cobra) の標準機能として提供し、細かな振る舞いは本仕様で規定しません。

未定義のコマンド・オプション、不要な位置引数、値のないオプション、不正なオプション値は、処理の開始前にエラーとします。`--repo` は一回の実行に一つだけ指定できます。`show` には `--config` を指定できません。指定した設定ファイルが存在しない、または読み取れない場合もエラーとします。

### 2.3 リポジトリ指定と対象選択

`--repo` の値は、[YAML 設定仕様](configuration.md) の第6.2節で定めるリポジトリ指定です。

`plan` / `apply` の `--repo` には、設定ファイルで宣言したリポジトリだけを指定できます。設定宣言全体は対象選択の有無にかかわらず検証します。

選択されなかったリポジトリについては、GitHub 上での実在性を確認しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-002` | Functional Requirement |

## 3. 認証情報

| 環境変数 | 選択規則 |
|---|---|
| `GH_TOKEN` | 空でなければ優先して使用 |
| `GITHUB_TOKEN` | `GH_TOKEN` が未設定または空の場合に使用 |

両方が未設定または空の場合は、GitHub API へのアクセス前にエラーとします。選択したトークンで認証に失敗しても、もう一方へ切り替えません。トークン用のコマンドラインオプションは設けず、`gh` コマンドや資格情報ストアからの取得、トークンの自動生成・保存も行いません。

認証トークン・認証ヘッダーの秘密情報は、通常出力、エラー、ログに表示・記録しません。外部 API の応答を診断に使う場合も秘密情報をそのまま出力しません。エラー表示では `401`、`403`、`404` を区別して示し、`404` だけからリポジトリの不存在を断定しません。`403` または `429` がレート制限によるものと判定できる場合は、権限不足ではなくレート制限として示します。

環境変数の優先順位は [GitHub CLI の環境変数仕様](https://cli.github.com/manual/gh_help_environment) と同じですが、reco が GitHub CLI の認証機構を利用することは意味しません。

reco は、利用者が `gh auth token` で取得したトークンを `GH_TOKEN` に渡す使い方を主に想定し、必要な権限の確認と案内はこのトークンを前提にします（[GitHub REST API 共通仕様](../research/rest-api.md) 7章、[GitHub リポジトリ API 仕様](../research/repositories.md) 1章、[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 1章）。fine-grained personal access token など、ほかのトークンも同じ環境変数で渡せます。

```text
$ GH_TOKEN=$(gh auth token) reco plan
```

このトークンの scope は利用者の権限を超えないため、操作できるかはトークン所有者の対象リポジトリでのロールで決まります。reco が動作を確認した条件は、トークン所有者が対象リポジトリの Admin ロールを持つことです。Write 以上のロールを持たないリポジトリでは、GitHub が General の値を応答に含めないため、`show`・`plan`・`apply` は観測操作の失敗としてエラー終了します。Write・Maintain ロールでの動作は確認していません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-003` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Internal Document |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Internal Document |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Internal Document |

## 4. `plan`：変更計画の確認

`plan` は設定宣言全体を検証し、選択したリポジトリの観測結果と期待状態を照合して、変更計画を表示します。GitHub の設定は更新しません。

| 表示区分 | 意味 | 利用者に示す情報 |
|---|---|---|
| 確認済みの差分 | 比較可能な現在値と期待値が異なる | リポジトリ、設定、現在値、期待値 |
| 比較不能な設定予定 | GitHub の取得条件により現在値を比較できず、期待状態の適用を検討する | リポジトリ、設定、期待値、比較できない理由 |
| 適用不能 | 安全な変更に必要な条件を確認できない | 対象と理由。差分・比較不能の区分とは別に適用不能と示す |
| 変更不要 | 管理範囲内の設定が期待状態と一致する | 確認した対象の件数 |

照合結果と適用可能性は別に示します。不一致（Mismatch）でも適用不能な場合があり、比較不能（Not Comparable）でも条件を確認できれば適用可能な変更予定となります。観測操作そのものの失敗は比較不能に含めず、エラーとして報告します。

集計では、変更予定のあるリポジトリ数、確認済みの差分数、比較不能な設定予定数を区別します。変更予定のあるリポジトリ数は、そのすべてが適用可能であることを意味しません。確認済みの差分数と比較不能な設定予定数は管理設定の単位で数え、比較不能な項目を確認済みの差分数に加算しません。API 更新操作の数や論理的な設定変更の数とも同一視しません。選択方式の Actions 許可条件（Selected actions の3項目）は一つの設定グループとして集計し、上位の Actions 許可ポリシーとは区別します。変更の必要がないと確認できた場合に限り `No changes.` と表示します。

一つのリポジトリに複数の変更予定があるときは、[YAML 設定仕様](configuration.md) の章と表の並びで表示します。設定宣言での記述順には従いません。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Plan: 1 repository to update, 1 known change, 0 not-comparable settings
```

```text
$ reco plan
No changes.
Repositories checked: 2
```

比較不能な設定予定は、現在値を推測せずに示します。Actions が無効のリポジトリでは `allowed_actions` の現在値を取得できませんが、宣言したポリシーで Actions を有効にする変更として適用できます（[YAML 設定仕様](configuration.md) 4章）。

```text
Repository: example/project-a
  actions.permissions.allowed_actions
    Current: unavailable (GitHub Actions is disabled)
    Desired: local_only (enables GitHub Actions)

Plan: 1 repository to update, 0 known changes, 1 not-comparable setting
```

安全な変更に必要な条件を確認できない場合は、照合結果に加えて `Applicability: not applicable (<理由>)` を示し、集計の後に `Plan cannot be applied.` と表示します。

宣言した変更に伴って GitHub が管理範囲外の設定を削除する場合は、変更予定に `Side effect:` として失われる設定と現在値を添えます。この変更は適用可能であり、利用者が承認すれば実行します。現在は、Actions 許可ポリシーを `selected` から `all` / `local_only` へ変え、既存の選択方式の Actions 許可条件が失われる場合が該当します（[アーキテクチャ設計](../design/architecture.md) 5.4）。Actions が無効のリポジトリで `all` / `local_only` を宣言した場合は、無効にする前のポリシーが `selected` であれば同じく失われますが、無効の間は値もポリシーも取得できないため、失われる可能性があることだけを示します。`Side effect:` は変更計画の一部として stdout へ表示し、`apply` では承認プロンプトの前に表示します。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: selected -> all
    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["example/*"])
```

`plan` の終了コード `0` は、差分がないことも、後の `apply` が成功することも保証しません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-005` | Functional Requirement |
| `alignment` | `FR-006` | Functional Requirement |
| `alignment` | `FR-007` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |

## 5. `apply`：承認と変更結果

`apply` は、選択された全対象について、設定宣言の検証、必要な観測、照合と適用可能性の判断が完了してから承認へ進みます。これらの事前検証で問題が見つかった場合は、対象と理由を示し、承認や更新を開始していないことを報告します。すべて変更不要と確認できた場合は、承認を求めず、更新せずに正常終了します。

すべて変更不要の場合の表示例は次のとおりです。

```text
$ reco apply
No changes.
Repositories checked: 2
```

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-008` | Functional Requirement |
| `alignment` | `FR-009` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `FR-012` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-002` | Invariant |
| `alignment` | `INV-003` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `alignment` | `INV-005` | Invariant |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Internal Document |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Internal Document |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Internal Document |

### 5.1 承認

変更予定を、確認済みの差分と比較不能な設定予定に分けて提示します。変更がある場合は明示的な対話承認を要求し、`y` または `yes` だけを承認として扱います。それ以外の入力、EOF、入力失敗、承認待ちの間の中断（`SIGINT`、`SIGTERM`）は承認しません。標準入力が対話端末でなければ、パイプなどから承認文字列が与えられても更新しません。この場合は変更予定を提示したうえで、承認を求めず、入力を読みません。

承認の入力は、標準入力の先頭1行だけを読み、行末の改行（LF）1つを除いた文字列を `y`・`yes` と比べます。大文字、前後の空白、CR を含む入力は承認しません。改行で終わらない入力は EOF として扱います。

```text
$ reco apply
Configuration: ./reco.yaml
Repository: example/project-a
  general.allow_auto_merge: false -> true

Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated

Apply complete.
Repositories updated: 1
```

承認が得られなければ更新を開始せず、理由を表示して終了コード `1` とします。拒否した場合は `Apply cancelled. No changes made.` のように、更新が開始されていないことを伝えます。一部の更新後に停止した場合は `No changes made.` と表示しません。

承認後は、承認した対象と変更内容を差し替えません。各更新直前にリポジトリ ID・関係する現在値・適用条件を確認できない場合や、関係する予定外の変化を検出した場合は、理由を報告して当該操作と後続の更新を開始しません。更新操作に未管理設定の既存値を含める必要がある場合も、その値を維持できると確認できなければ当該操作を開始しません。

更新は逐次実行します。論理的な設定変更の中では、先行する更新操作の成功を確認してから次の更新操作へ進み、別の論理的な設定変更へは、その期待状態を確認してから進みます。自動ロールバックは行いません。

一つのリポジトリに複数の変更予定があるときは、承認の前に提示する順も、論理的な設定変更を実行する順も、4章が定める変更予定の表示順に従います。

### 5.2 結果の表示区分

| 表示 | 意味 |
|---|---|
| `updated` | 必要な更新操作の成功と、論理的な設定変更全体の期待状態の一致を確認でき、変更対象外の設定の予期しない変化を検出しなかった |
| `failed` | 更新操作の明示的な拒否、更新要求の成否不明（HTTP リダイレクト応答を含む）、更新開始後の前提不成立、成功応答後の状態確認失敗・不一致、変更対象外の設定の予期しない変化などによって変更が完了しなかった。副作用がなかったことは意味しない |
| `not attempted` | 当該の論理的な設定変更について、更新操作を一度も開始していない。開始直前の前提不成立で開始できなかった場合と、先行する異常によって実行を止めた場合を含む |
| `no change` | 管理範囲内の設定が期待状態と一致することを確認でき、更新が不要だった |

結果は、リポジトリごとに次のとおり表示します。

- **すべての論理的な設定変更が完了した**：リポジトリの行に `updated` を表示する
- **論理的な設定変更が1件で、完了しなかった**：リポジトリの行に `failed` または `not attempted` と理由を表示する
- **論理的な設定変更が2件以上で、完了しなかったものを含む**：リポジトリの行に結果を表示せず、論理的な設定変更ごとに結果を実行順に表示し、完了しなかったものに理由を添える

いずれかの論理的な設定変更で更新操作を開始しており、完了しなかった論理的な設定変更があるときは、変更結果の末尾に注記（`Note:`）を添えます。開始した更新操作に成否不明があれば自動で再送していないことを、なければ自動ロールバックを行っていないことを示します。

選択方式の Actions 許可条件は、3項目を個別の変更として表示せず、一体の設定グループとして扱います。上位ポリシーと組み合わせて一つの論理的な設定変更となる場合も、その変更全体の結果を示します。

複数の API 更新操作のうち先行操作が成功し、後続操作を開始できなかった場合は、完了済み操作と未開始の操作を示し、変更全体を `failed` とします。更新を一度も開始していない変更は `not attempted` とし、変更前提の不成立と、先行する異常による停止のどちらに当たるかを区別して理由を示します。

`failed` には、更新操作の失敗、更新操作の成否不明、成功応答後の期待状態の確認失敗、成功応答後の期待状態との不一致、成功応答後の変更対象外の設定の予期しない変化のいずれに当たるかを添えて示します。先行操作の成功後に後続操作を開始できなかった場合は、その旨を示します。変更対象外の設定の予期しない変化では、観測した変化を示しますが、reco に起因するとは断定しません。変更対象外の設定には、同じリポジトリのほかの管理設定も含みます。更新要求の応答を確認できない場合は成否不明とし、その後の再観測で期待状態が一致しても、更新操作の成否が判明したことにはならず、`updated` に変更しません。この再観測で変更対象外の設定の変化を観測した場合も、更新操作の結果と分けて示します。成否不明になった原因は、理由の行では区別せず、エラーとして示します。

更新操作が成功したのに期待状態と一致しない場合、reco は原因を断定しません。設定に既知の原因の候補があれば、`Possible cause:` として可能性の形で添えます。`general.allow_auto_merge` では、GitHub Free の非公開リポジトリで自動マージを利用できず、GitHub が更新を反映しないまま成功を応答する場合があることを示します（[GitHub リポジトリ API 仕様](../research/repositories.md) 4章）。更新操作の成否が不明な場合は添えません。

### 5.3 異常時の表示例

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    actions.workflow.default_workflow_permissions: failed
      Update request: succeeded
      Desired state: not confirmed (read failed)
  example/project-b: not attempted

Error: unable to confirm the desired state for example/project-a
Note: Some changes may already have been applied. No automatic rollback was performed.
```

```text
Applying changes...
  example/project-a: failed
    Update request: unknown
    Desired state: matches (update outcome still unknown)
  example/project-b: not attempted

Error: unable to confirm an update result for example/project-a: redirect not followed (HTTP 307)
Note: No automatic retry was performed.
```

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: does not match
    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it

Note: Some changes may already have been applied. No automatic rollback was performed.
```

```text
Applying changes...
  example/project-a
    general.delete_branch_on_merge: failed
      Update request: succeeded
      Desired state: matches
      Other settings: general.allow_auto_merge changed false -> true (cause not determined)
    general.allow_auto_merge: not attempted
      Stopped: a preceding change did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

異常時の表示では、確認済みの操作結果、成否が不明な範囲、未実行の変更を区別します。

## 6. `show`：現在状態の表示

`show` は設定宣言を使用せず、指定リポジトリについて GitHub から確認できたサポート対象の設定を表示します。GitHub の取得条件によって確認できない値は、その理由とともに `unavailable` と示し、値を推測しません。認証・権限・通信による観測操作の失敗は、正常な観測結果ではなくエラーとします。GitHub の設定は変更しません。

```text
$ reco show --repo example/project-a
Repository: example/project-a

General:
  delete_branch_on_merge: true
  allow_auto_merge: false
Actions permissions:
  enabled: true
  allowed_actions: all
  selected_actions: unavailable (allowed_actions is not "selected")
Workflow permissions:
  default_workflow_permissions: read
  can_approve_pull_request_reviews: false
```

権限の不足により GitHub が値を応答に含めない場合は、取得条件による観測不能ではなく観測操作の失敗です（3章）。`unavailable` と表示せず、欠けた項目を示してエラーとします。

`enabled` は観測・表示する値ですが、設定宣言に書ける設定ではありません。`allowed_actions` を宣言したときに Actions が有効であることを期待状態に含める扱いは、[YAML 設定仕様](configuration.md) 4章が定めます。`show` の表示は人が読むためのものであり、設定宣言として再利用できる形式ではありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Internal Document |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Internal Document |

## 7. 出力と終了コード

観測した設定状態、変更計画、変更結果、`--help` と `--version` による使い方・バージョンは標準出力（stdout）へ表示します。エラー、警告、承認プロンプト、診断ログは標準エラー出力（stderr）へ表示します。エラー時は対象と理由を可能な範囲で示します。前掲の表示例は両ストリームをまとめた代表例であり、空白・改行・エラーメッセージの完全一致は契約としません。変更結果に添える注記（`Note:`）は変更結果の一部として stdout へ表示します。

| 状況 | 終了コード |
|---|---:|
| 使い方・バージョンの表示 (`--help`、`--version`、コマンドを伴わない `reco`) | `0` |
| `plan`：変更不要、または適用可能な変更予定（比較不能な設定予定を含む） | `0` |
| `apply`：変更完了、または変更不要 | `0` |
| `show`：観測成功（取得条件による観測不能な値を含み得る） | `0` |
| `plan`：適用不能な変更・観測操作の失敗 | `1` |
| `apply`：承認拒否・承認不能・変更失敗・更新操作の成否不明・変更前提の不成立 | `1` |
| 入力不正、認証・権限・API 通信の失敗、対象の不一致や確認不能 | `1` |
| GitHub API の HTTP リダイレクト応答（読み取り・更新） | `1` |
| 更新成功応答後の再観測失敗・期待状態との不一致・変更対象外の設定の予期しない変化 | `1` |

`apply` の変更失敗時には、既に実行された操作の影響が GitHub 上に残る可能性があります。終了コード `1` だけから、変更が一切行われなかったとは判断できません。

HTTP リダイレクトには追従しません。読み取り要求では観測失敗、更新要求では成否不明の `failed` として終了コード `1` を返します。対象と HTTP ステータスを示す一方、`Location` などに含まれる秘密情報は出力しません。更新成功後の再観測がリダイレクトで失敗した場合も、更新の成功と観測の失敗を区別します。

GitHub は、名前変更前の名前で指定したリポジトリへの要求にリダイレクト応答を返します（[GitHub リポジトリ API 仕様](../research/repositories.md) 2章）。このため、リダイレクト応答のエラーでは、リポジトリが名前変更または移転された可能性と、現在の名前で指定し直すことを案内します。転送先の名前は示しません。

```text
Error: fetch repository example/old-name: redirect not followed (HTTP 301): the repository may have been renamed or transferred; specify its current name
```

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Internal Document |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Internal Document |

## 8. References

- [要求定義](requirements.md)
- [YAML 設定仕様](configuration.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
