# Troubleshooting

## エラーの見方

reco のエラーは、標準エラー出力に JSON の1行で出ます。`error` の値が、reco が何をしようとして、どのリポジトリで、なぜ止まったかを示します。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: authentication failed (HTTP 401)"}
```

この例では、`example/project-a` の設定を読み取ろうとして、認証に失敗しています。エラーで止まったときは、`apply` が更新を始めたあとでなければ、GitHub の設定は何も変わっていません。

## 設定ファイルが見つからない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: open reco.yaml: no such file or directory"}
```

設定ファイルのあるディレクトリで実行するか、`--config` で設定ファイルのパスを指定します。コンテナイメージで動かしているなら、`-v` と `-w` を付けているかを確かめます。

## 設定ファイルの書き方の誤り

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: repositories[0].general.allow_auto_merge (line 4, column 25): must be true or false; got the quoted string \"true\"; write true without quotes"}
```

エラーが示す行と列を直します。この例は、真偽値を引用符で囲んだ誤りです。`--repo` で誤りのないリポジトリに絞っていても、設定ファイルのどこかに誤りがあれば止まります。

## `--repo` のリポジトリが設定ファイルにない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"repository example/other is not declared"}
```

`plan` と `apply` の `--repo` には、設定ファイルに書いたリポジトリだけを指定できます。名前の綴りを設定ファイルの `name` と揃えます。設定ファイルに書いていないリポジトリの設定を見るだけなら、`show` を使います。

## トークンがない、認証に失敗する

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"GH_TOKEN or GITHUB_TOKEN must be set to a non-empty token"}
```

`GH_TOKEN` にトークンを渡します。コンテナイメージでは `-e GH_TOKEN` も付けます。

```bash
export GH_TOKEN=$(gh auth token)
```

`authentication failed (HTTP 401)` が出るときは、渡したトークンが無効です。上のコマンドでトークンを取り直します。

## 権限が足りない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: not found or not accessible (HTTP 404)"}
```

`HTTP 404`、`HTTP 403`、`missing fields` のどれかを含むエラーは、トークンの持ち主（`gh` にログインしたアカウント）がそのリポジトリで十分なロールを持っていないときや、トークンの権限が足りないときに出ます。そのリポジトリで Admin ロールを持つアカウントの、`gh auth token` で取ったトークンを渡します。`HTTP 404` は、リポジトリの名前の綴りを誤ったときにも出ます。

## `Plan cannot be applied.` と表示された

変更計画に `Applicability: not applicable` の行が付いた変更があると、`plan` と `apply` は変更計画の最後に `Plan cannot be applied.` を表示し、エラーで終わります。`apply` は承認を求めず、ほかのリポジトリも含めて何も変えません。

```text
Repository: example/project-a
  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true
    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)

Plan: 1 repository to update, 1 change
Repositories checked: 1

Plan cannot be applied.
```

`apply` では、続けて次のエラーが出ます。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"plan cannot be applied: example/project-a: actions.permissions.sha_pinning_required is not applicable; no changes made"}
```

理由は、変更計画の `Applicability:` の行の括弧の中にあります。セミコロンの前が成り立っていないこと、後ろが設定ファイルの直し方です。この例では Actions が無効のため、同じリポジトリに `enabled: true` か `allowed_actions` を書き足してから、`plan` で `Plan cannot be applied.` が消えたことを確かめ、`apply` を実行し直します。パブリックリポジトリに `access_level` を書いて `(the repository is public; remove access_level, ...)` と示されたら、そのリポジトリの `access_level` を設定ファイルから消します。`(an environment on GitHub has a name reco cannot compare, ...)` と示されたら、GitHub 上に半角以外の文字を含む名前の Environment があり、設定ファイルに書いた Environment があるかを reco が確かめられません。`Applicability:` の行の上に名前が出ている Environment を、設定ファイルから消します。

## `applying environment changes is not supported` と表示された

`apply` は、Environment の作成と `deployment_branch_policy` の変更をまだ実行しません。変更計画にそれらがあると、変更計画を表示せず、何も変えずに次のエラーで終わります。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"example/project-a: environments.production cannot be applied: applying environment changes is not supported; no changes made"}
```

ほかの設定だけを変えたいときは、`environments` を設定ファイルから一時的に外して `apply` を実行します。外しても、GitHub 上の Environment は消えません。

## 承認できない

