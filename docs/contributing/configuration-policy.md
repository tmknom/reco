# 設定ファイルのインターフェイス設計方針

本ドキュメントは、設定ファイルのキー名、値、階層を決めるときの方針と、GitHub REST API の名前と異なる現行の名前の由来を定めます。キーと値そのものの正本は [YAML 設定仕様](../spec/configuration.md) で、本ドキュメントはそれを決める基準です。キーか値を足すか変える変更と、そのレビューで読みます。

## 1. GitHub REST API の名前に寄せる理由

reco の設定ファイルのキーと値は、GitHub REST API の名前を写すことを基本にします。利用者も開発者も、ある値の意味を調べるときは GitHub の公式ドキュメントを読みます。キー名が API の名前と同じなら、その名前で公式ドキュメントを検索すれば、値の意味、受け付ける値、制約にそのままたどり着けます。

reco が独自の名前を付けると、この対応が切れます。利用者は reco の名前と GitHub の名前の両方を覚え、両者の対応表を引くことになります。開発者も、[GitHub API 仕様](../research/README.md) と OpenAPI の抜粋が API の名前で書かれているため、仕様とコードを読むたびに名前を読み替えることになります。

このため、 **設定をまたいで語や表記をそろえる一貫性より、GitHub の名前との対応を優先します** 。たとえば真偽値のキーは `allow_auto_merge`、`has_wiki`、`sha_pinning_required`、`enabled` と形がそろいませんが、reco は `is_` や `enable_` のような接頭辞にそろえません。そろえると、一つ一つのキーが API のどのフィールドに当たるかを対応表で引くことになるためです。reco の中での一貫性は、GitHub に対応する名前がなく reco が名付けるキー（4章）にだけ求めます。

## 2. キーの種類と名前の由来

キーは、GitHub の何に対応するかによって、次の4種類に分かれます。名前の由来の決め方は種類ごとに違います。

- **設定のキー**：GitHub が値を保存している一つの設定を表すキー（`allow_auto_merge`、`days`、`enforcement` など）
- **関連リソースのキー**：関連リソースの一覧を表すキーと、その要素が持つキー（`environments`、`variables`、`rulesets` と、要素の `name`、`value` など）
- **まとまりのキー**：設定のキーをまとめるだけで、GitHub に対応する値を持たないキー（`general`、`actions.permissions`、`squash_merge_commit` など）。名前の決め方は3章が定める
- **reco 固有の概念のキー**：GitHub に対応する設定がなく、reco の宣言の意味だけを表すキー（存在しないことの宣言を表す `state`）。名前の決め方は4章が定める

### 2.1 設定のキーと関連リソースのキー

設定のキーと関連リソースのキーは、次の順に名前の由来を探し、最初に見つかったものを使います。

1. **更新の本文のフィールド名**：reco が更新のために送る本文のフィールド名を、そのまま使う（`delete_branch_on_merge`、`allowed_actions`、`bypass_actors`）
2. **エンドポイントのパス**：更新の本文にフィールドがなく、エンドポイントへの `PUT` で有効に、`DELETE` で無効にする設定は、パスの最後の部分の `-` を `_` にした名前を使う（`/private-vulnerability-reporting` の `private_vulnerability_reporting`）
3. **一覧のエンドポイント**：関連リソースや要素を一つずつ作るエンドポイントがあり、更新の本文に一覧のフィールドがないときは、一覧の応答で要素を並べる配列のフィールド名か、一覧のエンドポイントのパスの最後の部分を使う（`/actions/variables` の `variables`、`/rulesets` の `rulesets`）

API が同じ設定に複数の名前を持つときは、GitHub の設定画面と機能の説明文書で使われている機能名に近いほうを選びます。Dependabot security updates は、reco が使うエンドポイントのパスが `automated-security-fixes` で、リポジトリの `security_and_analysis` の項目名が `dependabot_security_updates` です。機能名に近い後者を使います。

### 2.2 API の名前から外れてよい条件

API の名前を使うと利用者が機能を特定しにくい場合に限り、API と異なる名前を使ってよいことにします。外れてよいのは、次の二つをともに満たすときです。

