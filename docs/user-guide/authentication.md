# トークンの用意

reco は、環境変数 `GH_TOKEN` か `GITHUB_TOKEN` で渡されたトークンで GitHub にアクセスします。

## gh のトークンを渡す

主な使い方は、GitHub CLI（`gh`）にログインしておき、`gh auth token` が出力するトークンを `GH_TOKEN` に渡すことです。

`gh auth login` でログインすると、`gh` は取得したトークンを手元に保存します。`gh auth token` は、その保存されたトークンを出力するコマンドです。reco 自身は `gh` からトークンを取り出さないため、環境変数で渡す必要があります。

```bash
export GH_TOKEN=$(gh auth token)
reco show --repo OWNER/REPO
```

`show` が対象のリポジトリの設定を表示すれば、トークンを渡せています。

## トークンの持ち主のロール

トークンでできることは、トークンの持ち主（`gh` にログインしたアカウント）が対象のリポジトリで持つロールを超えません。reco には、対象のリポジトリで Admin ロールを持つアカウントのトークンを渡します。

Read など Write より下のロールでは、GitHub が一般設定（`general`）の値を応答に含めないため、`show`、`plan`、`apply` のどれもエラーで止まります。

## References

- [CLI 仕様](../spec/cli.md)：認証情報の選択、`gh` から取得しないこと、トークンの持ち主のロール（3章）
- [GitHub REST API 共通仕様](../research/rest-api.md)：`gh auth login` と `gh auth token` の動作、トークンの権限がロールを超えないこと（7章）