承認を求められずに `approval not available` で終わるときは、標準入力が端末になっていません。端末から直接 `apply` を実行します。コンテナイメージでは `-it` を付けます。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"apply cancelled: approval not available: standard input is not a terminal; no changes made"}
```

入力したのに `approval declined` で終わるときは、受け付けない入力をしています。もう一度 `apply` を実行し、小文字の `y` か `yes` を入力します。

## `apply` が途中で止まった

`apply` の結果に `failed` か `not attempted` があるときです。reco は、どこかの変更が完了しなかったら、残りの変更を始めずに止まります。すでに反映した変更は元に戻りません。

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

`failed` の下の行は、次のことを示します。

| 行 | 意味 |
|---|---|
| `Update request: succeeded` | GitHub は更新を受け付けた |
| `Update request: rejected` | GitHub は更新を拒否した |
| `Update request: unknown` | GitHub が更新を受け付けたか分からない |
| `Desired state: matches` | 読み直した値は設定ファイルのとおりだった |
| `Desired state: does not match` | 読み直した値が設定ファイルのとおりでなかった |
| `Desired state: not confirmed (read failed)` | 更新のあとで値を読み直せず、設定ファイルのとおりになったか確かめられなかった。読み直せなかった原因は、結果のあとに出るエラーの行に出る |
| `Desired state: not confirmed (repository ID changed)` | 読み直したリポジトリが、承認したときとは別のリポジトリだった。同じ名前のリポジトリが、削除して作り直すなどして入れ替わっている |
| `Desired state: not in effect (<理由>)` | 書いた設定が GitHub で効いていなかった。括弧の中は、Actions が無効になっている（`GitHub Actions is disabled`）か、`allowed_actions` が `selected` でない（`allowed_actions is not "selected"`）か、リポジトリがパブリックになっている（`the repository is public`）こと |

次の順に進めます。

1. 結果のあとに出るエラーの行と、`failed` の下の行から、止まった原因を確かめる。エラーの行の内容に応じて、このページの該当するセクションで原因を取り除く
2. `plan` を実行し、残っている変更を確かめる。完了した変更は、変更計画に出ない
3. `apply` を実行し直して、残りの変更を適用する

### 自動マージの設定が反映されない

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: does not match
    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it
```

GitHub Free のアカウントが所有するプライベートリポジトリでは、自動マージを使えません。GitHub は更新を受け付けても、値を変えないことがあります。そのようなリポジトリには、`allow_auto_merge: true` を書かないようにします。

## そのほかのエラーと行

| 見えたもの | 次にすること |
|---|---|
| `rate limit exceeded` | GitHub の API の利用回数の上限を超えた。少なくとも1分待ってから実行し直す。同じエラーが続くなら、さらに間隔を空ける |
| `communication failed`、`unexpected response (HTTP 5xx)` | 通信の失敗か、GitHub 側の障害。時間をおいて実行し直す |
| `redirect not followed` | リポジトリの名前が変わったか、移転した可能性がある。設定ファイルの `name` と `--repo` に、今の名前を書く |
| `Precondition: ... changed since approval` | 承認したあとに、その設定の値が変わった。`plan` で今の値を確かめてから、`apply` し直す |
| そのほかの `Precondition:` | 更新を始める前の確認ができなかったか、一つ前の更新のあとの値が予定した値と違った。エラーの行があれば原因を取り除き、`plan` で確かめてから `apply` し直す |
| `Other settings: ... (cause not determined)` | 変えようとしていない設定が、更新の前後で変わっていた。誰かが変えたものかを確かめ、`plan` で今の値を確かめる |

## References

- [CLI 仕様](../spec/cli.md)：エラーの形式と各エラーの示し方（2章、3章、7章）、適用できない変更の表示と `apply` が止まること（4章、5章）、承認（5.1節）、変更結果の表示と各行（5.2節、5.3節）
- [要求定義](../spec/requirements.md)：適用できない変更をまとめて報告すること（FR-008）、リダイレクトに従わないこと（FR-004）、変更結果の報告（FR-013）、異常のあとに更新を続けないこと（INV-005）
- [GitHub REST API 共通仕様](../research/rest-api.md)：`404` だけでは不存在を断定できないこと、レート制限と待つべき時間
- [GitHub リポジトリ API 仕様](../research/repositories.md)：名前を変えたリポジトリへのリダイレクト、GitHub Free のプライベートリポジトリでの自動マージ
