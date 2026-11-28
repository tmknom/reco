# 設定ファイルのインターフェイス設計方針

本ドキュメントは、設定ファイルのキー名、値、階層を決めるときの方針と、GitHub REST API の名前と異なる現行の名前の由来を定めます。キーと値そのものの正本は [YAML 設定仕様](../spec/configuration.md) で、本ドキュメントはそれを決める基準です。キーか値を足すか変える変更と、そのレビューで読みます。

## 1. GitHub REST API の名前に寄せる理由

reco の設定ファイルのキーと値は、GitHub REST API の名前を写すことを基本にします。利用者も開発者も、ある値の意味を調べるときは GitHub の公式ドキュメントを読みます。キー名が API の名前と同じなら、その名前で公式ドキュメントを検索すれば、値の意味、受け付ける値、制約にそのままたどり着けます。

reco が独自の名前を付けると、この対応が切れます。利用者は reco の名前と GitHub の名前の両方を覚え、両者の対応表を引くことになります。開発者も、[GitHub API 仕様](../research/README.md) と OpenAPI の抜粋が API の名前で書かれているため、仕様とコードを読むたびに名前を読み替えることになります。

このため、 **設定をまたいで語や表記をそろえる一貫性より、GitHub の名前との対応を優先します** 。たとえば真偽値のキーは `allow_auto_merge`、`has_wiki`、`sha_pinning_required`、`enabled` と形がそろいませんが、reco は `is_` や `enable_` のような接頭辞にそろえません。そろえると、一つ一つのキーが API のどのフィールドに当たるかを対応表で引くことになるためです。名前の一貫性は、GitHub に対応する名前がなく reco が名付けるキー（4章）にだけ求めます。名前と別に、項目の省略の扱いと一覧の読み方は、設定をまたいでそろえます（6章）。

## 2. キーの種類と名前の由来

キーは、GitHub の何に対応するかによって、次の4種類に分かれます。名前の由来の決め方は種類ごとに違います。

- **設定のキー**：GitHub が値を保存している一つの設定を表すキー（`allow_auto_merge`、`days`、`enforcement` など）
- **リポジトリリソースのキー**：リポジトリリソースの一覧を表すキーと、その要素が持つキー（`environments`、`variables`、`rulesets` と、要素の `name`、`value` など）
- **まとまりのキー**：設定のキーをまとめるだけで、GitHub に対応する値を持たないキー（`general`、`actions.permissions`、`squash_merge_commit` など）。名前の決め方は3章が定める
- **reco 固有の概念のキー**：GitHub に対応する設定がなく、reco の宣言の意味だけを表すキー（存在しないことの宣言を表す `state`、宣言の対象を表す `repositories` とリポジトリ指定の `repositories[].name`）。名前の決め方は4章が定める

### 2.1 設定のキーとリポジトリリソースのキー

設定のキーとリポジトリリソースのキーは、次の順に名前の由来を探し、最初に見つかったものを使います。

1. **更新の本文のフィールド名**：reco が更新のために送る本文のフィールド名を、そのまま使う（`delete_branch_on_merge`、`allowed_actions`、`bypass_actors`）
2. **エンドポイントのパス**：更新の本文にフィールドがなく、エンドポイントへの `PUT` で有効に、`DELETE` で無効にする設定は、パスの最後の部分の `-` を `_` にした名前を使う（`/private-vulnerability-reporting` の `private_vulnerability_reporting`）
3. **一覧のエンドポイント**：リポジトリリソースや要素を一つずつ作るエンドポイントがあり、更新の本文に一覧のフィールドがないときは、一覧の応答で要素を並べる配列のフィールド名か、一覧のエンドポイントのパスの最後の部分を使う（`/actions/variables` の `variables`、`/rulesets` の `rulesets`）

API が同じ設定に複数の名前を持つときは、GitHub の設定画面と機能の説明文書で使われている機能名に近いほうを選びます。Dependabot security updates は、reco が使うエンドポイントのパスが `automated-security-fixes` で、リポジトリの `security_and_analysis` の項目名が `dependabot_security_updates` です。機能名に近い後者を使います。

### 2.2 API の名前から外れてよい条件

API の名前を使うと利用者が機能を特定しにくい場合に限り、API と異なる名前を使ってよいことにします。外れてよいのは、次の二つをともに満たすときです。

- **API の名前が、利用者の知る機能名と別の語である**：GitHub の設定画面と機能の説明文書が、その機能を API の名前と別の語で呼び、API の名前からは機能を特定しにくい
- **代わりの名前を GitHub の語から作れる**：代わりの名前を、GitHub の設定画面か公式ドキュメントで使われている語から作る。reco が新しく作った語は使わない

