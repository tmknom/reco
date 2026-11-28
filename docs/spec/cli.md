# CLI 仕様

## 1. 目的と適用範囲

本ドキュメントは、reco のコマンド、オプション、認証情報の選択、対話承認、出力先、エラーの形、終了コードを定めます。対象は、利用者が reco を操作し、その結果を機械的に受け取るための外部契約です。`plan`、`apply`、`show` が表示する内容は [表示仕様](output.md) が定めます。設定の照合や更新を実現する内部構成・API 操作順序は規定しません。

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

`--help` (`-h`) は `reco` と各コマンドに、`--version` (`-v`) はコマンドを伴わない `reco` に指定でき、使い方またはバージョンを表示して終了します。コマンドを指定せずに `reco` だけを実行した場合も使い方を表示します。これらと `help`、`completion` コマンドの細かな振る舞いは、`--version` の出力を除いて本仕様で規定しません。`--version` は、ビルド時に埋め込んだバージョンだけを1行で表示し、埋め込まれていなければ `dev` を表示します。リリースの工程が、この出力をタグのバージョンと完全一致で照合するためです。

未定義のコマンド・オプション、不要な位置引数、値のないオプション、不正なオプション値は、処理の開始前にエラーとします。`--repo` は一回の実行に一つだけ指定できます。`show` には `--config` を指定できません。指定した設定ファイルが存在しない、または読み取れない場合もエラーとします。

### 2.3 リポジトリ指定と対象選択

`--repo` の値は、[YAML 設定仕様](configuration.md) の第14.2節で定めるリポジトリ指定です。

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

reco は、利用者が `gh auth token` で取得したトークンを `GH_TOKEN` に渡す使い方を主に想定し、必要な権限の確認と案内はこのトークンを前提にします（[GitHub REST API 共通仕様](../research/rest-api.md) 7章）。fine-grained personal access token など、ほかのトークンも同じ環境変数で渡せます。トークンの権限が足りず、GitHub が読み取り要求を拒んだときは観測操作の失敗としてエラー終了し、更新要求を拒んだときはその更新操作の失敗になります。権限の不足により GitHub が値を応答に含めない場合も、観測操作の失敗としてエラー終了します。

```text
$ GH_TOKEN=$(gh auth token) reco plan
```

このトークンの scope は利用者の権限を超えないため、操作できるかはトークン所有者の対象リポジトリでのロールで決まります。reco が動作を確認した条件は、トークン所有者が対象リポジトリの Admin ロールを持つことです。Write 以上のロールを持たないリポジトリでは、`show`・`plan`・`apply` は観測操作の失敗としてエラー終了します（[GitHub リポジトリ API 仕様](../research/repositories.md) 1章）。Write・Maintain ロールでの動作は確認していません。

Environment に存在しないことを宣言した `plan` と `apply` は、その Environment が存在すれば、宣言を受け付けない種類である Environment Secret の一覧も読みます。`apply` は、承認を得てその Environment を削除します。Secret の一覧の読み取りには、リポジトリの collaborator であることと、personal access token (classic) なら `repo` scope、fine-grained personal access token なら Environments の read の権限が要ります。Environment の削除には、personal access token (classic) なら `repo` scope、fine-grained personal access token なら Administration の write の権限が要ります（[GitHub Actions シークレット API 仕様](../research/actions-secrets.md) 1章、[GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) 1章）。Secret の一覧を読めないトークンでは、Environment の削除を宣言した `plan` と `apply` は観測操作の失敗でエラー終了し、Secret がないとは読みません。fine-grained personal access token での動作は確認していません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-003` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `grounding` | [GitHub REST API 共通仕様](../research/rest-api.md) | Repository File |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |
| `grounding` | [GitHub Actions シークレット API 仕様](../research/actions-secrets.md) | Repository File |
| `grounding` | [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md) | Repository File |

## 4. `apply` の承認と実行

`apply` は、選択された全対象について、設定宣言の検証、必要な観測、照合と適用可能性の判断が完了してから承認へ進みます。事前の検証で問題が見つかった場合は、対象と理由を示し、承認や更新を開始していないことを伝えてエラー終了します。すべて変更不要と確認できた場合は、承認を求めず、更新せずに正常終了します。

選択された対象のどこかに適用不能な変更予定があれば、変更計画を最後まで表示し、承認を求めず、どのリポジトリも更新せずにエラー終了します。適用可能な変更予定だけを持つリポジトリも更新しません。このときのエラーは、適用不能な変更予定のすべての対象と設定を挙げ、更新を開始していないことを伝えます。観測操作に失敗した場合は、その時点で打ち切るため変更計画を表示せず、ほかの対象に適用不能な変更予定があっても観測操作の失敗だけを報告します。

変更がある場合は、変更計画を提示したうえで、その全体に一度だけ明示的な対話承認を求めます。承認プロンプトは `Apply these changes? [y/N]: ` です。`y` または `yes` だけを承認として扱い、それ以外の入力、EOF、入力失敗、承認待ちの間の中断（`SIGINT`、`SIGTERM`）は承認しません。標準入力が対話端末でなければ、パイプなどから承認文字列が与えられても更新しません。この場合は変更計画を提示したうえで、承認を求めず、入力を読みません。

承認の入力は、標準入力の先頭1行だけを読み、行末の改行（LF）1つを除いた文字列を `y`・`yes` と比べます。大文字、前後の空白、CR を含む入力は承認しません。改行で終わらない入力は EOF として扱います。

承認が得られなければ更新を開始せず、理由と、更新を開始していないことを示すエラーで終了します。一部の更新後に停止した場合は、更新を開始していないとは示しません。

承認後は、承認した対象と変更内容を差し替えません。各更新の直前に変更前提を確認できない場合は、理由を報告して当該操作と後続の更新を開始しません。

複数のリポジトリに変更予定があるときは、設定宣言の `repositories` の並びで一つずつ更新し、あるリポジトリのすべての論理的な設定変更が完了したときだけ、次のリポジトリへ進みます。完了しなかった論理的な設定変更があれば、残りのリポジトリの更新は開始しません。自動ロールバックは行いません。更新中に中断（`SIGINT`、`SIGTERM`）を受けた場合は、次の更新操作を開始しません。一つのリポジトリの論理的な設定変更は、原則として変更計画の表示順に実行します。表示順と異なる実行順は、[設定ごとの設計](../design/settings/README.md) の各文書の「論理的な設定変更と更新操作」が定めます。

`apply` は、更新要求のあいだを空け、GitHub が更新を取得の結果に反映するまでの遅れに備えて確認し直すため、更新する変更予定や個別パターンの数が多いほど時間がかかります（[変更の実行の設計](../design/execution.md)）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-008` | Functional Requirement |
| `alignment` | `FR-009` | Functional Requirement |
| `alignment` | `FR-010` | Functional Requirement |
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `alignment` | `INV-002` | Invariant |
| `alignment` | `INV-003` | Invariant |
| `alignment` | `INV-004` | Invariant |
| `alignment` | `INV-005` | Invariant |

