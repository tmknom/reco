# reco の概要

reco（Repository Configurator）は、GitHub リポジトリの設定の欲しい状態を YAML の設定ファイルに書き、GitHub 上の現在の設定との差を示して、利用者の承認を得てから設定を変える CLI です。この文書は、reco を導入するかを決めるために、何を解決し、どの設定を扱え、何を約束するかを示します。用語の意味は [用語集](glossary.md) にあります。

## reco が解決する問題

GitHub の機能やセキュリティの設定は変わり続けます。多数のリポジトリを持つと、どのリポジトリでどの設定が有効になっているかを覚えておくことも、設定画面を一つずつ開いて確かめ続けることも難しくなり、直すべき設定を見落としやすくなります。

reco を使うと、個々のリポジトリの設定を覚えていなくても、設定ファイルを読めば欲しい状態が分かり、`plan` を実行すれば今の設定がそこからずれていないかを確かめられます。

## 基本の流れ

設定ファイルには、管理したいリポジトリと、そのリポジトリで管理したい設定の値だけを書きます。次の例は、一つのリポジトリで、マージ後の head ブランチの自動削除を有効にしたいという宣言です。

```yaml
repositories:
  - name: example/project-a
    general:
      delete_branch_on_merge: true
```

設定ファイルを書いてから適用するまでの流れは次のとおりです。

```mermaid
flowchart LR
    W[設定ファイルに欲しい状態を書く] --> P["plan：現在の設定との差（変更計画）を見る"]
    P --> A["apply：変更計画を見て承認する"]
    A --> U[reco が設定を変え、反映を確かめる]
```

- **`plan`**：設定ファイルと GitHub 上の現在の設定を比べ、変更計画を表示する。GitHub の設定は変えない
- **`apply`**：変更計画を作り直して表示し、承認を求める。承認されたら、設定ファイルに書いた順にリポジトリを更新し、更新後の設定が期待値になったことを確かめて結果を示す
- **`show`**：設定ファイルを使わずに、一つのリポジトリの現在の設定を表示する。設定ファイルを書く前に今の値を確かめるときに使う

コマンドとオプションの詳細は [コマンドリファレンス](commands.md) にあります。

## 管理できる設定

reco が管理できるのは、次の表の設定です。表にない設定（マージコミット、squash マージ、rebase マージの許可や、ブランチの保護など）は扱えません。

| 設定ファイルのキー | GitHub の設定画面の項目 |
|---|---|
| `general.delete_branch_on_merge` | 「General」ページの「Pull Requests」の欄の「Automatically delete head branches」 |
| `general.allow_auto_merge` | 「General」ページの「Pull Requests」の欄の「Allow auto-merge」 |
| `actions.permissions.allowed_actions` | 「Actions」の「General」ページの「Actions permissions」の欄で選ぶ、実行を許可する Actions と再利用可能なワークフローの範囲（Actions 許可ポリシー）。`selected` は「Allow *OWNER*, and select non-*OWNER*, actions and reusable workflows」に当たる |
| `actions.permissions.selected_actions.github_owned_allowed` | `selected` のときの「Allow actions created by GitHub」 |
| `actions.permissions.selected_actions.verified_allowed` | `selected` のときの「Allow actions by Marketplace verified creators」 |
| `actions.permissions.selected_actions.patterns_allowed` | `selected` のときの「Allow or block specified actions and reusable workflows」 |
| `actions.workflow.default_workflow_permissions` | 「Actions」の「General」ページの「Workflow permissions」の欄で選ぶ、`GITHUB_TOKEN` の既定の権限（`read` か `write`） |
| `actions.workflow.can_approve_pull_request_reviews` | 「Actions」の「General」ページの「Workflow permissions」の欄の「Allow GitHub Actions to create and approve pull requests」 |

`allowed_actions` を書くと、Actions が有効であることも期待状態に含まれます。reco で Actions を無効にすることはできません。各設定の意味と、値を変えたときに GitHub 側で起きることは [設定ファイルの書き方](configuration.md) にあります。

## 対象にしていないもの

次のことは、reco の対象の外です。