2つ目の条件を求めるのは、外れた名前からも公式ドキュメントを検索できる状態を保つためです。もし reco が自分で作った語を許せば、その名前は GitHub のどの文書にも現れず、1章の対応が切れます。

Dependabot alerts は、二つの条件を満たす例です。API のエンドポイントは `vulnerability-alerts` ですが、設定画面と機能の説明文書はこの機能を「Dependabot alerts」と呼びます。利用者は `vulnerability_alerts` という名前から Dependabot alerts を思い浮かべにくく、代わりの `dependabot_alerts` は GitHub の機能名そのものです。

一方、「API の名前が長い」「ほかの設定のキーと語をそろえたい」「設定画面の表記と少し違う」は、どれも外れる理由になりません。たとえば `can_approve_pull_request_reviews` は、設定画面の表記（「Allow GitHub Actions to create and approve pull requests」）と語が違いますが、API の名前のままにしています。API の名前から機能を特定でき、1つ目の条件を満たさないためです。

API の名前から外れた名前は、追加する変更で7章の表に由来と理由を書き足します。

## 3. まとまりのキーと階層

まとまりのキーには API のフィールド名がないため、名前と階層を次の規則で決めます。

- **トップレベルのキーは GitHub の設定の領域ごとに置く**：`general`、`actions`、`environments`、`security`、`rulesets` がある。名前はパスの語を使い（`actions`、`environments`、`rulesets`）、パスに使える語がなければ設定画面のページ名を使う（`general`）。どちらでも決まらない `security` は reco が名付ける（4章）。トップレベルのキーごとに [設定ごとの設計](../design/settings/README.md) の文書が一つあるため、キーを足すことは設計文書を一つ足すことになる
- **トップレベルのキーの下のまとまりは、本文を持つ一つのエンドポイントを単位にする**：トップレベルのキーは設定の領域で分けるため、一つのエンドポイントの設定が二つのトップレベルのキーに分かれることがある（リポジトリのエンドポイントの `security_and_analysis` は `security` に置く）。その下では、項目が一つでもまとまりを作る（`access.access_level`、`artifact_and_log_retention.days`）。複数のエンドポイントを一つのまとまりにしない。設定画面で同じ欄にあるか、パスが同じ語で始まるかは、まとめる理由にならない
- **まとまりは、トップレベルのキー（リポジトリリソースでは要素）の直下に並べる**：別の設定の中に入れ子にするのは、親の設定が特定の値のときにだけ存在する設定に限る（`allowed_actions: selected` のときの `selected_actions`、`mode: selected` のときの `patterns`）。パスの上で別のエンドポイントの下にあるかは、階層に使わない（`/actions/permissions/workflow` の `workflow_permissions` は `permissions` と並べる）
- **名前は、パスの最後の部分の `-` を `_` にして、そのまま使う**：GitHub がパスで略した語も略したまま書く（`/actions/permissions/fork-pr-workflows-private-repos` の `fork_pr_workflows_private_repos`）。キーからそのまま REST API のドキュメントを検索できるようにするためである。最後の部分だけでは設定を特定できないときに限り、reco が名付け（4章）、7章の表に書く。特定できないとは、最後の部分がその親の部分に修飾されて初めて設定を指し、親の部分を外すと、ほかの設定やより広い対象を指すように読める場合をいう（`/actions/oidc/customization/sub` の `sub`、`/actions/permissions/workflow` の `workflow`）。上の規則でまとまりを親のまとまりと並べると、パスの上の親の部分が外れるため、この場合に当たるかを確かめる
- **API の平らなフィールドを入れ子にするのは、設定グループを作るときに限る**：共通の接頭辞をまとまりのキーにし、残りを子のキーにする（`squash_merge_commit_title` と `squash_merge_commit_message` の `squash_merge_commit.title` と `squash_merge_commit.message`）。子のキーに接頭辞を戻せば API の名前になるため、対応表が要らない
- **API が入れ子で持つ構造は、そのまま写す**：`conditions.ref_name.include`、`rules[].parameters` のように、API の本文が入れ子にしているものは同じ階層に置く
- **取得の応答と更新の本文で形が違うときは、更新の本文の形に合わせる**：Environment の `wait_timer` と `prevent_self_review` は、取得の応答では保護ルールの配列の中にあるが、更新の本文では Environment の直下にあるため、Environment の直下に置く

## 4. reco が名付けるキー

