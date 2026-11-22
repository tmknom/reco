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

理由は、変更計画の `Applicability:` の行の括弧の中にあります。セミコロンの前が成り立っていないこと、後ろが設定ファイルの直し方です。この例では Actions が無効のため、同じリポジトリに `enabled: true` か `allowed_actions` を書き足します。`Dependabot alerts is disabled` なら、同じリポジトリに `dependabot_alerts: true` を書き足します。`Secret scanning is disabled` なら `secret_scanning: true` を書き足し、`the repository is private` なら、そのリポジトリから、セミコロンの後ろに示されたキー（`secret_scanning` など）を消します。`no merge method would remain enabled` なら、セミコロンの後ろに示されたキー（`allow_rebase_merge` など）のどれかを `true` と書き足します。設定ファイルを直したら、`plan` で `Plan cannot be applied.` が消えたことを確かめ、`apply` を実行し直します。

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

Note: Some changes may already have been applied. No automatic rollback was performed. Run reco plan to see the remaining changes.
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
| `Desired state: not confirmed (repository ID changed)` | 読み直したリポジトリが、承認したときとは別のリポジトリだった。同じ名前のリポジトリが、削除して作り直すなどして入れ替わった可能性がある |
| `Desired state: not in effect (<理由>)` | 書いた設定が GitHub で効いていなかった。括弧の中は効いていない理由（`GitHub Actions is disabled` など） |

次の順に進めます。

1. 結果のあとに出るエラーの行と、`failed` の下の行から、止まった原因を確かめる。エラーの行の内容に応じて、このページの該当するセクションで原因を取り除く
2. `plan` を実行し、残っている変更を確かめる。完了した変更は、変更計画に出ない
3. `apply` を実行し直して、残りの変更を適用する

### 一つの変更の途中で止まった

一つの変更が複数の更新に分かれるときは、`Update request (...)` の行が更新の順に並びます。`allowed_actions` を `selected` に変えて `selected_actions` も変える変更と、`mode: selected` の Environment でパターンを取り除くか加える変更が当たります。Environment の作成や、`mode` を `selected` に変える更新は、その変更の最初の更新として並びます。

```text
Applying changes...
  example/project-a: failed
    Update request (environments.production.deployment_branch_policy): succeeded
    Update request (patterns to add [branch "main"]): succeeded
    Update request (patterns to add [branch "release/*"]): rejected
    Update request (patterns to add [tag "v*"]): not started
    Desired state: does not match
```

| 行 | 意味 |
|---|---|
| `Update request (<設定>): ...` | その設定を変える更新の結果 |
| `Update request (patterns to remove [...]): ...`、`Update request (patterns to add [...]): ...` | 括弧の中のパターンを取り除く更新と、加える更新の結果。取り除く更新どうし、または加える更新どうしで、同じ結果の更新が続くと、1行にまとめて並べる |
| `not started` | その更新を始めなかった。前の更新が完了しなかったか、すぐ下の `Precondition:` の行の理由で始められなかった |

この例では、`mode` を `selected` に変えて `main` のパターンまで加え、`release/*` を加える更新が拒否されて止まっています。`v*` はまだ加えていません。

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
| `redirect not followed`（下の `HTTP 303` の行を除く） | リポジトリの名前が変わったか、移転した可能性がある。設定ファイルの `name` と `--repo` に、今の名前を書く |
| `redirect not followed (HTTP 303): GitHub returns this when the pattern already exists` | Environment に加えようとしたパターンが、GitHub 上にすでにあった。`plan` で今のパターンを確かめてから、`apply` し直す |
| `Precondition: ... changed since approval` | 承認したあとに、その設定の値、Environment そのもの、変数のどれかが変わった。Environment そのものの変化は、作成する予定の名前の Environment が作られた、変える予定の Environment がなくなった、作り直されたなどの場合に当たる。`actions.variables.<名前>` では、作る予定の名前の変数が作られた、値を変える予定の変数の値が変わった、その変数がなくなった場合に当たる。`environments.<Environment の名前>.variables.<名前>` も同じで、同じ `apply` で作った Environment がそのあとでなくなった場合にも当たる。`plan` で今の値を確かめてから、`apply` し直す |
| そのほかの `Precondition:` | 更新を始める前の確認ができなかったか、一つ前の更新のあとの値が予定した値と違った。エラーの行があれば原因を取り除き、`plan` で確かめてから `apply` し直す |
| `request conflicts with the current state (HTTP 409)`（`create repository variable` か `create environment variable` のエラー） | 作ろうとした名前の変数が、GitHub 上にすでにある。更新の直前の確認のあとに作られた可能性がある。`plan` で今の値を確かめてから、`apply` し直す |
| `request not processable (HTTP 422)`（`create repository variable`、`update repository variable`、`create environment variable`、`update environment variable` のエラー） | GitHub が変数の作成か値の変更を受け付けなかった。値が上限（48,000 バイト）を超えたか、作成では変数の数が上限（リポジトリに500件、一つの Environment に100件）に達していた可能性がある。値が長すぎれば短くし、変数が多すぎれば GitHub 上の不要な変数を減らしてから `apply` し直す |
| `Other settings: ... (cause not determined)` | 変えようとしていない設定が、更新の前後で変わっていた。誰かが変えたものかを確かめ、`plan` で今の値を確かめる |

## References

- [CLI 仕様](../spec/cli.md)：エラーの形式と各エラーの示し方（2章、3章、7章）、適用できない変更の表示と `apply` が止まること（4章、5章）、承認（5.1節）、変更結果の表示と各行（5.2節、5.3節）
- [要求定義](../spec/requirements.md)：適用できない変更をまとめて報告すること（FR-008）、リダイレクトに従わないこと（FR-004）、変更結果の報告（FR-013）、異常のあとに更新を続けないこと（INV-005）
- [GitHub REST API 共通仕様](../research/rest-api.md)：`404` だけでは不存在を断定できないこと、レート制限と待つべき時間
- [GitHub リポジトリ API 仕様](../research/repositories.md)：名前を変えたリポジトリへのリダイレクト、GitHub Free のプライベートリポジトリでの自動マージ
