# reco ユーザーガイド

reco は、GitHub リポジトリの設定を YAML で書き、現在の設定との差を示して、承認を得てから変更する CLI です。このガイドは、GitHub の個人アカウントで複数のリポジトリを持ち、リポジトリの設定画面の項目をある程度知っている人に向けて、reco の使い方を説明します。

## 目的から選ぶ

| 目的 | 読む文書 |
|---|---|
| 管理したい設定を reco で扱えるか、reco で何ができるかを知りたい | [reco の概要](overview.md) |
| reco を入れたい | [インストール](installation.md) |
| reco に渡すトークンを用意したい | [トークンの用意](authentication.md) |
| 一つの設定で reco を試したい | [はじめての適用](getting-started.md) |
| 管理したい設定を設定ファイルにどう書くかを知りたい | [設定ファイルの書き方](configuration.md) |
| コマンドとオプション、出力の読み方を調べたい | [コマンドリファレンス](commands.md) |
| リポジトリを増やす、対象を絞る、定期的に確かめる、変えた値を戻す、管理をやめる方法を知りたい | [複数リポジトリの運用](operations.md) |
| エラーのあとや、`apply` が途中で止まったあと、何をすればよいかを知りたい | [困ったときは](troubleshooting.md) |

関心があるときに読む文書は次のとおりです。

| 目的 | 読む文書 |
|---|---|
| `allowed_actions` を `selected` とほかの値のあいだで変えるときに起きることを知りたい | [Actions 許可ポリシーの切り替え](actions-policy.md) |
| reco が設定を変えるときにどう安全を保つかを知りたい | [reco の安全性](safety.md) |
| 取得した配布物が reco のリリース用ワークフローで作られたものかを確かめたい | [配布物の検証](verification.md) |

## References

- [要求定義](../spec/requirements.md)：reco が保証すること
- [CLI 仕様](../spec/cli.md)：コマンド、出力、終了コード
- [YAML 設定仕様](../spec/configuration.md)：設定ファイルで受け付ける入力
- [reco の Issue](https://github.com/tmknom/reco/issues)：ガイドの誤りの報告先
