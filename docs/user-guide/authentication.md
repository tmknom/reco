# Authentication

reco は、環境変数 `GH_TOKEN` か `GITHUB_TOKEN` で渡されたトークンで GitHub にアクセスします。

## gh のトークンを渡す

GitHub CLI（`gh` コマンド）のトークンを利用する方法が、もっとも簡単です。
事前に `gh auth login` でログインし、`gh auth token` が出力するトークンを `GH_TOKEN` 環境変数へセットします。

```bash
export GH_TOKEN=$(gh auth token)
reco show --repo OWNER/REPO
```

`show` が対象のリポジトリの設定を表示すれば、トークンを渡せています。

## トークンの持ち主のロール

トークンでできることは、トークンの持ち主（`gh` にログインしたアカウント）が対象のリポジトリで持つロールを超えません。reco には、対象のリポジトリで Admin ロールを持つアカウントのトークンを渡します。

Read など Write より下のロールでは、GitHub がマージ方式などの一般設定（`general`）の値の一部を応答に含めないため、`show`、`plan`、`apply` のどれもエラーで止まります。

Admin より下のロールでは、Dependabot alerts、Dependabot security updates、Secret scanning、リポジトリの push protection の値を読めないため、`show` と、`security` にこれらの設定を書いたリポジトリの `plan` と `apply` がエラーで止まります。公開リポジトリの `apply` は、`general` だけを書いた場合も、更新の前後に Secret scanning の値を読むため、同じく止まります。`general` の設定を変えるにも Admin ロールが要るため、この読み取りで要るロールは増えません。Private vulnerability reporting の値は Admin より下のロールでも読めますが、変えるには Admin ロールが要ります。

## Environment を消すときの権限

Environment に `state: absent` を書いたリポジトリでは、その Environment が GitHub 上にあれば、`plan` と `apply` が Environment Secret の名前の一覧も読み、`apply` が承認のあとに Environment を消します。Admin ロールを持つアカウントの `gh auth token` のトークンなら、どちらもできます。fine-grained personal access token を渡すときは、Secret の一覧を読むのに Environments の read の権限を、Environment を消すのに Administration の write の権限を与えます。Secret の一覧を読めないトークンでは、`plan` と `apply` はエラーで止まります。

## References

- [CLI 仕様](../spec/cli.md)：トークンの選び方と、reco が動作を確かめたロール、Environment の削除と Environment Secret の一覧に要る権限（3章）
- [GitHub Actions シークレット API 仕様](../research/actions-secrets.md)：Environment Secret の一覧に要る権限（1章）
- [GitHub デプロイ環境 API 仕様](../research/deployment-environments.md)：Environment の削除に要る権限（1章）
