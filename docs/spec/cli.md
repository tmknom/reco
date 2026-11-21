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

`--help` (`-h`) は `reco` と各コマンドに、`--version` (`-v`) はコマンドを伴わない `reco` に指定でき、使い方またはバージョンを表示して終了します。コマンドを指定せずに `reco` だけを実行した場合も使い方を表示します。これらと `help`、`completion` コマンドは [cobra](https://github.com/spf13/cobra) の標準機能として提供し、`--version` の出力を除いて、細かな振る舞いは本仕様で規定しません。`--version` は、ビルド時に埋め込んだバージョンだけを1行で表示し、埋め込まれていなければ `dev` を表示します。リリースの工程が、この出力をタグのバージョンと完全一致で照合するためです。

未定義のコマンド・オプション、不要な位置引数、値のないオプション、不正なオプション値は、処理の開始前にエラーとします。`--repo` は一回の実行に一つだけ指定できます。`show` には `--config` を指定できません。指定した設定ファイルが存在しない、または読み取れない場合もエラーとします。

### 2.3 リポジトリ指定と対象選択

`--repo` の値は、[YAML 設定仕様](configuration.md) の第8.2節で定めるリポジトリ指定です。

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

reco は、利用者が `gh auth token` で取得したトークンを `GH_TOKEN` に渡す使い方を主に想定し、必要な権限の確認と案内はこのトークンを前提にします（[GitHub REST API 共通仕様](../research/rest-api.md) 7章、[GitHub リポジトリ API 仕様](../research/repositories.md) 1章、[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 1章、[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 1章、[GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) 1章）。fine-grained personal access token など、ほかのトークンも同じ環境変数で渡せます。`show` が読み取る Environment と個別パターンの一覧には、GitHub App の権限の分類で Actions の read に当たる権限が要ります。ほかのトークンがこの権限を持たず、GitHub が一覧の取得を拒んだときは、`show` は観測操作の失敗としてエラー終了します。

```text
$ GH_TOKEN=$(gh auth token) reco plan
```

このトークンの scope は利用者の権限を超えないため、操作できるかはトークン所有者の対象リポジトリでのロールで決まります。reco が動作を確認した条件は、トークン所有者が対象リポジトリの Admin ロールを持つことです。Write 以上のロールを持たないリポジトリでは、GitHub が General の値を応答に含めないため、`show`・`plan`・`apply` は観測操作の失敗としてエラー終了します。Write・Maintain ロールでの動作は確認していません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-003` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

## 4. `plan`：変更計画の確認

`plan` は設定宣言全体を検証し、選択したリポジトリの観測結果と期待状態を照合して、変更計画を表示します。GitHub の設定は更新しません。

| 表示区分 | 意味 | 利用者に示す情報 |
|---|---|---|
| 確認済みの差分 | 比較可能な現在値と期待値が異なる | リポジトリ、設定、現在値、期待値 |
| 比較不能な設定予定 | GitHub の取得条件により現在値を比較できず、期待状態の適用を検討する | リポジトリ、設定、取得条件から分かるリポジトリの状態、期待値 |
| 適用不能 | 安全な変更に必要な条件を確認できない | 対象と理由。差分・比較不能の区分とは別に適用不能と示す |
| 変更不要 | 管理範囲内の設定が期待状態と一致する | 確認した対象の件数 |

照合結果と適用可能性は別に示します。不一致（Mismatch）でも適用不能な場合があり、比較不能（Not Comparable）でも条件を確認できれば適用可能な変更予定となります。観測操作そのものの失敗は比較不能に含めず、エラーとして報告します。

集計では、変更予定のあるリポジトリ数と変更予定数を示します。変更予定のあるリポジトリ数は、そのすべてが適用可能であることを意味しません。変更予定数は、確認済みの差分と比較不能な設定予定を合わせて管理設定の単位で数えます。二つを区別するのは変更予定の行の現在値であり、集計では区別しません。比較不能な設定予定も、宣言した値へ変える変更として利用者が承認するものだからです。API 更新操作の数や論理的な設定変更の数とは同一視しません。選択方式の Actions 許可条件（Selected actions の3項目）は一つの設定グループとして集計し、上位の Actions 許可ポリシーとは区別します。変更の必要がないと確認できた場合に限り `No changes.` と表示します。

一つのリポジトリに複数の変更予定があるときは、[YAML 設定仕様](configuration.md) の章と表の並びで表示します。設定宣言での記述順には従いません。

関連リソースの変更予定は、YAML 設定仕様の章の並びのあと、同じ章の中では設定宣言でその関連リソースを書いた順に表示します。名前の並びは YAML 設定仕様の表から決まらないためです。

関連リソースの作成と削除は、確認済みの差分として示し、集計の変更予定数に数えます。存在しないと確認できたことは、比較できる事実だからです。作成と削除は、設定の値の変更と見分けられる形で示し、削除では失われる値を示します。宣言に名前のない関連リソースは、変更計画に示しません。

関連リソースの作成は、現在値の位置に `absent`、期待値の位置に `present` を置いて示します。`absent` は確かめた事実であり、比較不能の印である括弧には入れません。関連リソースとその中の設定は、`<種類のキー>.<名前>` と `<種類のキー>.<名前>.<設定>` の形で示し、名前は設定宣言に書いた綴りのまま示します。GitHub が大文字小文字の違う綴りで返しても、設定ファイルの行と対応づけられるようにするためです。

Environment では、存在しない Environment の作成と、その Environment の `deployment_branch_policy` を、別の変更予定として示し、集計でもそれぞれ数えます。作成する Environment の `deployment_branch_policy` は、属する先が存在しないため比較不能な設定予定であり、現在値の位置に `(environment absent)` を示します。`deployment_branch_policy` の値は、方式が `selected` なら個別パターンを含めて `selected [<種類> "<名前>", ...]` の形で示します。現在値は GitHub が返した順、期待値は設定宣言の順に並べ、個別パターンのない `selected` は `selected []` と示します。現在値か期待値のどちらかが `selected` の変更予定は、個別パターンを並べた値が長くなるため、設定の行の下に現在値（比較不能なら現在の状態）と、`->` に続けて期待値を1行ずつ示します。どちらも `selected` でない変更予定は1行で示します。`selected` から変えると GitHub は個別パターンを残しませんが、個別パターンは同じ管理設定の値の一部であり、現在値に示しているため、`Side effect:` は添えません。

現在値と期待値がどちらも `selected` の確認済みの差分には、`->` の行の下に、なくなるパターンを `Patterns to remove:`、加わるパターンを `Patterns to add:` の行で、この順に添えます。どちらも `[<種類> "<名前>", ...]` の形で、なくなるパターンは現在値の順、加わるパターンは期待値の順に並べ、該当するパターンがない行は示しません。二つの一覧の並びが違い、利用者が差を目で突き合わせなくて済むようにするためです。方式が変わる変更予定には添えません。現在値か期待値の一覧の全体が、そのままなくなるか加わるためです。

```text
Repository: example/project-a
  environments.production: absent -> present
  environments.production.deployment_branch_policy: (environment absent) -> protected_branches
  environments.staging.deployment_branch_policy:
    selected [branch "main", tag "v*"]
    -> all
  environments.preview.deployment_branch_policy:
    selected [branch "feature/*", branch "main"]
    -> selected [branch "main", branch "release/*"]
    Patterns to remove: [branch "feature/*"]
    Patterns to add: [branch "release/*"]

Plan: 1 repository to update, 4 changes
Repositories checked: 1
```

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

選択方式の Actions 許可条件の確認済みの差分は、3項目を並べた値が長いため、設定の行の下に現在値と、`->` に続けて期待値を1行ずつ示します。

```text
Repository: example/project-a
  actions.permissions.selected_actions:
    github_owned_allowed: true, verified_allowed: false, patterns_allowed: []
    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["actions/checkout@v7", "example/*"]

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

```text
$ reco plan
No changes.
Repositories checked: 2
```

変更予定のあるリポジトリは、設定宣言の `repositories` の並びで、リポジトリごとに `Repository:` の節として示し、節のあいだを空行で区切ります。変更不要のリポジトリは節を持ちません。集計の次の行の `Repositories checked:` は、変更予定の有無によらず、選択したリポジトリの数を示します。次の例は、3件を宣言して `--repo` を省略し、`example/project-b` が変更不要だった場合です。

```text
$ reco plan
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Repository: example/project-c
  actions.workflow.default_workflow_permissions: write -> read

Plan: 2 repositories to update, 2 changes
Repositories checked: 3
```

Actions の有効状態は、Actions permissions を取得できれば必ず確認できるため、`enabled` の変更予定は確認済みの差分として示します。Actions を無効にしても、GitHub は `allowed_actions`、`sha_pinning_required`、Selected actions の値を残すため（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 3.2・4章）、`Side effect:` は添えません。`enabled: true` を `allowed_actions` とともに宣言した場合は、有効状態の行を示さず、表示と集計は `enabled` を省略した場合と同じです（[YAML 設定仕様](configuration.md) 4章）。

```text
Repository: example/project-a
  actions.permissions.enabled: true -> false

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

Actions を無効から有効にする変更予定には、再開する設定（[ドメインモデル](../design/domain.md) の照合結果と適用可能性）を `Previous settings:` の行で添えます。GitHub は無効にする前の `allowed_actions`、`sha_pinning_required`、Selected actions の値を残しており、有効に戻すとそれらの値で Actions を実行できるようになるためです（GitHub Actions 権限 API 仕様 3.2・4章）。`enabled` を `false` から `true` に変える変更予定では、この3つを挙げます。Actions が無効のリポジトリで `allowed_actions` を宣言した変更予定（`(enables GitHub Actions)`）では、`sha_pinning_required` だけを挙げます。ポリシーは宣言した値になり、Selected actions は宣言した値になるか、失われ得ることを `Side effect:` で示すためです。同じリポジトリで宣言している設定は挙げず、挙げる設定が残らなければ行を添えません。再開のあとで、同じ変更計画が宣言した値に変えるためです。無効のあいだは値を取得できないため、値は示さず、残っていた値で再開することと、値を示せない理由を示します。値が失われるわけではないため、`Side effect:` とは別の行で示します。再開するときのポリシーを利用者が決めるには、`allowed_actions` を宣言します（[YAML 設定仕様](configuration.md) 4章）。`Previous settings:` は `Side effect:` の前に示し、`apply` では承認プロンプトの前に表示します。

```text
Repository: example/project-a
  actions.permissions.enabled: false -> true
    Previous settings: GitHub Actions resumes with allowed_actions, selected_actions, and sha_pinning_required kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

比較不能な設定予定は、確認済みの差分と同じ `<設定>: <現在> -> <期待値>` の形で示し、現在値の位置に、GitHub が値を返さない理由から分かるリポジトリの状態を括弧で示します。

| 値を返さない理由 | 示す状態 | 意味 |
|---|---|---|
| Actions が無効 | `(GitHub Actions disabled)` | Actions が動いておらず、この設定は効いていない |
| Actions 許可ポリシーが `selected` でない（選択方式の Actions 許可条件だけに当たる） | `(not in use)` | 選択方式の Actions 許可条件が使われていない |
| リポジトリが公開（`access_level` だけに当たる） | `(public repository)` | この設定は公開リポジトリに適用されない |
| 属する先の Environment が存在しない（Environment の中の設定に当たる） | `(environment absent)` | Environment がないため、この設定もない |
| 宣言した名前と同じかどうかを判定できない名前の Environment がある（Environment とその中の設定に当たる） | `(existence unknown)` | その名前の Environment が存在するかを確かめられない |

括弧の状態は、観測したリポジトリの状態から決まり、推測した値ではありません。GitHub が保存している値を示すものでもありません。利用者が知りたいのは承認すると何が起きるかであり、その判断には、いま効いている状態と宣言した値があれば足ります。確認済みの差分とは、現在値が括弧で示されることで区別します。

Actions が無効のリポジトリでは `allowed_actions` の現在値を取得できませんが、宣言したポリシーで Actions を有効にする変更として適用できます（[YAML 設定仕様](configuration.md) 4章）。この変更予定には、期待値に `(enables GitHub Actions)` を添えます。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)
    Previous settings: GitHub Actions resumes with sha_pinning_required kept from before it was disabled (reco cannot read it while GitHub Actions is disabled)
    Side effect: GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (reco cannot read them while GitHub Actions is disabled)

Plan: 1 repository to update, 1 change
Repositories checked: 1
```

`(enables GitHub Actions)` は、Actions が無効のリポジトリの `allowed_actions` にだけ添えます。選択方式の Actions 許可条件は、`allowed_actions` が `selected` でないリポジトリでは現在値を取得できないため、`allowed_actions` を `selected` へ変える変更予定と並べて、比較不能な設定予定として示します。選択方式の Actions 許可条件の比較不能な設定予定も、確認済みの差分と同じく、設定の行の下に現在の状態と、`->` に続けて期待値を1行ずつ示します。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: all -> selected
  actions.permissions.selected_actions:
    (not in use)
    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["example/*"]

Plan: 1 repository to update, 2 changes
Repositories checked: 1
```

安全な変更に必要な条件を確認できない変更予定には、変更予定の行のすぐ下に `Applicability: not applicable (<理由>)` を添えます。理由は、セミコロンの前に何が成り立っていないかを、後ろに設定ファイルの直し方を示します。適用不能な変更予定が一つでもあれば、`Repositories checked:` の後に空行を挟んで `Plan cannot be applied.` を一度だけ表示します。集計は、適用不能な変更予定と、適用不能な変更予定だけを持つリポジトリも数えます。現在は、Actions が無効のリポジトリで、`enabled: true` も `allowed_actions` も宣言せずに `sha_pinning_required` を宣言した場合と、公開リポジトリで `access_level` を宣言した場合と、宣言した名前と同じかどうかを判定できない名前の Environment があり、宣言した名前の Environment が存在するかを確かめられない場合が該当します（[アーキテクチャ設計](../design/architecture.md) 5.4）。

```text
Repository: example/project-a
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true
    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)

Plan: 1 repository to update, 1 change
Repositories checked: 1

Plan cannot be applied.
```

同じリポジトリで `enabled: true` か `allowed_actions` を宣言していれば、`sha_pinning_required` の変更予定は比較不能でも適用可能で、`Applicability:` の行を添えません。

```text
Repository: example/project-a
  actions.permissions.enabled: false -> true
    Previous settings: GitHub Actions resumes with allowed_actions and selected_actions kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true

Plan: 1 repository to update, 2 changes
Repositories checked: 1
```

公開リポジトリの `access_level` は、ほかの宣言によらず適用不能です（[アーキテクチャ設計](../design/architecture.md) 5.4）。直し方は、その宣言を設定ファイルから消すことです。

```text
Repository: example/project-a
  actions.access.access_level: (public repository) -> none
    Applicability: not applicable (the repository is public; remove access_level, which applies only to private repositories)

Plan: 1 repository to update, 1 change
Repositories checked: 1

Plan cannot be applied.
```

宣言した Environment が存在するかを確かめられない場合（[アーキテクチャ設計](../design/architecture.md) 5.4）、作成すると既存の Environment を変え得るため、その Environment とその中の設定の変更予定は適用不能です。直し方は、その Environment の宣言を設定ファイルから消すことです。

```text
Repository: example/project-a
  environments.cafe: (existence unknown) -> present
    Applicability: not applicable (an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration)

Plan: 1 repository to update, 1 change
Repositories checked: 1

Plan cannot be applied.
```

宣言した変更に伴って GitHub が管理範囲外の設定を削除する場合は、変更予定に `Side effect:` として失われる設定と現在値を添えます。この変更は適用可能であり、利用者が承認すれば実行します。現在は、Actions 許可ポリシーを `selected` から `all` / `local_only` へ変え、既存の選択方式の Actions 許可条件が失われる場合が該当します（[アーキテクチャ設計](../design/architecture.md) 5.4）。Actions が無効のリポジトリで `all` / `local_only` を宣言した場合は、無効にする前のポリシーが `selected` であれば同じく失われますが、無効の間は値もポリシーも取得できないため、失われる可能性があることと、値を示せない理由を示します。遷移先が `selected` の変更には添えません。3項目は宣言した値で上書きされ、管理範囲内の変更になるためです。`Side effect:` は、副作用を伴う変更予定の最後の行として示します。変更計画の一部として stdout へ表示し、`apply` では承認プロンプトの前に表示します。

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

選択された対象のどこかに適用不能な変更予定があれば、変更計画を4章の `plan` と同じ形で stdout へ最後まで表示し、承認を求めず、どのリポジトリも更新せずにエラー終了します。適用可能な変更予定だけを持つリポジトリも更新しません。理由は、`plan` と同じく変更予定の下の `Applicability:` の行で示します。stderr のエラーは、適用不能な変更予定のすべてを変更計画の並びで対象と設定を挙げ、`no changes made` を添えて、更新を開始していないことを伝えます。観測操作に失敗した場合は、その時点で打ち切るため変更計画を表示せず、ほかの対象に適用不能な変更予定があっても観測操作の失敗だけを報告します（FR-008）。

```text
$ reco apply
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Repository: example/project-b
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true
    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)

Repository: example/project-c
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true
    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)

Plan: 3 repositories to update, 3 changes
Repositories checked: 3

Plan cannot be applied.
```

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"plan cannot be applied: example/project-b: actions.permissions.sha_pinning_required is not applicable; example/project-c: actions.permissions.sha_pinning_required is not applicable; no changes made"}
```

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
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

### 5.1 承認

変更予定を、確認済みの差分と比較不能な設定予定を区別できる形（4章）で提示します。変更がある場合は明示的な対話承認を要求し、`y` または `yes` だけを承認として扱います。それ以外の入力、EOF、入力失敗、承認待ちの間の中断（`SIGINT`、`SIGTERM`）は承認しません。標準入力が対話端末でなければ、パイプなどから承認文字列が与えられても更新しません。この場合は変更予定を提示したうえで、承認を求めず、入力を読みません。

承認の入力は、標準入力の先頭1行だけを読み、行末の改行（LF）1つを除いた文字列を `y`・`yes` と比べます。大文字、前後の空白、CR を含む入力は承認しません。改行で終わらない入力は EOF として扱います。

```text
$ reco apply
Configuration: ./reco.yaml

Repository: example/project-a
  general.allow_auto_merge: false -> true

Repository: example/project-c
  actions.workflow.default_workflow_permissions: write -> read

Plan: 2 repositories to update, 2 changes
Repositories checked: 3

Apply these changes? [y/N]: y
Applying changes...
  example/project-a: updated
  example/project-b: no change
  example/project-c: updated

Apply complete.
Repositories updated: 2
```

承認の前には、変更計画を4章の `plan` と同じ形で提示します。承認は、提示した変更計画の全体に一度だけ求めます。

承認が得られなければ更新を開始せず、理由を表示して終了コード `1` とします。拒否した場合は `apply cancelled: approval declined; no changes made` のように、更新が開始されていないことを伝えます。一部の更新後に停止した場合は `no changes made` を示しません。

承認後は、承認した対象と変更内容を差し替えません。各更新直前にリポジトリ ID・関係する現在値・適用条件を確認できない場合や、関係する予定外の変化を検出した場合は、理由を報告して当該操作と後続の更新を開始しません。更新操作に未管理設定の既存値を含める必要がある場合も、その値を維持できると確認できなければ当該操作を開始しません。

更新は逐次実行します。論理的な設定変更の中では、先行する更新操作の成功を確認してから次の更新操作へ進み、別の論理的な設定変更へは、その期待状態を確認してから進みます。複数のリポジトリに変更予定があるときは、設定宣言の `repositories` の並びで一つずつ更新し、あるリポジトリのすべての論理的な設定変更が完了したときだけ、次のリポジトリへ進みます。完了しなかった論理的な設定変更があれば、残りのリポジトリには、更新要求も、更新直前の確認のための読み取り要求も送りません。変更不要のリポジトリには、変更計画を作るための観測のあと、要求を送りません。自動ロールバックは行いません。

更新要求のあいだは1秒以上空けます。そのため `apply` の所要時間は、2つ目以降の更新操作ごとに1秒以上延びます。別のリポジトリの更新要求とのあいだも同じです。`deployment_branch_policy` の個別パターンは、加えるパターンと取り除くパターンの一つごとに更新要求を送るため、パターンの数だけ所要時間が延びます。このあいだに中断（`SIGINT`、`SIGTERM`）を受けた場合は、次の更新操作を開始しません（[アーキテクチャ設計](../design/architecture.md) 6.2節）。

GitHub が更新を取得の結果に反映するまで時間がかかることがあるため、更新操作が成功したのに期待状態と一致しないとき、または期待値と比べる設定を GitHub の取得条件により確認できないときは、間隔を広げながら確認し直し、一致を確認できないまま合計約30秒が経てば `failed` とします（[アーキテクチャ設計](../design/architecture.md) 6.2節）。

一つの論理的な設定変更が複数の更新操作からなるときは、後続の更新操作の直前の確認にも同じ遅れが現れ得ます。先行する更新操作が成功したのに、その更新操作が変えた設定が更新前の値のままのときは、同じ間隔で確認し直します。合計約30秒が経っても変わらなければ、後続の更新操作を開始せずに `failed` とします。先行する更新操作の更新要求は再送しません。

一つのリポジトリに複数の変更予定があるときは、承認の前に提示する順も、論理的な設定変更を実行する順も、4章が定める変更予定の表示順に従います。`allowed_actions` と `sha_pinning_required` の両方に変更予定があるときも、別々の論理的な設定変更として一つずつ更新します。

### 5.2 結果の表示区分

| 表示 | 意味 |
|---|---|
| `updated` | 必要な更新操作の成功と、論理的な設定変更全体の期待状態の一致を確認でき、変更対象外の設定の予期しない変化を検出しなかった |
| `failed` | 更新操作の明示的な拒否、更新要求の成否不明（HTTP リダイレクト応答を含む）、更新開始後の前提不成立、成功応答後の状態確認失敗・不一致、変更対象外の設定の予期しない変化などによって変更が完了しなかった。副作用がなかったことは意味しない |
| `not attempted` | 当該の論理的な設定変更について、更新操作を一度も開始していない。開始直前の前提不成立で開始できなかった場合と、先行する異常によって実行を止めた場合を含む |
| `no change` | 管理範囲内の設定が期待状態と一致することを確認でき、更新が不要だった |

選択したすべてのリポジトリを、設定宣言の `repositories` の並びで示します。結果は、リポジトリごとに次のとおり表示します。

- **変更不要だった**：リポジトリの行に `no change` を表示する。先行するリポジトリが完了しなかった場合も同じである
- **すべての論理的な設定変更が完了した**：リポジトリの行に `updated` を表示する
- **論理的な設定変更が1件で、完了しなかった（先行するリポジトリが完了しなかった場合を除く）**：リポジトリの行に `failed` または `not attempted` と理由を表示する
- **論理的な設定変更が2件以上で、完了しなかったものを含む（先行するリポジトリが完了しなかった場合を除く）**：リポジトリの行に結果を表示せず、論理的な設定変更ごとに結果を実行順に表示し、完了しなかったものに理由を添える
- **先行するリポジトリが完了しなかったため、どの論理的な設定変更も開始しなかった**：論理的な設定変更の数によらず、リポジトリの行に `not attempted` と理由を表示する

すべてのリポジトリが `updated` または `no change` のときに限り、`Apply complete.` と、`updated` のリポジトリの数を `Repositories updated:` として示します。`no change` のリポジトリは数に含めません。

いずれかの論理的な設定変更で更新操作を開始しており、完了しなかった論理的な設定変更があるときは、変更結果の末尾に注記（`Note:`）を添えます。開始した更新操作に成否不明があれば自動で再送していないことを、なければ自動ロールバックを行っていないことを示します。複数の更新操作からなる論理的な設定変更では、すべての更新操作を判定に含めます。この判定は、選択したすべてのリポジトリの論理的な設定変更について行います。先に完了したリポジトリのあとで別のリポジトリが完了しなかった場合も、適用済みの変更が残るため注記を添えます。

選択方式の Actions 許可条件は、3項目を個別の変更として表示せず、一体の設定グループとして扱います。Actions 許可ポリシーを `selected` へ変える変更と組み合わせて一つの論理的な設定変更となる場合は、変更結果を管理設定ごとの行に分けず、その変更全体について一つ示します。承認の前に提示する変更予定は、管理設定ごとの行のままです。

論理的な設定変更ごとに結果を表示するときは、論理的な設定変更を、それが変える管理設定の名前で示します。二つの管理設定を変える論理的な設定変更は、更新操作の順に ` + ` でつないで示します（`actions.permissions.allowed_actions + actions.permissions.selected_actions`）。同じ管理設定を変える更新操作が複数あっても、その管理設定の名前は一度だけ示します。

Environment の作成も、完了すれば `updated` と示します。何を作成したかは、承認の前に提示した `absent -> present` が示します。`deployment_branch_policy` を宣言した Environment の作成は、一つの更新要求で Environment と方式を設定する一つの論理的な設定変更で、`environments.<名前> + environments.<名前>.deployment_branch_policy` と示します。方式を宣言していない作成は `environments.<名前>`、存在する Environment の方式の変更は `environments.<名前>.deployment_branch_policy` と示します。名前は設定宣言の綴りです。期待値の方式が `selected` の変更でなければ、更新操作は一つのため、`Update request` は1行です。

期待値の方式が `selected` の `deployment_branch_policy` の変更は、方式を `selected` にする更新要求（方式が変わる場合だけで、作成ではその作成の更新要求）、取り除く個別パターンごとの更新要求、加える個別パターンごとの更新要求を、この順に送る一つの論理的な設定変更です（順の理由は[アーキテクチャ設計](../design/architecture.md) 5.4節）。名前は、存在する Environment では `environments.<名前>.deployment_branch_policy`、作成と重なる場合は `environments.<名前> + environments.<名前>.deployment_branch_policy` です。

複数の更新操作からなる論理的な設定変更が、更新操作を開始したあとで完了しなかった場合は、`Update request` の行を実行順に示します。行は `Update request (<対象>): <結果>` の形で、結果は `succeeded`、`rejected`、`unknown`、開始しなかった更新操作を表す `not started` のいずれかです。`<対象>` は、その更新操作が変える管理設定です。ただし、個別パターンを取り除く更新操作は `patterns to remove [<パターン>, ...]`、加える更新操作は `patterns to add [<パターン>, ...]` と示し、パターンは `plan` と同じく `branch "main"` の形で書きます。個別パターンの更新操作のうち、向きと結果が同じで続けて並ぶものは、1行にまとめてパターンを実行順に並べます。そのため、この行は複数の更新要求を表すことがあります。先行する更新操作が成功し、後続の更新操作を開始できなかった場合は、完了済みの更新操作を `succeeded`、未開始の更新操作を `not started` と示し、変更全体を `failed` とします。`not started` は更新操作についての表示で、論理的な設定変更の `not attempted` とは区別します。

後続の更新操作を変更前提の不成立で開始できなかった場合は、その更新操作を含む `not started` の行の次に `Precondition:` の行で理由を示します。先行する更新操作が変えた設定が予定どおりの値でなかった場合は、`Precondition: <設定> is not the value expected after the preceding update` と示します。GitHub の反映が遅れている場合も、外部主体が値を変えた場合もこの表示になり、原因は断定しません。先行する更新操作が成功しなかったために開始しなかった場合は、理由の行を足しません。先行する更新操作が成功していれば、後続を開始できなかった場合も後続が失敗した場合も、更新後の確認を一度だけ行い、その結果と変更対象外の設定の予期しない変化を示します。この確認で期待状態との一致を確認しても、必要な更新操作の成功を確認していないため、`updated` に変更しません。最初の更新操作が拒否され、更新後の確認を行わなかった場合は `Desired state: not checked` と示します。

更新を一度も開始していない変更は `not attempted` とし、変更前提の不成立と、先行する異常による停止のどちらに当たるかを区別して理由を示します。先行する異常による停止は、同じリポジトリの先行する論理的な設定変更が完了しなかった場合（`Stopped: a preceding change did not complete`）と、先行するリポジトリが完了しなかった場合（`Stopped: a preceding repository did not complete`）を文言で区別します。

変更前提が、承認したあとに現在値が変わったことで成立しなかった場合は、変わった設定を示します（`Precondition: <設定> changed since approval`）。`Side effect:` で示した設定の値が承認の前に示した値から変わっていた場合は、その設定を示します。GitHub の取得条件により確認できなかった値を確認できるようになった場合と、その逆も、変化として示します。複数の設定が変わっていた場合は、論理的な設定変更が変える設定を優先して一つだけ示します。ただし、Actions が無効のリポジトリで、承認した変更が Actions を有効にしたあとに `sha_pinning_required` を確認できるようになったことは、予定どおりの変化で、示しません。`sha_pinning_required` の更新は `enabled: true` を送るため、その直前に Actions が無効だった場合は `Precondition: actions.permissions.enabled changed since approval` と示します。計画時に Actions が有効だった場合は、`sha_pinning_required` を確認できなくなったことを優先し、`actions.permissions.sha_pinning_required` を示します。`access_level` の更新の直前にリポジトリが公開になっていた場合も、`access_level` を確認できなくなったため、`Precondition: actions.access.access_level changed since approval` と示します。

Environment では、Environment そのものの変化を `Precondition: environments.<名前> changed since approval` と示します。作成の承認のあとに同じ名前（ASCII の英字の大文字小文字だけが違う名前を含む）の Environment が作られた場合、宣言した名前と同じかどうかを判定できない名前の Environment が現れて存在を確かめられなくなった場合、方式の変更の承認のあとに Environment がなくなった場合、同じ名前で作り直されて ID が変わった場合が当たります。先頭の更新操作の直前の確認で見つかった場合は、更新要求を送らずに `not attempted` になります。Environment が変わらず方式（`selected` の個別パターンを含む）だけが変わっていた場合は、`Precondition: environments.<名前>.deployment_branch_policy changed since approval` と示します。両方が変わっていた場合は Environment そのものを優先します。作り直された Environment の方式を承認時の値と比べても、承認した変更が当てはまるかは分からないためです。

個別パターンの更新操作のように後続の更新操作の直前に Environment そのものが変わっていた場合は、存在する Environment では、承認したときから変わったため、先頭の更新操作と同じく `Precondition: environments.<名前> changed since approval` と示します。同じ論理的な設定変更が作成した Environment では、`Precondition: environments.<名前> is not the environment expected after the preceding update` と示します。作成のあとに Environment がなくなった場合や、作成のあとの最初の確認で見た ID から変わった場合が当たります。先行する更新操作が成功しているため、どちらの場合も変更全体は `failed` です。後続の更新操作の直前に個別パターンの集合が予定どおりでなかった場合は、`Precondition: environments.<名前>.deployment_branch_policy is not the value expected after the preceding update` と示します。予定どおりの集合は、承認時に観測した集合に、先行する更新操作が取り除いたパターンと加えたパターンを反映したものです。方式を `selected` にした更新操作や作成のあとは、パターンが一つもないことを予定します。パターンの ID だけが違う場合（同じ種類と名前で作り直された場合）は、予定どおりとして扱います。

`failed` には、更新操作の失敗、更新操作の成否不明、成功応答後の期待状態の確認失敗、成功応答後の期待状態との不一致、成功応答後の変更対象外の設定の予期しない変化のいずれに当たるかを添えて示します。先行操作の成功後に後続操作を開始できなかった場合は、その旨を示します。変更対象外の設定の予期しない変化では、観測した変化を示しますが、reco に起因するとは断定しません。変更対象外の設定には、同じリポジトリのほかの管理設定も含みます。更新要求の応答を確認できない場合は成否不明とし、その後の再観測で期待状態が一致しても、更新操作の成否が判明したことにはならず、`updated` に変更しません。この再観測で変更対象外の設定の変化を観測した場合も、更新操作の結果と分けて示します。成否不明になった原因は、理由の行では区別せず、エラーとして示します。

再観測の失敗とリポジトリ ID の変化による期待状態の確認失敗は、`not confirmed` に確認できなかった理由を括弧で添えて示します（`not confirmed (read failed)`、`not confirmed (repository ID changed)`）。

期待値と比べる設定が GitHub の取得条件により観測不能だったことによる確認失敗は、`not in effect` に、宣言した設定が効いていない理由を括弧で添えて示します。理由は、Actions が無効なら `GitHub Actions is disabled`、Actions 許可ポリシーが `selected` でなければ `allowed_actions is not "selected"`、リポジトリが公開なら `the repository is public` です。`allowed_actions` を宣言できるのは Actions を有効にする場合に限られ、`sha_pinning_required` は Actions を無効にする宣言と同時には宣言できず、`selected_actions` を宣言できるのは Actions 許可ポリシーを `selected` と宣言した場合に限られます（[YAML 設定仕様](configuration.md) 4章）。`access_level` は公開リポジトリには適用されません（[YAML 設定仕様](configuration.md) 6章）。このため、この観測不能は、宣言した設定が効いていないことを意味します。値そのものは確認できていないため、`does not match` とは示しません。`not confirmed` とも示さないのは、reco が確認に失敗したと読ませず、利用者が確かめるべきリポジトリの状態を示すためです。

Environment の方式の変更のあとに Environment が存在しなかった場合は、`not in effect (the environment does not exist)` と示します。宣言した方式を持つ Environment がないためです。作成のあとに Environment が存在しなかった場合は、存在しないことを確認できているため、`does not match` と示します。作成と方式の変更のどちらでも、宣言した名前と同じかどうかを判定できない名前の Environment が現れ、存在を確かめられなかった場合は、reco が確認できなかったことを示す `not confirmed (environment existence unknown)` と示します。

変更対象外の設定は、管理設定と同じ形の名前で示します。Environment の保護設定は、`show` の項目名に Environment の名前を前置して `environments.<名前>.wait_timer`、`environments.<名前>.required_reviewers`、`environments.<名前>.can_admins_bypass` と示し、値も `show` と同じ表記にします（`required_reviewers` は `none`、または `reviewers: [User 583231, Team 42], prevent_self_review: true` の1行）。名前は設定宣言の綴りです。複数が変わっていれば、`show` の並びで1行ずつ示します。

更新操作が成功したのに期待状態と一致しない場合、reco は原因を断定しません。設定に既知の原因の候補があれば、`Possible cause:` として可能性の形で添えます。`general.allow_auto_merge` では、GitHub Free の非公開リポジトリで自動マージを利用できず、GitHub が更新を反映しないまま成功を応答する場合があることを示します（[GitHub リポジトリ API 仕様](../research/repositories.md) 4章）。更新操作の成否が不明な場合は添えません。

### 5.3 異常時の表示例

次の表示例は、stdout へ表示する変更結果です。完了しなかった論理的な設定変更があるときは、変更結果のあとに、宣言順で最初に完了しなかった論理的な設定変更のエラーを stderr へ1行で表示します（7章）。

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    actions.workflow.default_workflow_permissions: failed
      Update request: succeeded
      Desired state: not confirmed (read failed)
  example/project-b: not attempted
    Stopped: a preceding repository did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

この例で stderr へ表示するエラーは次のとおりです。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"change actions.workflow.default_workflow_permissions failed: fetch workflow permissions example/project-a: unexpected response (HTTP 502)"}
```

```text
Applying changes...
  example/project-a: failed
    Update request: unknown
    Desired state: matches (update outcome still unknown)
  example/project-b: not attempted
    Stopped: a preceding repository did not complete

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

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: not in effect (GitHub Actions is disabled)

Note: Some changes may already have been applied. No automatic rollback was performed.
```

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: matches
    Other settings: actions.permissions.sha_pinning_required changed false -> true (cause not determined)

Note: Some changes may already have been applied. No automatic rollback was performed.
```

```text
Applying changes...
  example/project-a: not attempted
    Precondition: actions.permissions.selected_actions changed since approval
```

二つの更新操作からなる論理的な設定変更で、先行する更新操作が成功し、後続の更新操作を開始できなかった場合は次のとおりです。

```text
Applying changes...
  example/project-a: failed
    Update request (actions.permissions.allowed_actions): succeeded
    Update request (actions.permissions.selected_actions): not started
    Precondition: actions.permissions.allowed_actions is not the value expected after the preceding update
    Desired state: does not match

Note: Some changes may already have been applied. No automatic rollback was performed.
```

先行する更新操作が成否不明の場合は、後続の更新操作を開始しません。

```text
Applying changes...
  example/project-a: failed
    Update request (actions.permissions.allowed_actions): unknown
    Update request (actions.permissions.selected_actions): not started
    Desired state: does not match

Note: No automatic retry was performed.
```

ほかの論理的な設定変更と並ぶときは、二つの管理設定をまとめた名前で示します。

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    actions.permissions.allowed_actions + actions.permissions.selected_actions: failed
      Update request (actions.permissions.allowed_actions): succeeded
      Update request (actions.permissions.selected_actions): rejected
      Desired state: does not match
    actions.workflow.default_workflow_permissions: not attempted
      Stopped: a preceding change did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

先行するリポジトリが完了しなかった場合は次のとおりです。

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: does not match
  example/project-b: no change
  example/project-c: not attempted
    Stopped: a preceding repository did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

先に完了したリポジトリのあとで、別のリポジトリの変更前提が成立しなかった場合は次のとおりです。

```text
Applying changes...
  example/project-a: updated
  example/project-b: not attempted
    Precondition: general.allow_auto_merge changed since approval
  example/project-c: not attempted
    Stopped: a preceding repository did not complete

Note: Some changes may already have been applied. No automatic rollback was performed.
```

Environment の作成の承認のあとに、同じ名前の Environment が作られていた場合は次のとおりです。

```text
Applying changes...
  example/project-a: not attempted
    Precondition: environments.production changed since approval
```

Environment の作成と、別の Environment の方式の変更が並び、方式の変更のあとに保護設定の変化を観測した場合は次のとおりです。

```text
Applying changes...
  example/project-a
    environments.production + environments.production.deployment_branch_policy: updated
    environments.staging.deployment_branch_policy: failed
      Update request: succeeded
      Desired state: matches
      Other settings: environments.staging.wait_timer changed 30 -> none (cause not determined)

Note: Some changes may already have been applied. No automatic rollback was performed.
```

Environment の方式を `selected` に変えたあと、個別パターンを加える途中で GitHub に拒否された場合は次のとおりです。

```text
Applying changes...
  example/project-a: failed
    Update request (environments.production.deployment_branch_policy): succeeded
    Update request (patterns to add [branch "main"]): succeeded
    Update request (patterns to add [branch "release/*"]): rejected
    Update request (patterns to add [tag "v*"]): not started
    Desired state: does not match

Note: Some changes may already have been applied. No automatic rollback was performed.
```

個別パターンを入れ替える変更で、パターンを取り除いたあとに外部主体が個別パターンを変えていた場合は次のとおりです。

```text
Applying changes...
  example/project-a: failed
    Update request (patterns to remove [branch "feature/*", branch "old/*"]): succeeded
    Update request (patterns to add [branch "release/*", tag "v*"]): not started
    Precondition: environments.production.deployment_branch_policy is not the value expected after the preceding update
    Desired state: does not match

Note: Some changes may already have been applied. No automatic rollback was performed.
```

存在しない Environment を `selected` と個別パターンで宣言し、作成のあとのパターンの追加が成否不明になった場合は次のとおりです。ほかの論理的な設定変更と並ぶため、名前で示します。

```text
Applying changes...
  example/project-a
    general.allow_auto_merge: updated
    environments.production + environments.production.deployment_branch_policy: failed
      Update request (environments.production): succeeded
      Update request (patterns to add [branch "main"]): unknown
      Update request (patterns to add [tag "v*"]): not started
      Desired state: does not match

Note: No automatic retry was performed.
```

異常時の表示では、確認済みの操作結果、成否が不明な範囲、未実行の変更を区別します。

## 6. `show`：現在状態の表示

`show` は設定宣言を使用せず、指定リポジトリについて GitHub から確認できた設定と関連リソースを表示します。認証・権限・通信による観測操作の失敗は、正常な観測結果ではなくエラーとします。GitHub の設定は変更しません。

GitHub の取得条件によって確認できない設定は、その取得条件が `show` のほかの行から分かるかどうかで、示し方を分けます。

- **ほかの行から分かる**：行ごと示さない。上位の設定によってその設定が効いていない状態（Actions が無効、Actions 許可ポリシーが `selected` でない）がこれに当たり、その状態は上位の設定の行で示されるためである
- **ほかの行から分からない**：行を省かず、値の位置に `unavailable (<理由>)` と示す。行を省くと、確認できなかった設定と reco が表示しない設定を区別できないためである。公開リポジトリの `access_level` がこれに当たる

```text
$ reco show --repo example/project-a
Repository: example/project-a

General:
  delete_branch_on_merge: true
  allow_auto_merge: false
Actions permissions:
  enabled: true
  allowed_actions: all
  sha_pinning_required: false
Workflow permissions:
  default_workflow_permissions: read
  can_approve_pull_request_reviews: false
Actions access:
  access_level: none
Environments: none
```

`allowed_actions` が `selected` のときは、`selected_actions` の3項目を字下げして、`sha_pinning_required` の前に示します。`patterns_allowed` は、パターンを二重引用符で囲んで並べます。空の集合は `[]` と示します。

```text
Actions permissions:
  enabled: true
  allowed_actions: selected
  selected_actions:
    github_owned_allowed: true
    verified_allowed: false
    patterns_allowed: ["actions/checkout@v7", "example/*"]
  sha_pinning_required: false
```

Actions が無効のリポジトリでは、`allowed_actions`、`selected_actions`、`sha_pinning_required` を取得できないため、`enabled` だけを示します。

```text
Actions permissions:
  enabled: false
```

`access_level` は非公開リポジトリにだけ適用される設定で、公開リポジトリでは取得できません（[GitHub Actions 権限 API 仕様](../research/actions-permissions.md) 6章）。Actions が無効のリポジトリでも、非公開であれば値を示します。

```text
Actions access:
  access_level: unavailable (the repository is public)
```

関連リソースは、種類ごとの節に、存在するものをすべて示します。並びは GitHub が一覧で返した順で、reco は並べ替えません。名前は二重引用符で囲み、制御文字をエスケープして示します。一つもないときは、節を省かず、節の見出しに `none` と示します。節を省くと、存在しないと確かめたことと、reco がその種類を表示しないことを区別できないためです。

Environment は `Environments:` の節に示します。Environment ごとに示す項目は次のとおりです。

- **`deployment_branch_policy`**：`mode` に、デプロイ元制限の方式（`all`、`protected_branches`、`selected`）を示す。`selected` のときに限り、`patterns` に個別パターンを `<種類> "<名前>"` の形で、GitHub が返した順に並べる。種類は `branch` か `tag` で、空の集合は `[]` と示す
- **`wait_timer`**：デプロイを待たせる分数。この保護ルールがなければ `none` と示す
- **`required_reviewers`**：この保護ルールがなければ `none` と示す。あれば、`reviewers` にレビュアーを `User <ID>` か `Team <ID>` の形で並べ、`prevent_self_review` を示す
- **`can_admins_bypass`**：管理者が保護ルールを迂回できるか

`wait_timer`、`required_reviewers`、`can_admins_bypass` は reco が宣言を受け付けない設定ですが、確認できた値を示します。

```text
Environments:
  - name: "production"
    deployment_branch_policy:
      mode: selected
      patterns: [branch "main", branch "release/*", tag "v*"]
    wait_timer: 30
    required_reviewers:
      reviewers: [User 583231, Team 42]
      prevent_self_review: true
    can_admins_bypass: false
  - name: "staging"
    deployment_branch_policy:
      mode: protected_branches
    wait_timer: none
    required_reviewers: none
    can_admins_bypass: true
```

権限の不足により GitHub が値を応答に含めない場合は、取得条件による観測不能ではなく観測操作の失敗です（3章）。行を省かず、欠けた項目を示してエラーとします。

`show` の表示は人が読むためのものであり、設定宣言として再利用できる形式ではありません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |
| `grounding` | [GitHub Actions 権限 API 仕様](../research/actions-permissions.md) | Repository File |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

## 7. 出力と終了コード

観測した設定状態、変更計画、変更結果、`--help` と `--version` による使い方・バージョンは標準出力（stdout）へ表示します。エラー、警告、承認プロンプト、診断ログは標準エラー出力（stderr）へ表示します。変更結果に添える注記（`Note:`）は変更結果の一部として stdout へ表示します。

エラーは、診断ログと同じ JSON の1行で stderr へ表示します。`msg` は `command failed` とし、`error` に対象と理由を可能な範囲で示します。エラーは `--log-level` の値によらず表示します。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"apply cancelled: approval declined; no changes made"}
```

前掲の表示例は代表例であり、空白・改行・エラーメッセージの完全一致は契約としません（`--version` の出力は除きます）。

| 状況 | 終了コード |
|---|---:|
| 使い方・バージョンの表示 (`--help`、`--version`、コマンドを伴わない `reco`) | `0` |
| `plan`：変更不要、または適用可能な変更予定（比較不能な設定予定を含む） | `0` |
| `apply`：変更完了、または変更不要 | `0` |
| `show`：観測成功（取得条件による観測不能な値を含み得る） | `0` |
| `plan`：適用不能な変更・観測操作の失敗 | `1` |
| `apply`：適用不能な変更・承認拒否・承認不能・変更失敗・更新操作の成否不明・変更前提の不成立 | `1` |
| 入力不正、認証・権限・API 通信の失敗、対象の不一致や確認不能 | `1` |
| GitHub API の HTTP リダイレクト応答（読み取り・更新） | `1` |
| 更新成功応答後の再観測失敗・期待状態との不一致・変更対象外の設定の予期しない変化 | `1` |

複数のリポジトリを選択した `apply` は、すべてのリポジトリが変更完了または変更不要のときに限り `0` を返します。

`plan` は、適用不能な変更予定があっても変更計画を最後まで stdout へ表示し、そのあとで適用不能な設定を対象ごとに示すエラーを stderr へ表示します。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"plan cannot be applied: example/project-a: actions.permissions.sha_pinning_required is not applicable"}
```

`apply` の変更失敗時には、既に実行された操作の影響が GitHub 上に残る可能性があります。終了コード `1` だけから、変更が一切行われなかったとは判断できません。

HTTP リダイレクトには追従しません。読み取り要求では観測失敗、更新要求では成否不明の `failed` として終了コード `1` を返します。対象と HTTP ステータスを示す一方、`Location` などに含まれる秘密情報は出力しません。更新成功後の再観測がリダイレクトで失敗した場合も、更新の成功と観測の失敗を区別します。

GitHub は、名前変更前の名前で指定したリポジトリへの要求にリダイレクト応答を返します（[GitHub リポジトリ API 仕様](../research/repositories.md) 2章）。このため、リダイレクト応答のエラーでは、リポジトリが名前変更または移転された可能性と、現在の名前で指定し直すことを案内します。転送先の名前は示しません。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/old-name: redirect not followed (HTTP 301): the repository may have been renamed or transferred; specify its current name"}
```

ただし、個別パターンを加える要求への `303` は、同じ種類と名前のパターンがすでにある場合に GitHub が返す応答です（[GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) 1章）。その直前の確認で同じリポジトリを読めているため、名前変更の案内ではなく、パターンがすでにある場合の応答であることを示します。この場合も成否不明の `failed` で、成功や変更なしとは扱いません。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"change environments.production.deployment_branch_policy failed: add deployment pattern branch \"main\" to environment \"production\" example/project-a: update outcome unknown: redirect not followed (HTTP 303): GitHub returns this when the pattern already exists"}
```

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |
| `grounding` | [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md) | Repository File |

## 8. References

- [要求定義](requirements.md)
- [YAML 設定仕様](configuration.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)
- [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md)
