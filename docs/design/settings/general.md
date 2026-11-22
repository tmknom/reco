# General の設計

設定宣言の `general`（[YAML 設定仕様](../../spec/configuration.md) 3章）に属する設定の設計を定めます。文書の構成は [設定ごとの設計](README.md) に従います。

## 設定と管理の単位

| 日本語 | 英語 | 意味 |
|---|---|---|
| リポジトリ一般設定 | General | Pull Request の Auto-merge の許可やマージ後のブランチ自動削除など、リポジトリ全般の振る舞いに関する設定の領域 |

`allow_auto_merge` と `delete_branch_on_merge` は、それぞれ単独の管理の単位です。General の2設定は、Secret scanning の2設定と同じ更新エンドポイント（`PATCH /repos/{owner}/{repo}`）で更新します。この依存の扱いは [Security の設計](security.md) の Secret scanning が定めます。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-001` | Functional Requirement |
| `alignment` | `FR-005` | Functional Requirement |

## 観測

リポジトリ ID を返す `GET /repos/{owner}/{repo}` は General の値も返すため、どの観測も General を含みます。

トークン所有者が Write 以上のロールを持たないリポジトリでは、`GET /repos/{owner}/{repo}` の応答から `allow_auto_merge` と `delete_branch_on_merge` がそろって欠けます（[GitHub リポジトリ API 仕様](../../research/repositories.md) 1.1）。この欠落は取得条件による観測不能として扱わず、観測操作の失敗のままとします。原因は権限の不足であり、ドメインモデルは権限による失敗を観測結果に含めないためです。観測不能として `plan` を進めても、同じ権限では更新できず、利用者が対処すべき原因が後の失敗まで見えなくなります。

`GET /repos/{owner}/{repo}` の応答の `private` と `permissions` は、ほかの設定の領域の取得条件と応答の解釈に使います（[Actions の設計](actions.md)、[Security の設計](security.md)）。`private` は、どの設定の領域を読む観測でも取得契約に含め、欠けた応答は取得契約に適合しない応答とします。欠けた `private` を公開とみなすと、取得条件による観測不能を根拠なく作るためです。`permissions` は取得契約に含めず、Dependabot の領域を読まない観測は、欠けていても `null` でも失敗しません（読む観測での扱いは [Security の設計](security.md) の Dependabot の観測）。ただし、型の合わない `permissions`（オブジェクトでない、`admin` が boolean でない）は解釈できない本文であり、どの設定の領域を読む観測でも取得契約に適合しない応答とします。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-004` | Functional Requirement |
| `alignment` | `QR-002` | Quality Requirement |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## 更新方式

General は、宣言された変更対象だけを `PATCH /repos/{owner}/{repo}` に送り、省略したフィールドの保持に依拠して部分更新します。省略したフィールドが保持されることを実際の動作で確認しています（[GitHub リポジトリ API 仕様](../../research/repositories.md) 4章）。これが、宣言された変更対象だけを送る方式の根拠です。

更新後に再取得して更新直前の値と比べるのは、この更新エンドポイントで reco が観測する設定のうち、その論理的な設定変更が変えない設定です（`delete_branch_on_merge` を変える論理的な設定変更では、`allow_auto_merge` と Secret scanning の2設定）。Secret scanning の2設定は、非公開リポジトリでは観測不能のため比べません。トップレベルだけを送る `PATCH` が `security_and_analysis` の中の項目を変えなかったことを、公開リポジトリでの実際の動作で確認しており（GitHub リポジトリ API 仕様 5章）、これを非公開リポジトリでの保全の根拠とします。名前、説明など、reco が読まないフィールドと、取得条件の判定にだけ読む公開範囲は再確認せず、保持の確認結果を根拠に保全されるとみなします。

Secret scanning の2設定を比べるため、General を変える更新操作の更新直前と更新後の観測は Secret scanning の領域も読みます（[Security の設計](security.md) の Secret scanning の観測）。

| Relationship | Target | Target Type |
|---|---|---|
| `alignment` | `FR-011` | Functional Requirement |
| `alignment` | `INV-001` | Invariant |
| `grounding` | [GitHub リポジトリ API 仕様](../../research/repositories.md) | Repository File |

## References

- [設定ごとの設計](README.md)
- [YAML 設定仕様](../../spec/configuration.md)
- [GitHub リポジトリ API 仕様](../../research/repositories.md)