- **API の名前が、利用者の知る機能名と別の語である**：GitHub の設定画面と機能の説明文書が、その機能を API の名前と別の語で呼び、API の名前からは機能を特定しにくい
- **代わりの名前を GitHub の語から作れる**：代わりの名前を、GitHub の設定画面か公式ドキュメントで使われている語から作る。reco が新しく作った語は使わない

2つ目の条件を求めるのは、外れた名前からも公式ドキュメントを検索できる状態を保つためです。もし reco が自分で作った語を許せば、その名前は GitHub のどの文書にも現れず、1章の対応が切れます。

Dependabot alerts は、二つの条件を満たす例です。API のエンドポイントは `vulnerability-alerts` ですが、設定画面と機能の説明文書はこの機能を「Dependabot alerts」と呼びます。利用者は `vulnerability_alerts` という名前から Dependabot alerts を思い浮かべにくく、代わりの `dependabot_alerts` は GitHub の機能名そのものです。

一方、「API の名前が長い」「ほかの設定のキーと語をそろえたい」「設定画面の表記と少し違う」は、どれも外れる理由になりません。たとえば `can_approve_pull_request_reviews` は、設定画面の表記（「Allow GitHub Actions to create and approve pull requests」）と語が違いますが、API の名前のままにしています。API の名前から機能を特定でき、1つ目の条件を満たさないためです。

API の名前から外れた名前は、追加する変更で6章の表に由来と理由を書き足します。

## 3. まとまりのキーと階層

まとまりのキーには API のフィールド名がないため、名前と階層を次の規則で決めます。

- **トップレベルのキーは GitHub の設定の領域ごとに置く**：`general`、`actions`、`environments`、`security`、`rulesets` がある。名前はパスの語を使い（`actions`、`environments`、`rulesets`）、パスに使える語がなければ設定画面のページ名を使う（`general`）。どちらでも決まらない `security` は reco が名付ける（4章）。トップレベルのキーごとに [設定ごとの設計](../design/settings/README.md) の文書が一つあるため、キーを足すことは設計文書を一つ足すことになる
- **トップレベルのキーの下のまとまりは、本文を持つ一つのエンドポイントを単位にする**：トップレベルのキーは設定の領域で分けるため、一つのエンドポイントの設定が二つのトップレベルのキーに分かれることがある（リポジトリのエンドポイントの `security_and_analysis` は `security` に置く）。その下では、項目が一つでもまとまりを作る（`access.access_level`、`artifact_and_log_retention.days`）。複数のエンドポイントを一つのまとまりにしない。設定画面で同じ欄にあるか、パスが同じ語で始まるかは、まとめる理由にならない
- **まとまりは、トップレベルのキー（関連リソースでは要素）の直下に並べる**：別の設定の中に入れ子にするのは、親の設定が特定の値のときにだけ存在する設定に限る（`allowed_actions: selected` のときの `selected_actions`、`mode: selected` のときの `patterns`）。パスの上で別のエンドポイントの下にあるかは、階層に使わない（`/actions/permissions/workflow` の `workflow` は `permissions` と並べる）
- **名前は、パスの最後の部分の `-` を `_` にして、そのまま使う**：GitHub がパスで略した語も略したまま書く（`/actions/permissions/fork-pr-workflows-private-repos` の `fork_pr_workflows_private_repos`）。キーからそのまま REST API のドキュメントを検索できるようにするためである。最後の部分だけでは設定を特定できないとき（`/actions/oidc/customization/sub` の `sub`）に限り、reco が名付け（4章）、6章の表に書く
- **API の平らなフィールドを入れ子にするのは、設定グループを作るときに限る**：共通の接頭辞をまとまりのキーにし、残りを子のキーにする（`squash_merge_commit_title` と `squash_merge_commit_message` の `squash_merge_commit.title` と `squash_merge_commit.message`）。子のキーに接頭辞を戻せば API の名前になるため、対応表が要らない
- **API が入れ子で持つ構造は、そのまま写す**：`conditions.ref_name.include`、`rules[].parameters` のように、API の本文が入れ子にしているものは同じ階層に置く
- **取得の応答と更新の本文で形が違うときは、更新の本文の形に合わせる**：Environment の `wait_timer` と `prevent_self_review` は、取得の応答では保護ルールの配列の中にあるが、更新の本文では Environment の直下にあるため、Environment の直下に置く

