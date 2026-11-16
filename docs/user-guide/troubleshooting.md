# 困ったときは

reco がエラーで止まったときや、`apply` の結果が期待と違ったときに、原因の候補と次にすることを、見えた症状ごとに示します。出力の各行の意味は [変更計画と変更結果の読み方](reading-output.md) に、用語の意味は [用語集](glossary.md) にあります。

## 症状の一覧

| 見えたもの | セクション |
|---|---|
| `load configuration`、`is not declared`、`GH_TOKEN or GITHUB_TOKEN must be set` を含むエラー | [GitHub に問い合わせる前に止まる](#github-に問い合わせる前に止まる) |
| `HTTP 401`、`HTTP 403`、`HTTP 404`、`missing fields`、`rate limit exceeded`、`communication failed`、`unexpected response (HTTP 5xx)`（xx は数字）を含むエラー | [GitHub への問い合わせで止まる](#github-への問い合わせで止まる) |
| `redirect not followed` を含むエラー | [名前を変えたか移転したリポジトリ](#名前を変えたか移転したリポジトリ) |
| `apply cancelled: approval ...` のエラー | [承認できない](#承認できない) |
| `apply` の結果の `failed`、`not attempted`、`Note:` | [`apply` が途中で止まった](#apply-が途中で止まった) |
| `Desired state: does not match`、`Possible cause:` | [適用したのに反映されない](#適用したのに反映されない) |
| `Other settings:` | [変えていない設定が変わったと報告された](#変えていない設定が変わったと報告された) |
| `Precondition:` | [承認のあとで状況が変わったと報告された](#承認のあとで状況が変わったと報告された) |

## エラーの見方

reco のエラーは、標準エラー出力（stderr）に JSON の1行で出ます（[出力先](commands.md#出力先)）。`error` の値が、reco が何をしようとして、どの対象で、なぜ止まったかを示します。

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: authentication failed (HTTP 401)"}
```

この例では、`fetch repository example/project-a` が「`example/project-a` の設定を GitHub から読み取ろうとした」ことを、最後の `authentication failed (HTTP 401)` が理由を示します。

`apply` が途中で止まったときは、変更結果（stdout）のあとにエラーの行が出ます。示されるのは、設定ファイルに書いた順で最初に完了しなかった変更の原因です。

## GitHub に問い合わせる前に止まる

次のエラーは、reco が GitHub にアクセスする前に出ます。GitHub の設定は何も変わっていません。

### 設定ファイルが見つからない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: open reco.yaml: no such file or directory"}
```

- **原因の候補**：設定ファイルのない場所で実行した。`--config` のパスを誤った。コンテナイメージで `-w` を付けずに実行した（[コンテナイメージで動かす](installation.md#コンテナイメージで動かす)）
- **次にすること**：設定ファイルのあるディレクトリで実行するか、`--config` で設定ファイルのパスを指定する

### 設定ファイルの書き方の誤り

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"load configuration ./reco.yaml: repositories[0].general.allow_auto_merge (line 4, column 25): must be true or false (unquoted, lowercase); got \"true\""}
```

- **原因の候補**：エラーが示す行と列の書き方の誤り。出会いやすい誤りの例は [よくある書き方の誤り](configuration.md#よくある書き方の誤り) にある
- **次にすること**：示された箇所を直す。`--repo` で別のリポジトリに絞っていても止まる（[オプション](commands.md#オプション)）

### `--repo` のリポジトリが設定ファイルにない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"repository example/other is not declared"}
```

- **原因の候補**：`plan` か `apply` の `--repo` に、設定ファイルに書いていないリポジトリを指定した。名前の綴りを誤った
- **次にすること**：`--repo` の名前を設定ファイルの `name` と揃える。設定ファイルに書いていないリポジトリの設定を見るだけなら、`show` を使う

### トークンがない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"GH_TOKEN or GITHUB_TOKEN must be set to a non-empty token"}
```

- **原因の候補**：`GH_TOKEN` も `GITHUB_TOKEN` も設定されていないか、空である。コンテナイメージで `-e GH_TOKEN` を付けていない
- **次にすること**：[トークンを用意する](installation.md#トークンを用意する) の手順で `GH_TOKEN` にトークンを渡す

## GitHub への問い合わせで止まる

次のエラーは、GitHub から設定を読み取ろうとして失敗したときに出ます。`fetch repository` や `fetch actions permissions`、更新中なら `update repository` などに続く名前が、失敗したリポジトリです。

`plan` と `apply` は、読み取りに失敗したリポジトリが一つでもあれば、更新を一つも始めずに止まります（[全リポジトリを確かめてから更新を始める](safety.md#全リポジトリを確かめてから更新を始める)）。原因を取り除くまでほかのリポジトリだけを進めたいときは、`--repo` で絞れます（[一つのリポジトリに絞って実行する](operations.md#一つのリポジトリに絞って実行する)）。

`apply` の更新の途中で同じ理由の失敗が起きたときは、変更結果が `failed` か `not attempted` になります。そのときは、ここで原因を取り除いてから、[`apply` が途中で止まった](#apply-が途中で止まった) に進みます。

### `authentication failed (HTTP 401)`

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: authentication failed (HTTP 401)"}
```

- **原因の候補**：トークンが無効になっている（期限切れ、取り消し、値の誤り）。`GH_TOKEN` に古いトークンが残っている（`GH_TOKEN` が空でなければ reco はそれを使い、認証に失敗しても `GITHUB_TOKEN` に切り替えない。[認証情報の環境変数](commands.md#認証情報の環境変数)）
- **次にすること**：`gh auth token` でトークンを取り直し、`GH_TOKEN` に渡し直す。無効なトークンのまま実行を繰り返さない。GitHub は、無効な認証情報での要求が短時間に続くと、そのユーザーの認証を有効なトークンも含めて一時的に拒否し、`403` を返すことがある

### `missing fields`

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: unexpected response: missing fields: allow_auto_merge, delete_branch_on_merge"}
```

- **原因の候補**：例のように `allow_auto_merge` などの一般設定の値が欠けているなら、トークンの持ち主が、そのリポジトリで Write 以上のロールを持っていない
- **次にすること**：トークンの持ち主のロールを確かめる（[トークンを用意する](installation.md#トークンを用意する)）。reco で動作を確かめているのは、Admin ロールを持つ場合である

### `permission denied (HTTP 403)`

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch actions permissions example/project-a: permission denied (HTTP 403)"}
```

- **原因の候補**：トークンの持ち主のロールか、トークンの権限が足りない。無効なトークンでの実行を繰り返したために、GitHub が一時的に認証を拒否している。レート制限と判定できた拒否は、この文字列ではなく `rate limit exceeded` と示される
- **次にすること**：まずトークンの持ち主のロールを確かめる（[トークンを用意する](installation.md#トークンを用意する)）。Admin ロールを持っていれば、`gh auth token` で取ったトークンを渡しているかを確かめる（reco が主に想定する使い方）。無効なトークンでの実行を繰り返していたなら、トークンを取り直し、時間をおいてから実行する

### `not found or not accessible (HTTP 404)`

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: not found or not accessible (HTTP 404)"}
```

- **原因の候補**：トークンの持ち主がそのリポジトリを見られない。トークンの権限か、トークンでアクセスできるリポジトリの範囲が足りない。GitHub は、見る権限のない非公開リポジトリにも `404` を返すため、このエラーはリポジトリがないことを意味しない。名前の綴りを誤った
- **次にすること**：まずトークンの持ち主が、そのリポジトリにアクセスできるロールを持つかを確かめる（[トークンを用意する](installation.md#トークンを用意する)）。ロールが足りていれば、トークンの権限と、トークンでアクセスできるリポジトリの範囲を確かめる。最後に、設定ファイルの `name` か `--repo` の綴りを確かめる

### `rate limit exceeded`

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/project-a: rate limit exceeded (HTTP 429)"}
```

HTTP ステータスは、例の `429` のほか `403` のこともあります。

- **原因の候補**：GitHub の API の利用回数の上限（レート制限）を超えた
- **次にすること**：少なくとも1分待ってから実行し直す。GitHub が求める待ち時間はこれより長いことがあり、reco はその時刻を示さないため、同じエラーが続くなら、さらに間隔を空ける。レート制限で失敗した読み取りを、reco は自動でやり直さない。制限を超えたまま要求を続けると、GitHub に利用を禁止されることがある

### `communication failed` か、`unexpected response (HTTP 5xx)`

- **原因の候補**：通信の失敗か、GitHub 側の一時的な障害
- **次にすること**：時間をおいて実行し直す

## 名前を変えたか移転したリポジトリ

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"fetch repository example/old-name: redirect not followed (HTTP 301): the repository may have been renamed or transferred; specify its current name"}
```

- **原因の候補**：リポジトリの名前が変わったか、別のオーナーに移転された。HTTP ステータスは `301` のほか、更新中なら `307` のこともある。名前を変えたリポジトリの古い名前への要求に、GitHub はリダイレクトを返す。reco はリダイレクトに従わず、新しい名前も示さない（[リダイレクトに従わない](safety.md#リダイレクトに従わない)）
- **次にすること**：GitHub でリポジトリの今のオーナーと名前を確かめ、設定ファイルの `name` と `--repo` を書き直す。名前を変えた場合は、古い名前のリポジトリの URL をブラウザで開くと、GitHub は今の名前のページへ移す（古い名前で新しいリポジトリが作られていない場合）

## 承認できない

### 承認を求められずに終わる

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"apply cancelled: approval not available: standard input is not a terminal; no changes made"}
```

- **原因の候補**：標準入力が端末でない。パイプで入力を渡した、CI の中で実行した、コンテナイメージを `-it` なしで実行した、のどれかに当たる
- **次にすること**：端末から `apply` を実行する。コンテナイメージでは `-it` を付ける（[コンテナイメージで動かす](installation.md#コンテナイメージで動かす)）。CI の中では `apply` で設定を変えられない（[自動化に組み込むとき](commands.md#自動化に組み込むとき)）

### 入力したのに承認されない

```text
{"time":"...","level":"ERROR","msg":"command failed","error":"apply cancelled: approval declined; no changes made"}
```

- **原因の候補**：承認として受け付ける入力（[承認](commands.md#承認)）以外を入力した。`Y` のような大文字や、前後に空白のある入力もこれに当たる
- **次にすること**：もう一度 `apply` を実行し、小文字の `y` か `yes` だけを入力する。`no changes made` のとおり、GitHub の設定は変わっていない

## `apply` が途中で止まった

`apply` の結果に `failed` か `not attempted` があり、終了コードが `1` のときです。結果の最後に `Note:` があれば、その `apply` の変更の一部がすでに GitHub に反映されている可能性があります。reco は、反映した変更を元に戻しません（[reco が引き受けないこと](safety.md#reco-が引き受けないこと)）。

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

次の順に進めます。

1. 変更結果の行から、どこまで適用されたかを読む（[`apply` の結果の区分](reading-output.md#apply-の結果の区分)）
2. stderr のエラーの行と、`failed` の変更の理由の行から、止まった原因を確かめ、このページの該当するセクションで取り除く
3. `plan` を実行し、残っている変更予定を確かめる。完了した変更は、ほかの手段で変えられていなければ変更計画に出ない。`Update request` に `unknown`（成否不明）があれば、その更新が GitHub に反映されたかを reco は知らないため、この `plan` で今の値を確かめてから次へ進む
4. 残りの変更を適用するなら、`apply` を実行し直す（[リポジトリを並べる順序](operations.md#リポジトリを並べる順序)）。途中で止まった `apply` で反映された変更を元に戻したいなら、[変えた値を戻す](operations.md#変えた値を戻す) の手順で元の値を書いて `apply` する

`Update request (actions.permissions.allowed_actions): succeeded` のあとに、二つ目の `Update request` が `succeeded` 以外で続くときは、GitHub に中間状態が残っている可能性があります（[二つの更新操作からなる変更の途中で止まると、中間状態が残る](safety.md#二つの更新操作からなる変更の途中で止まると中間状態が残る)）。上の手順 3 と 4 で `apply` し直すと、残りの更新が行われます。GitHub の設定画面で直してもかまいません。

## 適用したのに反映されない

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: does not match
    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it
```

GitHub は更新を受け付けたのに、reco が読み取り直した値が期待値になっていなかったときの表示です。`Possible cause:` の行の意味は [原因を断定しない行](reading-output.md#原因を断定しない行) にあります。

- **原因の候補**：`Possible cause:` の行が示すとおり、GitHub Free のアカウントが所有する非公開リポジトリで `allow_auto_merge: true` を書いた（[リポジトリの一般設定](configuration.md#リポジトリの一般設定)）。GitHub への反映が遅れている。reco は約30秒まで確かめ直してから `failed` にするため、反映の遅れで `does not match` になることは多くない
- **次にすること**：少し待ってから `plan` を実行する。変更予定が出なければ、遅れて反映されている。同じ変更予定が出続け、`Possible cause:` の条件に当たるリポジトリなら、そのリポジトリでは GitHub が値を変えない可能性が高いため、設定ファイルの値を見直す

## 変えていない設定が変わったと報告された

```text
Applying changes...
  example/project-a: failed
    Update request: succeeded
    Desired state: matches
    Other settings: actions.permissions.sha_pinning_required changed false -> true (cause not determined)
```

`Other settings:` の行は、変更対象外の設定の予期しない変化を示します。変えようとした設定が期待値になっていても（`Desired state: matches`）、この行があれば `failed` になります。行の意味は [原因を断定しない行](reading-output.md#原因を断定しない行) にあります。

- **原因の候補**：ほかの人や自動化が、更新の前後のあいだにその設定を変えた。reco の更新に伴って変わった可能性もあり、reco は原因を断定しない
- **次にすること**：その変化が意図したものか（誰かが変えたのか）を確かめる。そのあと `plan` を実行し、残っている変更予定を確かめる。変わった設定が管理設定なら、変更計画に差分として出るため、設定ファイルと GitHub のどちらの値を残すかを決める。`actions.permissions.sha_pinning_required` のように reco の設定ファイルに書けない設定は reco では戻せないため、戻すなら GitHub の設定画面で直す。同時の変更を避ける手順は [ほかの人や自動化と同時に変えるとき](operations.md#ほかの人や自動化と同時に変えるとき) にある

## 承認のあとで状況が変わったと報告された

`Precondition:` の行は、更新操作の直前に、変更前提が成り立つと確かめられなかったことを示します（[更新を始めなかった理由](reading-output.md#更新を始めなかった理由)）。

### `changed since approval`

```text
Applying changes...
  example/project-a: not attempted
    Precondition: general.allow_auto_merge changed since approval
```

- **原因の候補**：承認したあと、更新の直前までに、ほかの人や自動化がその設定を変えた
- **次にすること**：`plan` を実行し直し、今の値を見て、設定ファイルと GitHub のどちらの値を残すかを決める。この変更は更新操作を始めていない

### `is not the value expected after the preceding update`

```text
Applying changes...
  example/project-a: failed
    Update request (actions.permissions.allowed_actions): succeeded
    Update request (actions.permissions.selected_actions): not started
    Precondition: actions.permissions.allowed_actions is not the value expected after the preceding update
    Desired state: does not match
```

- **原因の候補**：二つの更新操作からなる変更で、一つ目の更新のあと、GitHub への反映が遅れた。ほかの人や自動化が、そのあいだに値を変えた。reco はどちらかを断定しない
- **次にすること**：GitHub には中間状態が残っている可能性がある。`plan` を実行し直し、残っている変更予定を確かめてから、`apply` し直す

### ほかの `Precondition:` の行

`changed since approval` と `is not the value expected after the preceding update` 以外の `Precondition:` の行は、次の二つです。

- **`not confirmed (read failed)`**：変更前提を確かめるための読み取りに失敗した。原因は stderr のエラーの行に出るため、[症状の一覧](#症状の一覧) で探す。更新操作のあいだの待ち時間に Ctrl-C で中断したときも同じ行が出るが、その場合は原因を取り除く必要はない
- **`repository ID differs from the approved repository`**：設定ファイルの `name` が指すリポジトリが、承認したときと別のものになっていた（たとえば、同じ名前でリポジトリが作り直された）。GitHub で、`name` が意図したリポジトリを指しているかを確かめる

どちらの場合も、そのあとは [`apply` が途中で止まった](#apply-が途中で止まった) の手順で進めます。

## References

- [CLI 仕様](../spec/cli.md)：オプションと設定ファイルの読み込みのエラー（2.2節）、`--repo` に指定できるリポジトリ（2.3節）、認証情報の選択と、`401`・`403`・`404`・レート制限の示し方、Write より下のロールで一般設定の値が欠けること（3章）、承認と、反映を約30秒まで確かめ直すこと（5.1節）、変更結果の表示と各行（5.2節）、異常時の表示例（5.3節）、エラーの形式、終了コード、リダイレクト（7章）
- [要求定義](../spec/requirements.md)：リダイレクトに従わないこと（FR-004）、変更結果の報告（FR-013）、成否不明の更新を再送しないこと（INV-005）
- [GitHub REST API 共通仕様](../research/rest-api.md)：`404` だけでは不存在を断定できないこと、無効な認証情報を繰り返したときの `403`、レート制限と待つべき時間
- [GitHub リポジトリ API 仕様](../research/repositories.md)：名前を変えたリポジトリへのリダイレクト（`301` と `307`）、GitHub Free の非公開リポジトリでの自動マージ