3章の規則で名前が決まらないまとまりのキー（`security`、`oidc_subject`）、reco 固有の概念のキー、値の形を変えたために API に対応する名前がないキー（5.2節）は、reco が名付けます。GitHub の名前に寄せられないぶん、reco の中で表記をそろえます。

- **表記**：snake_case で書き、語を略さない。シーケンスのキーは複数形にする
- **衝突の回避**：同じ親の中で、API のフィールド名と同じ語を別の意味で使わない
- **reco 固有の概念は種類をまたいで同じ形にする**：存在しないことの宣言は、Repository Variable、Environment Variable、Ruleset のどれでも `state: absent` と書く。リポジトリリソースの種類を足して存在しないことの宣言を受け付けるときも、同じキーと値を使う
- **識別子は `name` にそろえる**：一覧の要素を名前で指すキーは、リポジトリリソースの要素の `name` と同じ語にする。リポジトリ指定も、値が API の `full_name` に当たる `OWNER/REPO` の形でも、キーは `repositories[].name` とする。要素を指すキーが一覧ごとに違うと、利用者がどのキーで要素を指すかを一覧ごとに覚えることになるため

reco 固有の概念のキーは、概念そのものを [ドメインモデル](../design/domain.md) か YAML 設定仕様が定めてから足します。GitHub に対応がない以上、キーの意味を確かめられる場所は reco の文書だけになるためです。

## 5. 値

値も、キーと同じく API の表記を写すことを基本にします。

- **列挙値は API の表記のまま書く**：大文字小文字と区切りを変えない（`PR_TITLE`、`local_only`、`User`、`RepositoryRole`、`~DEFAULT_BRANCH`）。設定ごとに表記がそろわないのは API を写した結果であり、reco はそろえない
- **受け付ける値を絞っても、値の名前は変えない**：`access_level` の `organization`、`enforcement` の `evaluate`、担当者の `Team` を受け付けないのは YAML 設定仕様の判断であり、受け付ける値は API の値のまま書く
- **ID は API の ID のまま書く**：担当者の `id`、Bypass の `actor_id` のように、API が数値の ID で指すものは、login やロール名に読み替えない

### 5.1 API の表現が YAML の値にならない設定

API が、利用者が YAML に書く値でない形で状態を表すときは、状態を真偽値で表します。キー名は2章の規則で決めた設定の名前のままにします。

- **状態のオブジェクト**：`security_and_analysis` の項目は `status` に `enabled` か `disabled` を持つオブジェクトである。reco は `secret_scanning: true` のように真偽値で書く
- **エンドポイントへの操作**：Dependabot alerts、Dependabot security updates、Private vulnerability reporting は、エンドポイントへの `PUT` で有効に、`DELETE` で無効にし、更新の本文を持たない。reco はこれらも真偽値で書く

### 5.2 API が `null` やフィールドの組で表す状態

reco はどの位置でも `null` を受け付けません（YAML 設定仕様 14.1節）。このため、API が `null` で表す状態には名前のある値を与えます。名前は、公式ドキュメントがその状態を説明する語から取ります。

API が複数のフィールドの組で一つの状態を表し、組の中に GitHub が受け付けない組み合わせがあるときは、一つの列挙値にまとめてよいことにします。受け付けない組み合わせを、宣言の形の上で書けなくするためです。まとめた列挙のキーは reco が名付け（4章）、値は API のフィールド名から取ります。API のフィールド名が2.2節の1つ目の条件に当たるときは、設定画面の選択肢の名前から取ります。

まとめるときも、設定のキーは API の名前のままにします（`deployment_branch_policy`）。5.1節と同じく、キーが表す設定は変わらず、値の形だけが変わるためです。まとめたフィールドの名前（`protected_branches`、`custom_branch_policies`）は、子のキーとして残しません。もし `protected_branches` を真偽値のキーとして残したまま列挙と並べれば、API と同じ名前のキーが API と違う規則で読まれ、公式ドキュメントで調べた利用者が意味を取り違えます。

`deployment_branch_policy.mode` が、この二つの規則を当てはめた例です。API の `deployment_branch_policy` は、`null`、`protected_branches: true`、`custom_branch_policies: true` のどれかで、二つのフィールドをともに `true` にする組は受け付けません。reco はこれを `mode` の三つの値にまとめます（値の由来は7章）。

## 6. 省略と一覧の読み方

項目を省略したときの扱いと一覧の読み方は、名前と違い、GitHub の名前との対応を切らずにそろえられるため、設定をまたいで次の規則でそろえます。

### 6.1 値を構成する項目の省略

