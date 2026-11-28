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