## 4. reco が名付けるキー

3章の規則で名前が決まらないまとまりのキー（`security`、`oidc_subject`）、reco 固有の概念のキー、値の形を変えたために API に対応する名前がないキー（5.2節）は、reco が名付けます。GitHub の名前に寄せられないぶん、reco の中で表記をそろえます。

- **表記**：snake_case で書き、語を略さない。シーケンスのキーは複数形にする
- **衝突の回避**：同じ親の中で、API のフィールド名と同じ語を別の意味で使わない
- **reco 固有の概念は種類をまたいで同じ形にする**：存在しないことの宣言は、Repository Variable、Environment Variable、Ruleset のどれでも `state: absent` と書く。関連リソースの種類を足して存在しないことの宣言を受け付けるときも、同じキーと値を使う

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

`deployment_branch_policy.mode` が、この二つの規則を当てはめた例です。API の `deployment_branch_policy` は、`null`、`protected_branches: true`、`custom_branch_policies: true` のどれかで、二つのフィールドをともに `true` にする組は受け付けません。reco はこれを `mode` の三つの値にまとめます（値の由来は6章）。

## 6. API の名前と異なる現行のキーと値

次の表は、API の名前と異なる現行のキーと値の由来です。表にない設定のキーと関連リソースのキーは、2.1節の規則で API の名前をそのまま使っています。

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

まとまりのキーのうち、名前がパスの最後の部分のままでないものの由来は次のとおりです。表にないまとまりのキー（`actions`、`actions.permissions`、`actions.workflow`、`actions.access` など）は、3章の規則でパスの最後の部分をそのまま使っています。

| YAML の名前 | 由来 |
|---|---|
| `general` | 設定画面の「General」ページ。設定はリポジトリのエンドポイント（`/repos/{owner}/{repo}`）の本文にあり、パスに使える語がない |
| `actions.oidc_subject` | パス `/actions/oidc/customization/sub` の最後の部分 `sub` だけでは設定を特定できないため、`oidc` を足し、`sub` を略さずに書いた（4章） |
| `security` | reco が名付けた。リポジトリのエンドポイントの `security_and_analysis`、`vulnerability-alerts`、`automated-security-fixes`、`private-vulnerability-reporting` にまたがるセキュリティ機能の設定をまとめる |

## 7. キーか値を足すか変えるときの確認

キーか値を足すか変えるときは、YAML 設定仕様を直す同じ変更で、次の点を確かめます。レビューでも同じ点を確かめます。

1. 2章のどの種類のキーかを決め、その種類の規則で名前を決めたか
2. 設定のキーと関連リソースのキーが API の名前と違うなら、2.2節の二つの条件をともに満たすか。満たさなければ API の名前に戻す
3. 列挙値が API の表記のままか。値の形を変えたなら、5.1節か5.2節のどちらに当たるか
4. reco が名付けたキーが、4章の表記に従い、既存の reco 固有の概念のキーと同じ形か
5. API の名前と異なる名前を足したなら、6章の表に由来と理由を足したか。名前を API の名前に戻したなら、表から消したか

API の名前を確かめるときは [GitHub API 仕様](../research/README.md) を読み、必要な名前が書かれていなければ、公式ドキュメントで確かめて GitHub API 仕様に追記してから使います（[ドキュメント規約](docs-policy.md) 1章）。設定画面の項目名も、GitHub の公式ドキュメントで確かめてから由来に書きます。

## 8. References

- [YAML 設定仕様](../spec/configuration.md)：キー、型、許容値の正本
- [Constitution](../CONSTITUTION.md)：現在の状態と変更内容の理解を優先する原則
- [ドキュメント規約](docs-policy.md)：文書の責務と、GitHub API の事実を参照する向き
- [GitHub リポジトリ API 仕様](../research/repositories.md)：`general` と `security` の API の名前
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：`actions` のエンドポイントのパスとフィールド名
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)：`deployment_branch_policy`、`reviewers`、保護ルールの `type`、設定画面の選択肢
- [GitHub デプロイ元パターン API 仕様](../research/deployment-branch-policies.md)：パターンの `name` と `type`