設定の省略は管理範囲外を意味します（YAML 設定仕様 2章）。複数の項目で一つの値を表すもの（設定グループと、Ruleset のように中身の全体を一つの値とするリポジトリリソース）の項目を省略できるのは、次の二つをともに満たすときに限ります。

- **GitHub が、省略した項目を送らない更新を受け付ける**：マージメッセージの `title` と `message` は、GitHub が片方だけの更新を拒否することがあるため、省略できない
- **宣言を適用しても、省略した項目の値を GitHub が変えない**：`oidc_subject.use_immutable_subject` は、省略しても GitHub が保存された値を保つため、省略できる。`selected_actions` の3項目と `fork_pr_workflows_private_repos` の4項目は、許可ポリシーや Fork からの実行の可否を変える更新で GitHub が残りの項目を変えることがあるため、省略できない。Ruleset のルールの `parameters` は、`rules` を置き換える更新で GitHub が送らなかったパラメータを既定値に戻すため、どれも省略できない

2つ目の条件を求めるのは、省略を管理範囲外と読んだ利用者の期待を守るためです。もし GitHub が適用の中で変える項目の省略を許せば、その項目は管理範囲外にならず、利用者が書かなかった値に変わります。どちらの条件に当たるかは、GitHub API 仕様の確認事項から決め、確かめられない項目は省略できないことにします。

### 6.2 一覧の読み方

シーケンスの値には、YAML 設定仕様 2.1節が定める二つの読み方があり、新しいシーケンスはどちらかに分類します。

- **リポジトリリソースの一覧**：要素の一つ一つが、親の設定から独立して GitHub 上に存在し続けるリポジトリリソースを表す一覧（`environments`、`rulesets` など）。書いた名前だけを管理し、要素を消しても削除しない
- **値の一覧**：一覧の全体が一つの設定の値になる一覧（`patterns_allowed`、`deployment_branch_policy.patterns` など）。宣言した一覧が適用後の一覧になり、宣言にない要素はなくなる

GitHub が要素ごとに作成と削除のエンドポイントを持つかは、分類の基準になりません。`deployment_branch_policy.patterns` の要素は GitHub 上で一つずつ作られますが、方式を `selected` から変えると GitHub がすべて消すため、デプロイ元制限という一つの設定の値の一部として扱い、値の一覧に分類します。

要素が `name` のキーを持つかも、分類の基準になりません。`deployment_branch_policy.patterns[].name` は API のフィールド名で、値の一覧の要素のキーです。読み方の違いはキーの名前に表れないため、利用者ガイドの [Configuration](../user-guide/configuration.md) は、一覧ごとにどちらの読み方かを示します。

## 7. API の名前と異なる現行のキーと値

次の表は、API の名前と異なる現行のキーと値の由来です。表にない設定のキーとリポジトリリソースのキーは、2.1節の規則で API の名前をそのまま使っています。

| YAML の名前 | API の名前 | 由来と理由 |
|---|---|---|
| `security.dependabot_alerts` | エンドポイント `vulnerability-alerts` | 機能名「Dependabot alerts」（2.2節）。値はエンドポイントへの操作を真偽値にしたもの（5.1節） |
| `security.dependabot_security_updates` | エンドポイント `automated-security-fixes`、`security_and_analysis.dependabot_security_updates` | API の二つの名前のうち、機能名に近い後者（2.1節）。値はエンドポイントへの操作を真偽値にしたもの（5.1節） |
| `security.secret_scanning`、`security.secret_scanning_push_protection` | `security_and_analysis.secret_scanning.status`、`security_and_analysis.secret_scanning_push_protection.status` | 項目名のまま、`status` のオブジェクトを真偽値にしたもの（5.1節） |
| `security.private_vulnerability_reporting` | エンドポイント `private-vulnerability-reporting` | パスの語（2.1節）。値はエンドポイントへの操作を真偽値にしたもの（5.1節） |
| `general.squash_merge_commit.title`、`.message` と `general.merge_commit.title`、`.message` | `squash_merge_commit_title`、`squash_merge_commit_message`、`merge_commit_title`、`merge_commit_message` | 設定グループを作るための入れ子（3章） |
| `environments[].deployment_branch_policy.mode` | `deployment_branch_policy` の `protected_branches` と `custom_branch_policies` の組、または `null` | フィールドの組を一つの列挙にまとめた、reco が名付けたキー（5.2節） |
| `mode` の値 `all` | `deployment_branch_policy: null` | 公式ドキュメントが `null` を「すべてのブランチを許可する」と説明する語（5.2節） |
| `mode` の値 `protected_branches` | `protected_branches: true` | フィールド名（5.2節） |
| `mode` の値 `selected` | `custom_branch_policies: true` | 公式ドキュメントの選択肢は「No restriction」「Protected branches only」「Selected branches and tags」で、`custom_branch_policies` は選択肢に現れず、API の名前から選択肢を特定しにくい（2.2節）。`selected` は選択肢「Selected branches and tags」の語 |
| `environments[].deployment_branch_policy.patterns` | 一覧の応答の `branch_policies`（エンドポイント `deployment-branch-policies`） | `branch_policies` は設定画面と機能の説明文書に現れず、機能の説明文書は要素を「名前のパターン」と呼び、REST API のドキュメントも要素を「name pattern」と説明する（2.2節）。`patterns` はその語から取った |
| `repositories`、`repositories[].name` | 対応するキーはない。`name` の値の `OWNER/REPO` は、リポジトリの取得の応答の `full_name` に当たる | 宣言の対象を表す reco 固有の概念のキー（4章）。`name` は、ほかの一覧の要素を指す `name` と同じ語にそろえた（4章） |