- **Organization が所有するリポジトリと GitHub Enterprise Server**：対象は、GitHub.com の個人アカウントが所有するリポジトリである。reco はトークンの持ち主とリポジトリの所有者が一致するかを確かめないため、設定ファイルには個人アカウントが所有するリポジトリだけを書く必要がある
- **宣言していないリポジトリの自動発見**：`plan` と `apply` が対象にするのは、設定ファイルに書いたリポジトリだけである。アカウントのリポジトリを自動で探して対象に加えることはない
- **リポジトリの作成と削除**：reco は、既存のリポジトリの設定を変えるだけである
- **[設定の良し悪しの評価や提案](safety.md#設定の良し悪しを判断しない)**：reco は、書いた値がセキュリティ上望ましいかを評価せず、どの値にすべきかも提案しない。値は利用者が決める
- **承認を省いた適用**：`apply` は、設定を変える前に必ず端末での対話による明示的な承認を求め、承認を省くオプションはない。このため、CI のように標準入力が端末でない環境からは `apply` で設定を変えられない
- **終了コードによる変更予定の検出**：`plan` は CI でも実行できるが、終了コードで変更予定の有無を区別できない（[自動化に組み込むとき](commands.md#自動化に組み込むとき)）
- **リダイレクトへの追従**：名前を変えたか移転したリポジトリを古い名前で指定すると、reco はエラーで止まる。設定ファイルと `--repo` には現在の名前を書く（[名前を変えたか移転したリポジトリ](troubleshooting.md#名前を変えたか移転したリポジトリ)）

## 安全のための約束

reco は、設定の変更について次のことを約束します。それぞれの条件と理由は [reco の安全性の約束](safety.md) にあります。

- **[宣言していない設定を変えない](safety.md#宣言していない設定を変えない)**：reco が変更の対象に選ぶのは、設定ファイルに書いた設定だけである。例外は、Actions 許可ポリシーを `all` か `local_only` にする変更で、GitHub の仕様により `selected_actions` の値が消える場合である。reco はこれを副作用として変更計画に示し、承認を求める
- **[全リポジトリを確かめてから更新を始める](safety.md#全リポジトリを確かめてから更新を始める)**：設定ファイルの誤りや、どれか一つのリポジトリの読み取りの失敗などが見つかれば、どのリポジトリも更新しない
- **[承認した内容だけを実行する](safety.md#承認した内容だけを実行する)**：表示した変更計画の承認を得るまで更新を始めず、承認のあとで対象や変更内容を差し替えない
- **[承認のあとで状況が変わったら止まる](safety.md#承認のあとで状況が変わったら止まる)**：更新操作の直前ごとに変更前提を確かめ、成り立つと確かめられなければその更新とそのあとの更新を始めない
- **[異常を見つけたら、それ以上更新しない](safety.md#異常を見つけたらそれ以上更新しない)**：更新の失敗などを見つけたら、残りの変更も、あとに並ぶリポジトリも更新しない
- **[反映を確かめてから完了と報告する](safety.md#反映を確かめてから完了と報告する)**：GitHub が更新を受け付けただけでは完了とせず、読み取り直した設定が期待状態になっていることを確かめる
- **[認証情報を表に出さない](safety.md#認証情報を表に出さない)**：トークンを保存せず、出力、エラー、ログに表示しない

一方、次のことは reco が引き受けず、利用者が自分で確かめるか備える必要があります。

- **[変更を自動で元に戻さない](safety.md#変更を自動で元に戻さない)**：`apply` が途中で止まっても、すでに反映した変更はそのまま残る
- **[成否が分からない更新を再送しない](safety.md#成否が分からない更新を再送しない)**：GitHub が更新を受け付けたか分からないときは、送り直さずに止まる
- **[ほかの人や自動化による同時の変更は防げない](safety.md#ほかの人や自動化による同時の変更は防げない)**：更新の直前の確認と更新のあいだや、更新後の確認より後にほかの人が設定を変えても、reco は見つけられない
- **[二つの更新操作からなる変更の途中で止まると、中間状態が残る](safety.md#二つの更新操作からなる変更の途中で止まると中間状態が残る)**：Actions 許可ポリシーが `all` か `local_only` のとき、または Actions が無効のときに、`selected` にする変更は二つの更新操作からなり、そのあいだで止まると、宣言していない許可を含み得る状態が残ることがある

## 始め方

reco を入れてトークンを用意する手順は [インストールと認証の準備](installation.md) に、一つのリポジトリの一つの設定で `show`、`plan`、`apply` を試す手順は [はじめての適用](getting-started.md) にあります。

## References

- [Constitution](../CONSTITUTION.md)：reco が解決する問題と提供する価値、目的としないこと
- [要求定義](../spec/requirements.md)：対象範囲とサポート対象・対象外（2章）、安全性の約束の全条件（5章の不変条件、FR-012、FR-013、QR-001）
- [CLI 仕様](../spec/cli.md)：コマンドの役割（2.1節）、承認（5.1節）、終了コードとリダイレクトに従わないこと（7章）
- [YAML 設定仕様](../spec/configuration.md)：設定ファイルのキーと許容値
- [GitHub リポジトリ API 仕様](../research/repositories.md)：一般設定に当たる設定画面の項目名（4章）
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：Actions の設定に当たる設定画面の項目名（3.2節、4章、5.2節）