## 5. 出力先とエラー

観測した設定状態、変更計画、変更結果、`--help` と `--version` による使い方・バージョンは標準出力（stdout）へ表示します。エラー、警告、承認プロンプト、診断ログは標準エラー出力（stderr）へ表示します。変更予定の注記と変更結果の注記は、それぞれ変更計画と変更結果の一部として stdout へ表示します。

エラーは、診断ログと同じ JSON の1行で stderr へ表示します。`msg` は `command failed` とし、`error` に対象と理由を可能な範囲で示します。エラーは `--log-level` の値によらず表示します。`error` の文言は外部契約にしません。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"apply cancelled: approval declined; no changes made"}
```

`plan` は、適用不能な変更予定があっても変更計画を最後まで stdout へ表示し、そのあとで適用不能な設定を対象ごとに示すエラーを stderr へ表示します。`apply` で完了しなかった論理的な設定変更があるときは、変更結果のあとに、宣言順で最初に完了しなかった論理的な設定変更のエラーを stderr へ表示します。更新操作の成否が不明になった原因は、このエラーで示します。リポジトリリソースの更新のエラーは、変更結果と同じくリポジトリリソースの名前だけを示し、値と更新要求の本文を示しません（[表示仕様](output.md) 4.2節）。

HTTP リダイレクトには追従しません。読み取り要求では観測操作の失敗、更新要求では成否不明として、対象と HTTP ステータスを示します。`Location` などに含まれる秘密情報は出力しません。更新成功後の再観測がリダイレクトで失敗した場合も、更新の成功と観測の失敗を区別します。

GitHub は、名前変更前の名前で指定したリポジトリへの要求にリダイレクト応答を返します（[GitHub リポジトリ API 仕様](../research/repositories.md) 2章）。このため、リダイレクト応答のエラーでは、リポジトリが名前変更または移転された可能性と、現在の名前で指定し直すことを案内します。転送先の名前は示しません。ただし、更新エンドポイントが名前変更以外の理由でリダイレクト応答を返すと設定ごとの設計が定めている場合は、名前変更の案内ではなく、その理由を示します。この場合も成否不明であり、成功や変更なしとは扱いません。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/old-name: redirect not followed (HTTP 301): the repository may have been renamed or transferred; specify its current name"}
```

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-013` | Functional Requirement |
| `alignment` | `QR-001` | Quality Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../research/repositories.md) | Repository File |

## 6. 終了コード

| 状況 | 終了コード |
|---|---:|
| 使い方・バージョンの表示 (`--help`、`--version`、コマンドを伴わない `reco`) | `0` |
| `plan`：変更不要、または適用可能な変更予定（比較不能な設定予定を含む） | `0` |
| `apply`：変更完了、または変更不要 | `0` |
| `show`：観測成功（観測不能な値を含み得る） | `0` |
| `plan`：適用不能な変更・観測操作の失敗 | `1` |
| `apply`：適用不能な変更・承認拒否・承認不能・変更失敗・更新操作の成否不明・変更前提の不成立 | `1` |
| 入力不正、認証・権限・API 通信の失敗、対象の不一致や確認不能 | `1` |
| GitHub API の HTTP リダイレクト応答（読み取り・更新） | `1` |
| 更新成功応答後の再観測失敗・期待状態との不一致・変更対象外の設定の予期しない変化 | `1` |

複数のリポジトリを選択した `apply` は、すべてのリポジトリが変更完了または変更不要のときに限り `0` を返します。

`plan` の終了コード `0` は、差分がないことも、後の `apply` が成功することも保証しません。`apply` の変更失敗時には、既に実行された操作の影響が GitHub 上に残る可能性があります。終了コード `1` だけから、変更が一切行われなかったとは判断できません。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `FR-007` | Functional Requirement |
| `alignment` | `FR-013` | Functional Requirement |

## 7. References

- [表示仕様](output.md)
- [要求定義](requirements.md)
- [YAML 設定仕様](configuration.md)
- [変更の実行の設計](../design/execution.md)
- [設定ごとの設計](../design/settings/README.md)
- [GitHub REST API 共通仕様](../research/rest-api.md)
- [GitHub リポジトリ API 仕様](../research/repositories.md)