まとまりのキーのうち、名前がパスの最後の部分のままでないものの由来は次のとおりです。表にないまとまりのキー（`actions`、`actions.permissions`、`actions.access`、`actions.artifact_and_log_retention` など）は、3章の規則でパスの最後の部分をそのまま使っています。

| YAML の名前 | 由来 |
|---|---|
| `general` | 設定画面の「General」ページ。設定はリポジトリのエンドポイント（`/repos/{owner}/{repo}`）の本文にあり、パスに使える語がない |
| `actions.oidc_subject` | パス `/actions/oidc/customization/sub` の最後の部分 `sub` だけでは設定を特定できないため、`oidc` を足し、`sub` を略さずに書いた（4章） |
| `actions.workflow_permissions` | パス `/actions/permissions/workflow` の最後の部分 `workflow` は、親の部分 `permissions` を外すとワークフロー全般の設定に読めるため、外れた `permissions` を後ろに足した。設定画面の欄の名前「Workflow permissions」と同じ語で、REST API の操作の題名「Set default workflow permissions for a repository」にも現れる |
| `security` | reco が名付けた。リポジトリのエンドポイントの `security_and_analysis`、`vulnerability-alerts`、`automated-security-fixes`、`private-vulnerability-reporting` にまたがるセキュリティ機能の設定をまとめる |

## 8. キーか値を足すか変えるときの確認

キーか値を足すか変えるときは、YAML 設定仕様を直す同じ変更で、次の点を確かめます。レビューでも同じ点を確かめます。

1. 2章のどの種類のキーかを決め、その種類の規則で名前を決めたか
2. 設定のキーとリポジトリリソースのキーが API の名前と違うなら、2.2節の二つの条件をともに満たすか。満たさなければ API の名前に戻す
3. 列挙値が API の表記のままか。値の形を変えたなら、5.1節か5.2節のどちらに当たるか
4. reco が名付けたキーが、4章の表記に従い、既存の reco 固有の概念のキーと同じ形か
5. 値を構成する項目に省略できるものを定めたなら、6.1節の二つの条件をともに満たすか。シーケンスを足したなら、6.2節のどちらの一覧かを決め、YAML 設定仕様と利用者ガイドに書いたか
6. API の名前と異なる名前を足したなら、7章の表に由来と理由を足したか。名前を API の名前に戻したなら、表から消したか

API の名前を確かめるときは [GitHub API 仕様](../research/README.md) を読み、必要な名前が書かれていなければ、公式ドキュメントで確かめて GitHub API 仕様に追記してから使います（[ドキュメント規約](docs-policy.md) 1章）。由来に書く設定画面の項目名も、GitHub API 仕様に書かれたものを使います（同 5.6節）。

## 9. References

- [YAML 設定仕様](../spec/configuration.md)：キー、型、許容値の正本
- [Constitution](../CONSTITUTION.md)：現在の状態と変更内容の理解を優先する原則
- [ドキュメント規約](docs-policy.md)：文書の責務と、GitHub API の事実を参照する向き
- [GitHub リポジトリ API 仕様](../research/repositories.md)：`general` と `security` の API の名前
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：`actions` のエンドポイントのパスとフィールド名
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)：`deployment_branch_policy`、`reviewers`、保護ルールの `type`、設定画面の選択肢
- [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md)：パターンの `name` と `type`
- [GitHub Actions OIDC API 仕様](../research/actions-oidc.md)：`use_immutable_subject` を省略した更新での値の保持
- [GitHub Rules API 仕様](../research/rules.md)：`rules` を置き換える更新で、送らなかったパラメータが既定値に戻ること
