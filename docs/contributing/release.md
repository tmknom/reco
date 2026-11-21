# リリースワークフロー設計

reco を、Git タグのプッシュを起点に GitHub Releases と GitHub Container Registry（GHCR）へ公開するワークフローの設計です。
対象はワークフロー `.github/workflows/release.yml` で、各ジョブの責務、権限、成果物の受け渡し、検証の条件、失敗時の扱いを定めます。

リポジトリは `tmknom/reco`、CLI の名前は `reco` です。
CLI の名前は、`release.yml` の `APP_NAME` と `.goreleaser.yaml` の `project_name` に書いた値です。

## 対象と前提

| 項目 | 方針 |
|---|---|
| リポジトリ | 個人アカウントが所有する Public リポジトリ。起動者の制限（[リリースの起点とバージョン](#リリースの起点とバージョン)）は、所有者がユーザーであることを前提とする |
| 実行基盤 | GitHub-hosted runners |
| 配布する CLI | リポジトリ直下の main パッケージ。バイナリ名は `reco` |
| アーティファクト | Linux / macOS / Windows × amd64 / arm64 の 6 種類の実行ファイルと、バイナリごとの SBOM（SPDX JSON）。アーカイブにはせず、バイナリをそのまま配布する |
| コンテナ | `linux/amd64`、`linux/arm64` のマルチアーキテクチャイメージ `ghcr.io/tmknom/reco` |
| リリースの起点 | ローカルで作成してプッシュした `vX.Y.Z` 形式の Git タグ（[タグの作成](#タグの作成)） |
| 認証 | `GITHUB_TOKEN` と GitHub Actions の OIDC だけを使う。長期保管する秘密鍵や GitHub App は導入しない |
| cgo | `CGO_ENABLED=0` で固定する |

deb / rpm / apk の OS パッケージと Homebrew Formula は、現時点では配布しません。

## 保証する性質と保証しない内容

リリースの各工程は、次の性質を確保するために置いています。
「証明を生成したこと」と「期待する生成元・対象を検証したこと」は別の条件として扱い、後者を公開の条件にします。

| 性質 | 保証・検証する内容 | 保証しない内容 |
|---|---|---|
| 完全性 | 実行ファイルと SBOM の SHA-256、OCI イメージの digest が期待値に一致する | ハッシュ値だけで期待値の入手元を信頼できること |
| 来歴 | 成果物の digest と、リポジトリ、タグ、コミット、署名したワークフローの対応が署名で検証できる | 実行された処理やソースコードが安全であること、SBOM の記載内容が正確であること |
| 機能品質 | テストが成功する。linux_amd64 の実行ファイルと、amd64 / arm64 のコンテナで、`--version` の出力がタグと一致する | 全障害・全欠陥の不存在、Linux 以外の OS での実行 |
| 公開の一貫性 | GitHub Release と GHCR のイメージを `container-image.txt` の digest で結び付け、公開後に照合する | GitHub Releases と GHCR の原子的な同時公開 |

再現可能性（同じ入力から同じバイト列を再生成できること）と既知の脆弱性の検査は、現時点の保証に含めません。
扱いは [将来の拡張](#将来の拡張) に記載します。

Linux 以外の OS でのバイナリの実行も、リリースの工程では確かめません。
`CGO_ENABLED=0` でクロスコンパイルするため、OS ごとの挙動の違いはソースコードの性質であり、リリースの工程が持ち込むものではないからです。
OS ごとの確認は、プルリクエストで実行する通常 CI の役割とします（[将来の拡張](#将来の拡張)）。

SLSA の Build Level を名乗ることは目的にしません。

## リリースの起点とバージョン

リリースは、`v[0-9]+.[0-9]+.[0-9]+` に一致するタグの `push` で始まります。
`v0` のような形式外のタグやプレリリースのタグでは、ワークフローが起動しません。

- **バージョンの正本**：Git タグである。ビルドはタグが指すコミット SHA（`GITHUB_SHA`）から行い、実行中に `main` の最新状態を解決し直さない
- **起動者の制限**：すべてのジョブの `if` で `github.actor_id == github.repository_owner_id` と `github.triggering_actor == github.repository_owner` の両方を要求する。`github.actor` は再実行しても最初の実行者のままで、再実行はその権限で動くため、再実行した者を `github.triggering_actor` で確かめる。`triggering_actor` には ID がないため、ユーザー名で比べる。最初のジョブだけに置かないのは、「Re-run failed jobs」では成功済みのジョブが実行し直されず、その `if` も評価し直されないためである。必須なのは書き込みの権限を持つジョブ（attest、container、release）で、読み取り専用のジョブにも置くのは、ジョブごとに要否を判断せずに済む規則にするためである
- **タグの移動**：公開物を残したまま、タグを別のコミットへ移動しない。ソースを直す場合は新しいバージョンを使うか、[失敗時のやり直し](#失敗時のやり直し) の手順で公開物を削除してから作り直す
- **正式リリースの完了点**：GitHub Release の公開である。タグや GHCR のイメージが存在しても、GitHub Release が公開されるまではリリース済みと扱わない
- **ワークフローの成功**：post-verify までのすべてのジョブが成功した状態である。正式リリースの完了点とは別に判定するため、post-verify が失敗した場合は「公開済みだが、公開後の照合に失敗した」状態になる

CLI は `--version` で、ビルド時に `-X main.version=<バージョン>` で埋め込んだ値だけを1行で出力します（出力の形式は [CLI](../spec/cli.md) が定めます）。
GoReleaser が埋め込む値は先頭の `v` を除いた形（`1.2.3`）で、実行ファイルのファイル名に入るバージョンと同じです。
ワークフローはこの出力を、ビルドしたバイナリにタグのバージョンが埋め込まれたことの確認に使います（build で linux_amd64 の実行ファイル、verify-image でイメージ）。

アプリケーション名とバージョンは、ワークフローの `env` に1回だけ書き、各ジョブはそれを参照します。
preflight の outputs で後続のジョブへ渡す方法は採りません。
outputs は `needs` に直接書いたジョブからしか参照できず、全ジョブの `needs` に preflight を加えることになるためです。
また、`github.ref_name` は実行中に変わらないため、preflight を通しても値の信頼性は増さないためです。

## タグの作成

タグは、リポジトリの所有者が手元で `make release` を実行して作ります。
`make release` は `scripts/release/tag.sh` を実行し、次の順に進みます。

1. 作業ツリーに未コミットの変更がなく、現在のブランチが `main` で、`HEAD` が `origin` の `main` と一致することを確かめる
2. `origin` のタグのうち `vX.Y.Z` 形式で最大のものを、現在のバージョンとする。該当するタグがなければ `v0.0.0` とする
3. bump level（`patch`、`minor`、`major`）を対話で選ぶ。`major` は追加で確認する
4. 次のバージョンを示して確認を得てから、注釈付きタグを作り、`origin` へプッシュする
5. `gh` があれば、起動したリリースワークフローの実行をブラウザで開く

現在のバージョンは、ローカルのタグではなく `origin` のタグから決めます。
preflight が同じバージョンの公開物の有無を判定するのは `origin` 側で、[失敗時のやり直し](#失敗時のやり直し) で削除し忘れたローカルのタグを基準にすると、バージョンを飛ばしてしまうためです。
タグの一覧を取得できなかった場合は、「タグがない」とは扱わずに止めます。
そう扱うと、既存のリリースがあっても `v0.0.1` を提案してしまうためです。
次のバージョンと同じ名前のローカルのタグが残っている場合も止めます。

タグの作成をワークフローに任せることはしません。
`GITHUB_TOKEN` でプッシュしたタグはワークフローを起動しないため、起動させるには PAT や GitHub App のトークンが要り、[対象と前提](#対象と前提) の認証の方針に反します。
`GITHUB_TOKEN` でも起動できる `workflow_dispatch` は、起動者がリポジトリの所有者ではなく GitHub Actions の bot になり、[リリースの起点とバージョン](#リリースの起点とバージョン) の起動者の制限で止まります。
`workflow_call` で呼び出す方法では、`github.ref` が呼び出し元のブランチになり、Attestation のソースの ref が変わるため、[利用者による検証](#利用者による検証) の `--source-ref` が通らなくなります。

## ジョブ構成

```text
push refs/tags/vX.Y.Z
    │
    ▼
preflight ── タグが main の履歴にあるか、同じバージョンの公開物がないか
    │
    ▼
test ── go mod verify / go vet / go test
    │
    ▼
build ── GoReleaser で実行ファイル 6 件、SBOM 6 件、SHA256SUMS を生成し、ビルド設定とバージョンを確認
    │
    ▼
attest ── 実行ファイルと SBOM の Artifact Attestation
    │
    ▼
container ── Linux 向けの実行ファイルを OCI 化して GHCR へ公開、証明
    │
    ▼
verify-image ── GHCR から digest 指定で取得し、証明の検証、バイナリ照合、実行
    │
    ▼
release ── 来歴を検証し、container-image.txt を作り、GitHub Release を公開
    │
    ▼
post-verify ── 公開済みのアセットと GHCR を取得し直して照合
```

取り消しにくい処理ほど後ろに置きます。
GHCR への公開は、ビルドと実行ファイルの確認、来歴証明が成功してから行い、GitHub Release の公開は、GHCR に公開したイメージの検証が成功してから行います。

| ジョブ | `needs` | `GITHUB_TOKEN` の権限 | ソースの Checkout |
|---|---|---|---|
| preflight | なし | `contents: read`、`packages: read` | あり（全履歴） |
| test | preflight | `contents: read` | あり |
| build | test | `contents: read` | あり（全履歴） |
| attest | build | `id-token: write`、`attestations: write` | なし |
| container | build、attest | `contents: read`、`packages: write`、`id-token: write`、`attestations: write` | あり（ベースイメージの参照だけを読む） |
| verify-image | container | `attestations: read` | なし |
| release | attest、container、verify-image | `contents: write`、`attestations: read` | なし |
| post-verify | container、release | `contents: read`、`attestations: read` | なし |

ワークフロー全体の `permissions` は `{}` とし、権限は各ジョブに明示します。
Action はトークンを引数で渡さなくても `github.token` を参照できる場合があるため、権限の付与そのものを絞ります。

権限の割り当ては、次の考え方に従います。

- **コードを実行するジョブ**（test、build）には、OIDC と公開の権限を与えない
- **署名するジョブ**（attest）は、Checkout もビルドもせず、受け取ったチェックサムに署名するだけにする
- **公開するジョブ**（container、release）では、リポジトリのコードを実行しない。container はベースイメージの参照を読むためだけに Checkout し、ビルドに使う Dockerfile はワークフロー定義の中で書き出す（[コンテナイメージ](#コンテナイメージ)）。release は Checkout せずに受け取ったファイルをデータとして登録する

## 各ジョブの責務

### preflight

リリースできない状態のタグで、テストやビルドの前に止めます。
タグの指すコミットが `main` の履歴にない場合と、同じバージョンの GitHub Release または GHCR のタグがすでにある場合に失敗します。

存在の確認では、確認できなかったことを「存在しない」と扱いません。
`gh` も Docker Buildx も、存在しない場合と認証や通信の障害の場合で同じ終了コードを返すため、終了コードだけで判定すると、障害を「存在しない」と誤判定して先へ進み、GHCR のタグを上書きしてしまいます。
そのため、エラーメッセージが「存在しない」ことを示す場合だけを「存在しない」とし、それ以外の失敗では止めます。

GitHub Release の確認は、Draft を検出しません。
GitHub の REST API は push の権限を持つ場合にしか Draft の Release を返さず、preflight の `contents: read` では取得できないためです。
Draft を見るためだけに preflight へ書き込みの権限は与えません。
前の実行の Draft が残っていると、GHCR への公開の後に release ジョブで止まるため、やり直す前に Draft も削除します（[失敗時のやり直し](#失敗時のやり直し)）。

GHCR は、非公開のパッケージのタグも検出できるよう、`packages: read` を与えてログインしてから確認します。
preflight はリポジトリのコードを実行せず、加えるのは読み取りの権限だけのため、公開の権限は持ちません。

preflight は「Re-run all jobs」では実行し直されますが、「Re-run failed jobs」では成功済みのため実行されません。
そのため、container も公開の直前に同じ判定で GHCR のタグの存在を確かめます（[container](#container)）。

### test

`go mod verify`、`go vet`、`go test` を、配布するバイナリと同じビルドの環境変数で実行します。

コード変更に対する包括的な品質の検証は、プルリクエストで動く通常 CI の役割です。
このジョブは、リリースするコミットそのものについて、重要な品質条件を通常 CI とは独立に確かめ直します。
タグはプルリクエストで検証したコミットとは別のコミットに付けられ得るため、通常 CI があってもこのジョブを残します。

### build

GoReleaser OSS で実行ファイル 6 件、SBOM 6 件、`SHA256SUMS` を生成し、`release-assets` という名前の Actions Artifact としてアップロードします。
このジョブの出力が、以降のすべてのジョブが扱う正規の成果物です。

GoReleaser の `archives` の `binary` 形式は、配布するバイナリを `dist/` 直下に置かず、ビルド先のパスのまま配布名を与えます。
そのため build は、`SHA256SUMS` に列挙された名前ごとに `dist/artifacts.json` からパスを1つだけ引き、配布名でアップロード用のディレクトリへ集めます。
名前に対応するパスが0件や複数件なら止めます。

アップロードの前に、集めたファイルについて次を確かめます。

- **ファイルの過不足**：`SHA256SUMS` の列挙が、タグのバージョンを含む期待どおりのバイナリと SBOM の名前と一致する
- **ビルド設定**：`go version -m` でバイナリに記録されたビルド設定を読み、`GOOS` と `GOARCH` がファイル名と一致し、`CGO_ENABLED=0` で、`.goreleaser.yaml` で固定した CPU の下限（`GOAMD64`、`GOARM64`）でビルドされている。配布名と中身の取り違えを、バイナリを実行せずに検出する
- **SBOM の対象**：各 SBOM が対象のバイナリの名前と SHA-256 を記録している。記載内容の正確さは確かめない
- **バージョンの埋め込み**：linux_amd64 のバイナリを実行し、`--version` の出力がタグと一致する。`-X` の変数名を誤ってもリンカは警告なしに無視するため、実行して確かめる
- **ディレクトリの中身**：バイナリの実行の後に、アップロードするディレクトリに配布するファイルと `SHA256SUMS` 以外がない。Upload はディレクトリごとアップロードするため、実行で生じたファイルも含めて確かめる
- **照合**：最後に `SHA256SUMS` で照合し、照合したファイルをそのままアップロードする

実行ファイルの確認を別ジョブにせず build に置くのは、アップロードするファイルそのものを確かめられ、build が OIDC も公開の権限も持たないためです。
test ジョブでは、GoReleaser が作ったバイナリがまだ存在しないため、`-X` が効いたかを確かめられません。

`--version` を公開前に実行して確かめるのは、linux_amd64 だけです。
linux_arm64 は verify-image が確かめますが、GHCR への公開後になります。
darwin と windows は、`ldflags` が全ターゲットで共通であるという設定上の前提に頼ります。
`.goreleaser.yaml` の `builds` にターゲット別の上書きを加えると、この前提は崩れます。

### attest

`actions/attest` に `subject-checksums` として `SHA256SUMS` を渡し、`SHA256SUMS` に列挙された実行ファイル 6 件と SBOM 6 件に SLSA Build Provenance の Artifact Attestation を付けます。
`SHA256SUMS` 自体は subject に含まれません。

`actions/attest` は predicate の入力を省いたときに Build Provenance を作るため、入力は省きます。
既定の挙動が将来変わっても公開しないよう、検証する側（container、verify-image、release）の `gh attestation verify` に `--predicate-type https://slsa.dev/provenance/v1` を明示します。

署名するジョブを build と分けるのは、GoReleaser やビルド中に動くコードに OIDC の権限を渡さないためです。
ただし、attest が署名するのは同じ実行の build がアップロードした Artifact の中身であり、build の実行環境が侵害されていればその出力にも署名が付きます。
この限界は、build と attest を1ジョブにまとめた場合と変わりません。

### container

Linux 向けの実行ファイル 2 件からマルチアーキテクチャのイメージをビルドし、GHCR に `vX.Y.Z` のタグで公開します。
公開した Image Index の digest に Artifact Attestation を付け、digest 付きの参照を後続のジョブへ渡します。

GHCR への公開も取り消しにくいため、[release](#release) と同じく、来歴の検証を公開の条件にします。
検証するのは、イメージに格納する Linux 向けの実行ファイル 2 件だけです。

公開の直前にも、同じバージョンのタグがないことを preflight と同じ判定で確かめます。
これは、公開後の Attestation の生成などでこのジョブが失敗したとき、「Re-run failed jobs」で同じタグへ再び公開するのを防ぐためです。
BuildKit が既定で付ける Provenance にはビルドの時刻などが記録されるため、再び公開すると Image Index の digest が変わり、タグが別のイメージを指す可能性があります。

公開した後に Attestation の生成で失敗すると、Attestation のないイメージがタグ付きで GHCR に残ります。
再実行はこの確認で止まるため、そのイメージを再利用して証明を付け直すことはしません。
GitHub Release は公開されておらず、このイメージを参照する `container-image.txt` もないため、GHCR のバージョンを削除するか、新しいバージョンでやり直します。

イメージの設計は [コンテナイメージ](#コンテナイメージ)、証明の設計は [来歴証明](#来歴証明) に記載します。

### verify-image

GHCR に公開したイメージを、container の出力した digest 付きの参照で取得して検証します。
amd64 と arm64 の runner で1回ずつ実行し、エミュレーションではなく、それぞれの CPU でイメージを実行します。
来歴とバイナリの一致を確かめてから、イメージを実行します。

- **来歴**：Image Index の digest の Attestation が、期待する predicate type、リポジトリ、署名したワークフロー、タグ、コミット SHA で検証できる
- **バイナリ**：イメージ内のバイナリが、Linux 向けの実行ファイルとバイト単位で一致する
- **実行ユーザー**：`nonroot:nonroot` である
- **バージョン**：`--version` の出力がタグと一致する

GHCR にはログインせず、匿名で取得します。
パッケージが Public であることが前提で、`packages: read` も与えません。
パッケージが非公開のままだと取得に失敗し、GitHub Release は公開されません。

### release

受け取った成果物と公開先の状態を検証してから、GitHub Release を公開します。
アセットとして登録するのは、`SHA256SUMS` に列挙されたバイナリ 6 件と SBOM 6 件、`SHA256SUMS`、container の出力した参照を1行で書いた `container-image.txt` だけです。

公開するバイナリと SBOM は、`SHA256SUMS` との照合に加えて来歴を検証します。
`SHA256SUMS` との照合だけでは、バイナリと `SHA256SUMS` が一緒に差し替えられた場合を検出できないためです。
期待値は、predicate type、リポジトリ、署名したワークフロー、タグ、コミット SHA で、署名された subject の名前がファイル名と一致することも確かめます。
検証は実行 ID までは照合しないため、同じコミットとタグで実行した過去の実行の署名も受け入れます。

公開の直前には、タグがビルドしたコミットを指したままであることを確かめます。
Release はタグの指すコミットに付くため、タグが付け替えられていればビルドと異なるコミットに付いてしまうからです。
確かめた直後の付け替えまでは防げないため、post-verify が公開後にもう一度確かめます。

`gh release create` は `make_latest` を指定しないため、REST API の既定値 `true` により、公開した Release はバージョンの大小にかかわらず「Latest」になります。
そのため、大きいバージョンを公開した後に小さいバージョンを公開すると（失敗した古い実行を後から再実行した場合など）、小さいほうが「Latest」を奪います。
REST API は `make_latest` に `true`、`false`、`legacy` を定義していますが、動作確認では、`make_latest: false` を渡しても、公開した小さいバージョンが「Latest」になりました。
`legacy` は確かめていません。
防ぐには、公開の後に元の「Latest」へ `make_latest: true` を付け直す必要があります。

この付け直しは、ワークフローでは行いません。
古いバージョンを後から公開する場面はまれで、起きても「Latest」を手動で付け直せば済みます。
一方で自動化すると、CLI のリリースが、その実行で公開していない Release を書き換えることになります。
代わりに、公開するバージョンが現在の「Latest」より `sort -V` で小さければ、警告を出します。
警告のための確認なので、「Latest」を取得できなくても公開は止めません。

Draft を確認してから公開する独立した工程は設けません。
公開するファイルと公開の前提は公開の前に検証を終えており、アップロードされたアセットの照合は post-verify が公開後に行うためです。
ただし、アセットを指定した `gh release create` は内部で Draft を作ってから公開するため、途中で失敗すると、アセットの一部だけを持つ Draft が残る可能性があります。
そのため、公開の前に Draft を含めて同じタグの Release がないことを確かめ、既存の Release を書き換えないようにします。
post-verify も、Release が Draft でないことを確かめます。

### post-verify

公開した GitHub Release と GHCR のイメージを取得し直し、ワークフローが生成した正規の成果物と照合します。
このジョブの成功をもって、ワークフローの成功とします（[リリースの起点とバージョン](#リリースの起点とバージョン)）。

- **公開の状態**：Release が Draft ではなく、タグが `GITHUB_SHA` を指したままである
- **アセット**：取得したアセットが、正規の成果物に `container-image.txt` を加えたものと過不足なく一致する
- **イメージ**：GHCR の `vX.Y.Z` タグが、`container-image.txt` に記録した digest を指している
- **利用者の手順**：取得したアセットと `container-image.txt` の参照で、[利用者による検証](#利用者による検証) と同じコマンドが成功する

verify-image と同じく、GHCR にはログインせず、`packages: read` も与えません。

このジョブが失敗した時点で、GitHub Release はすでに公開されています。
自動では取り消さず、失敗の内容を見てから削除してやり直すかを判断します。
`release-assets` の保持期間は 1 日のため、それを過ぎると post-verify だけを再実行できません。
その場合は [失敗時のやり直し](#失敗時のやり直し) の手順で全体をやり直します。

## 成果物とジョブ間の受け渡し

GitHub Release のアセットは、次の 14 件です。

| アセット | 内容 | 生成するジョブ |
|---|---|---|
| `reco_<バージョン>_<os>_<arch>` | Linux、macOS 向けの実行ファイル。4 件 | build |
| `reco_<バージョン>_windows_<arch>.exe` | Windows 向けの実行ファイル。2 件 | build |
| `<実行ファイル名>.sbom.json` | 実行ファイルごとの SBOM（SPDX JSON）。6 件 | build |
| `SHA256SUMS` | 実行ファイル 6 件と SBOM 6 件の SHA-256 | build |
| `container-image.txt` | GHCR のイメージの digest 付きの参照。1行 | release |

`<バージョン>` は先頭の `v` を除いた形です。
`LICENSE` はアセットに含めません。

HTTP でダウンロードしたファイルには実行権限が付かないため、Linux と macOS の利用者は、ダウンロードしたバイナリに実行権限を付けてから使います（利用者向けの手順は `install -m 0755` で付ける）。

ジョブ間で受け渡す Actions Artifact は `release-assets` の1つだけで、中身は実行ファイル 6 件、SBOM 6 件、`SHA256SUMS` です。
コンテナに格納する Linux バイナリは、container がこの Artifact の実行ファイルをそのまま使います。
アーティファクトと同じバイト列のため、実行ファイルを `SHA256SUMS` と Artifact Attestation で検証すれば足ります。
Actions Artifact はファイルの実行権限を保持しないため、後続のジョブはバイナリを実行せず、イメージには Dockerfile の `COPY --chmod=0755` で実行権限を付けて格納します。

イメージの参照（イメージ名、タグ付きの参照、digest 付きの参照）は、それぞれ1か所で定め、各ジョブでは組み立てずに受け取ります。
digest だけを渡して各ジョブがイメージ名と連結すると、連結の誤りで `container-image.txt` が別のイメージを指しても、公開するまで検出できないためです。
digest 付きの参照は container が一度だけ作り、verify-image、release、post-verify は同じ値を使います。

`actions/download-artifact` の転送単位の digest 検査とは別に、GitHub Release の公開より前に成果物を受け取るジョブ（attest、container、verify-image、release）で `sha256sum -c SHA256SUMS` を実行します。
転送の検査は Artifact の破損を検出するものであり、公開の停止条件はファイル単位の照合と署名の検証に置くためです。

## ビルドの設定

Go のビルドとパッケージングは GoReleaser OSS に任せ、担当範囲をバイナリ、SBOM、チェックサムの生成までに限ります。

| 処理 | 担当 |
|---|---|
| Go のクロスコンパイル | GoReleaser OSS |
| 配布名の付与 | GoReleaser OSS の `archives`（`binary` 形式） |
| SBOM の生成 | GoReleaser OSS の `sboms` が呼び出す Syft |
| `SHA256SUMS` の生成 | GoReleaser OSS の `checksum` |
| コンテナイメージの生成と GHCR への公開 | Docker Buildx（GoReleaser の Docker 機能は使わない） |
| 来歴証明 | GitHub Artifact Attestations（GoReleaser の署名機能は使わない） |
| GitHub Release の公開とリリースノート | GitHub CLI（GoReleaser の公開機能と changelog は使わない） |

GoReleaser は runner の環境変数を引き継ぐため、ビルドに関わる環境変数は `.goreleaser.yaml` で既定値でも明示し、その値が runner の設定に左右されないようにします。
値と理由は `.goreleaser.yaml` のコメントに書いています。
ツールチェーンや依存モジュールなど、環境変数以外のビルドの入力は、それぞれ別の方法で固定します（Go のバージョンは後述、依存モジュールは `go.sum`）。

SBOM には生成時刻と実行ごとに変わる文書の識別子が入るため、同じ入力から生成し直しても同じバイト列になりません。
SBOM の記載内容は、実行ファイルに埋め込まれたビルド情報から Syft が読み取ったもので、正確さはリリースの工程で確かめません。

`before.hooks` は置きません。
`go generate` や `go mod tidy` をリリース中に実行すると、ソースや依存の定義がビルドの途中で変わり得るためです。
依存の整合性はテストの段階で `go mod verify` が確かめます。

Go のバージョンは `go.mod` で固定し、`actions/setup-go` に `go-version-file: go.mod` を渡します。
`actions/setup-go` は、`toolchain` 行があればその値を、なければ `go` 行の値を使います。
reco の `go.mod` には `toolchain` 行があるため、ビルドに使うバージョンは `toolchain` 行の値です。
`toolchain` 行を削除するときは、`go` 行をパッチバージョンまで書きます（`1.27.0` など）。
パッチバージョンを省くと、`actions/setup-go` が利用できるパッチバージョンを解決するため、固定が成り立ちません。
`GOTOOLCHAIN=local` により、ビルド中に別のツールチェーンを取得しません。
`actions/setup-go` のキャッシュは、同じリポジトリの他のワークフローも書き込めるため無効にします。

## コンテナイメージ

イメージを定義する Dockerfile は、container ジョブがワークフロー定義の中で書き出します。

- **Dockerfile をワークフロー定義に固定する**：リポジトリに置いた Dockerfile をビルドに使わない。Dockerfile はビルドの定義で、`RUN`、`syntax` ディレクティブ（BuildKit が指定されたフロントエンドのイメージを取得し、ビルドに使う）、URL や Git リポジトリを指定した `ADD`、イメージを参照する `COPY --from` を加えると、ビルド中に実行したコマンドの結果や外部から取得したファイルが、実行ファイルのような検証を経ずに署名付きのイメージに入る。container ジョブは公開と署名の権限を持つため、Dockerfile の内容を、もともと `main` の変更のレビューに依存しているワークフロー定義に置く。命令を検査する仕組みを持たずに済み、書き出す Dockerfile は `RUN` を使わないため、QEMU も要らない
- **ベースイメージ**：Distroless static の nonroot。非 cgo の Go バイナリが必要とすることの多い CA 証明書とタイムゾーン情報を含む。Image Index の digest で固定し、Dependabot で更新する。Dependabot が読めるよう、固定する値はリポジトリ直下の `Dockerfile` の `FROM` に置き、`.github/dependabot.yml` の `docker` の `directory` を `/` にする。このファイルはビルドに使わず、container ジョブは、コメント行と空行を除いて digest で固定した `FROM` の 1 行だけであり、イメージ名がワークフロー定義に書いた名前と一致することを確かめてから、digest だけを取り出す。名前もワークフロー定義に固定するのは、命令と同じく、別のベースイメージに変えると検証を経ない内容が署名付きのイメージに入るためである。別のベースイメージへ移るときは、ワークフロー定義の名前と固定用のファイルを同時に変える
- **Go を再コンパイルしない**：格納するのは Linux 向けの実行ファイルそのものである。イメージ内のバイナリはアーティファクトとバイト単位で一致する
- **実行ユーザーと作業ディレクトリ**：`USER nonroot:nonroot` と、バイナリを指す `ENTRYPOINT` だけを指定し、`WORKDIR` は指定しない。作業ディレクトリはベースイメージの `/home/nonroot` のままのため、利用者は設定ファイルのあるディレクトリをマウントし、`docker run -w` で作業ディレクトリに指定する必要がある
- **ビルドコンテキスト**：Linux バイナリ 2 つだけを置いた専用ディレクトリを渡す。ソースコードは Buildx に渡らない。Buildx の既定の Git コンテキストは、先行ステップで用意したファイルを反映しないため使わない
- **タグ**：`vX.Y.Z` だけを付け、`latest` は付けない。イメージの識別と検証には digest を使う
- **OCI ラベル**：値はワークフローの `env` と `github` コンテキストから取り、手で書かない。ラベルはプラットフォームごとのイメージの設定に入り、Image Index には入らない。GHCR のパッケージページは、マルチアーキテクチャのイメージでは Image Index の注釈を読むことがあるため、ラベルの値がパッケージページに表示されるとは限らない。`org.opencontainers.image.version` は、タグの値（`v1.2.3`）をそのまま使う。v を除いた値にするにはそのためのステップが要り、それに見合う利点がないため

GHCR へは `docker/login-action` に `GITHUB_TOKEN` を渡してログインします。
初回の公開直後のパッケージは非公開のため、パッケージの設定で公開範囲を Public に変え、リポジトリに紐づいていることを一度だけ確認します。
匿名で取得する verify-image は、公開範囲を変えるまで失敗するため、初回のリリースは verify-image で止まる見込みです。
公開範囲を変えてから「Re-run failed jobs」で再実行します（[失敗時のやり直し](#失敗時のやり直し) の verify-image の行）。
再実行は、build の完了から 1 日以内に行います。
それを過ぎると `release-assets` が消えて verify-image が失敗し、GHCR にイメージが残っているため全体のやり直しも preflight で止まります。
リポジトリが Public であることと、GHCR のパッケージが Public であることは別の設定です。

## 来歴証明

来歴証明には GitHub Artifact Attestations（`actions/attest`）を使い、SLSA Build Provenance を生成します。

| subject | 生成するジョブ | 保存先 |
|---|---|---|
| 実行ファイル 6 件と SBOM 6 件（`SHA256SUMS` に列挙されたもの） | attest | GitHub の Attestation API |
| GHCR の Image Index digest | container | GitHub の Attestation API と GHCR（`push-to-registry: true`） |

イメージの証明は、container ジョブの中で生成します。
レジストリへ証明を保存するには `packages: write` が要り、別ジョブにしても同じ権限を持つジョブが2つに増えるだけだからです。
subject はタグではなく digest にします。
タグは付け替えられるためです。

storage record（組織の linked artifacts ページに成果物の保存先を記録する機能）は作りません。
`artifact-metadata: write` を必要とし、作らなくても来歴の検証は変わらないためです。

Docker BuildKit が既定で付ける Provenance はそのまま残しますが、GitHub の署名が付いた来歴としては扱いません。
Public リポジトリでは BuildKit の Provenance にビルド引数が記録され得るため、ビルド引数に秘密情報を渡しません。

`SHA256SUMS` と `container-image.txt` は証明の subject に含めません。
この2ファイルには、現時点で署名による保護がありません。

## 公開の順序と部分成功

公開は次の順に進みます。

1. 正規の成果物を生成して中身とバージョンを確かめ、来歴証明を成功させる
2. GHCR にイメージを公開し、digest と来歴証明を確定する
3. GHCR から digest 指定で取得したイメージを検証する
4. `container-image.txt` を作り、GitHub Release を公開する
5. 公開済みのアセットと GHCR を取得し直して照合する

GitHub Releases と GHCR は原子的に公開できないため、手順 2 から 4 の間は GHCR にだけイメージがある状態になります。
この間に失敗した場合の部分成功は、避けられない状態として受け入れます。

## 失敗時のやり直し

ワークフローは、公開途中の状態から自動で回復しません。
また、公開途中で生成したものを自動で削除するロールバックも行いません。

| 失敗した位置 | 対応 |
|---|---|
| preflight、test、build、attest | 何も公開されていない。原因を直し、必要ならタグを削除して新しいコミットにタグを付け直す。ソースが変わらないなら失敗したジョブを再実行してよい |
| container | GHCR にイメージがある可能性がある。公開前に失敗したなら「Re-run failed jobs」で再実行してよい。公開後に失敗したなら、再実行は公開直前の確認で止まるため、GHCR のバージョンを削除してから再実行するか、新しいバージョンでやり直す |
| verify-image | GHCR にイメージがある。「Re-run failed jobs」では container が実行されず、公開済みのイメージをそのまま検証し直す。「Re-run all jobs」は preflight で止まるため、GHCR のバージョンを削除してから実行する |
| release 以降 | GitHub Release が公開されているか、Draft が残っている可能性がある。下記の手順で全体をやり直す。Draft も Release と同じく削除する |

attest 以降のジョブは `release-assets` を取得するため、「Re-run failed jobs」で再実行できるのは、build の完了から `release-assets` の保持期間（1 日）のあいだだけです。
過ぎた場合は、下記の手順で全体をやり直します。

同じバージョンを作り直すときは、GitHub Release（Draft を含む）、Git タグ、GHCR の該当バージョンを手動で削除してから、タグをプッシュし直します。
preflight は Draft を検出しないため、Draft が残っていると GHCR への公開の後に release ジョブで止まります。

```bash
gh release delete vX.Y.Z --yes
git push origin :refs/tags/vX.Y.Z
git tag -d vX.Y.Z
```

GHCR の該当バージョンは、パッケージの設定画面から削除します。
preflight は同じバージョンの Release と GHCR のタグがあると止まるため、一部だけ削除した状態では再実行できません。

`concurrency` は `group: release`、`cancel-in-progress: false` とし、実行中の公開を後続の実行が取り消さないようにします。
`queue` は既定の `single` のままのため、グループ内で待機できる実行は1つだけです。
待機中の実行は、後から待機に入った実行に置き換えられて取り消されます。
取り消された実行は、何も公開していないため、再実行してやり直します。
同時実行の制御では管理者の直接操作を防げないため、preflight の存在確認と post-verify の照合は省略しません。

## Immutable Release を採用しない運用

GitHub の Immutable Releases は、現時点では有効にしません。
有効にすると、公開済みの Release のアセットと関連する Git タグを変更、削除できなくなり、[失敗時のやり直し](#失敗時のやり直し) の手順で同じバージョンを作り直せなくなるためです。

この運用は、次の前提に立ちます。

- Git タグ、GitHub Release、GHCR のパッケージバージョンは、やり直しのために削除してよい
- 不整合な状態になった場合は、すべて削除して初めからやり直してよい
- Tag Rulesets によるタグの更新、削除の制限も導入しない
- Release Attestation が生成されないため、`gh release verify` と `gh release verify-asset` は使わない。公開後のアセットは post-verify が取得し直して照合する

この運用で失う保証は2つです。

- 公開後の Release のアセットと Git タグを、書き換えから保護すること
- `SHA256SUMS` と `container-image.txt` の内容を、署名で検証できるようにすること

実行ファイルとイメージには Artifact Attestation が付くため、それらの来歴の検証は Immutable Release がなくても成り立ちます。

## 利用者による検証

利用者は `gh attestation verify` で、アーティファクトの生成元を確かめられます。
Attestation は、検証する側が期待値を知っていて初めて意味を持つため、期待値を次のように定めます。

| 期待値 | 値 |
|---|---|
| リポジトリ | `tmknom/reco` |
| 署名したワークフロー | `tmknom/reco/.github/workflows/release.yml` |
| ソースの ref | `refs/tags/vX.Y.Z` |
| 実行基盤 | GitHub-hosted runners（`--deny-self-hosted-runners`） |

実行ファイルは次のように検証します。

```bash
gh attestation verify reco_X.Y.Z_linux_amd64 \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/vX.Y.Z \
  --deny-self-hosted-runners
```

SBOM も、ファイル名を `<実行ファイル名>.sbom.json` に変えた同じコマンドで検証します。

コンテナイメージは、`container-image.txt` に書かれた digest 付きの参照で特定してから検証します。

```bash
gh attestation verify "oci://$(cat container-image.txt)" \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/vX.Y.Z \
  --deny-self-hosted-runners
```

署名したワークフローは、`--cert-identity` ではなく `--signer-workflow` で指定します。
GitHub CLI v2.103.3 では、`--signer-workflow` は前方一致ではありませんでした。
`release.ym` のような途中までのパス、`.` を別の文字に変えたパス、`release.yml@refs` のように ref の途中までを付けた値では、検証が失敗しました。
渡したパスの直後に、証明書の SubjectAlternativeName の `@<ref>` が続く場合だけ一致すると考えられます。
`--cert-identity` はタグまで含む値を渡す必要があり、タグの照合は `--source-ref` と重なります。

Attestation の subject は Image Index の digest だけです。
Index に含まれる `linux/amd64` や `linux/arm64` のマニフェストの digest を指定すると、Attestation が見つからず検証に失敗します。

`SHA256SUMS` によるハッシュの照合も使えますが、それだけではバイナリと `SHA256SUMS` が一緒に差し替えられた場合を検出できません。
来歴の検証と併用します。

post-verify は、公開済みのアセットに対してこれと同じ引数で検証を実行します。
そのため、利用者向けの手順が実際に通ることを、リリースのたびに確かめられます。

## 外部 Action とツールの固定

- **Action**：`uses` はすべて完全なコミット SHA で固定し、行末のコメントにバージョンを書く。更新は Dependabot（`github-actions`）に任せる
- **GoReleaser と Syft**：GitHub Releases から Linux amd64 のアーカイブを取得し、ワークフローに書いた SHA-256 と照合してから使う。インストール用の Action（`goreleaser/goreleaser-action`、`anchore/sbom-action/download-syft`）は使わない。Action の SHA で固定できるのは Action のコードだけで、Action が実行時に取得するバイト列は固定できないためである。たとえば `goreleaser/goreleaser-action` v7.3.0 は、アーカイブを同じ Release の `checksums.txt` と照合するだけで、`checksums.txt` を取得できなければ警告を出して照合を飛ばし、cosign による署名の検証は runner に cosign があるときしか行わない。Dependabot では更新できないため、バージョンとハッシュは手動で更新する
- **Go**：`go.mod` の `toolchain` 行で固定する（[ビルドの設定](#ビルドの設定)）
- **ベースイメージ**：Image Index の digest で固定し、Dependabot で更新する（[コンテナイメージ](#コンテナイメージ)）

固定する値は、追加や更新のたびに公式のリリース情報で確認します。

GoReleaser と Syft は、どちらも Release の `checksums.txt` に cosign の署名（Sigstore バンドル `checksums.txt.sigstore.json`）を付けています。
ワークフローの実行時には、この署名を検証しません。
ハッシュを固定していれば取得したバイト列は一意に決まり、実行時の署名検証は完全性に何も加えないうえ、cosign 自体を固定してインストールする手間が増えるためです。
署名が役立つのは、固定するハッシュが正規の発行元のものかを確かめる場面のため、バージョンとハッシュを更新するときに、次のように手元で検証してから `checksums.txt` の値を書き写します。

```bash
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity https://github.com/goreleaser/goreleaser/.github/workflows/release.yml@refs/tags/vX.Y.Z \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify-blob syft_X.Y.Z_checksums.txt \
  --bundle syft_X.Y.Z_checksums.txt.sigstore.json \
  --certificate-identity https://github.com/anchore/syft/.github/workflows/release.yaml@refs/heads/main \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

署名者の identity は、GoReleaser v2.20.2 と Syft v1.57.1 の署名証明書に記録された値です。
Syft はタグではなく `main` ブランチのワークフローで署名しているため、identity からはバージョンを確かめられません。

## 設計上の不変条件

1. リリースタグを、公開物を残したまま別のコミットへ移動しない。
2. リリースの実行中に `main` の最新コミットを解決し直さない。
3. `CGO_ENABLED=0` でビルドする。
4. リリースの工程でソースコードや依存の定義を変更しない。
5. Go のバイナリは build ジョブの GoReleaser だけが生成し、コンテナ化で再コンパイルしない。
6. コンテナ内のバイナリは、Linux 向けの実行ファイルとバイト単位で一致する。
7. コンテナイメージは、digest で固定したベースイメージとビルドコンテキストのファイルだけで作り、ビルド中にコマンドを実行しない。
8. 来歴証明の subject は、実行ファイルと SBOM の SHA-256、Image Index の digest である。
9. OIDC の権限は attest と container だけに、公開の権限は container と release だけに与える。
10. 必須の検証に失敗した場合は、GitHub Release を公開しない。
11. 既存の GitHub Release と GHCR のバージョンタグを、自動で上書きしない。
12. 公開済みの Release のアセットを自動で変更しない。作り直す場合は Release ごと削除する。
13. 公開後に、GitHub Release と GHCR の成果物を正規の成果物と照合する。

## 残るリスク

- **build の実行環境の侵害**：attest は build の出力に署名するため、build が侵害されていればその出力にも正しい署名が付く
- **GitHub 基盤への依存**：署名、実行、公開のすべてが GitHub の基盤に依存しており、基盤全体の侵害には対抗できない
- **タグ起点のワークフロー定義**：タグの指すコミットにあるワークフロー定義そのものが実行される。悪意のある定義が実行された後では、preflight は防御にならない。`main` の変更のレビューと、タグを作成できる者の限定に依存する
- **実行中のタグの付け替え**：Tag Rulesets でタグを保護していないため、実行中にタグを付け替えられる可能性がある。release が公開の直前に、post-verify が公開後にタグの参照先を確認するが、確認の直後の付け替えは防げない
- **存在確認の誤判定**：エラーメッセージの文言で判定するため、`gh` や Docker Buildx の文言が変わると、存在しないバージョンでも preflight や container が止まる
- **公開物の書き換え**：Immutable Release を採用していないため、公開後のアセットとタグは書き換えから保護されていない
- **作り直す前のビルドの Attestation**：利用者による検証はタグの名前を固定し、コミットを固定しない。[失敗時のやり直し](#失敗時のやり直し) で同じバージョンを作り直しても、前の実行が付けた Attestation は残るため、前のビルドの実行ファイルやイメージも利用者の検証を通る
- **Linux 以外の OS での実行**：macOS と Windows のバイナリは、リリースのどの工程でも実行していない。通常 CI が OS ごとに確かめていなければ、OS 固有の不具合を公開前に検出できない
- **待機中の実行の取り消し**：`concurrency` の `queue` が `single` のため、リリースの実行中に2つ以上の実行が待機に入ると、先に待機していた実行が取り消される。取り消しは実行の一覧に残るため、気付いた時点で再実行する

## 将来の拡張

次の拡張は、必要性を判断してから着手します。

| 拡張 | 着手の条件 | 概要 |
|---|---|---|
| OS matrix の通常 CI | OS 固有の処理がコードに入ったとき | プルリクエストで、Linux / macOS / Windows の runner で `go test` を実行する |
| SBOM Attestation | `gh attestation verify` だけで、SBOM がどのバイナリを記述しているかまで確かめたいとき | `actions/attest` の `sbom-path` で、バイナリの digest を subject、SBOM を predicate とする Attestation を付ける。1回の呼び出しで渡せる SBOM は1つのため、バイナリごとに呼び出す。現在は、SBOM 自体の来歴と、SBOM に記録したバイナリの SHA-256 で対応を確かめる |
| コンテナイメージの SBOM | 提供を求められたとき | 格納するバイナリは Linux 向けの実行ファイルと同じため、その SBOM で代えられないかを先に判断する |
| `govulncheck` | 外部のモジュールに依存しているとき | 通常 CI と定期実行で既知の脆弱性を検出し、結果からリスクを評価する。リリースワークフローには加えない。脆弱性を減らすリリースまで止めないよう、検出の有無をリリースの必須条件にしない |
| 再ビルドによる再現性の確認 | Attestation と実行確認に加えて必要と判断したとき | 別の runner で再ビルドし、正規の `SHA256SUMS` のバイナリの行で照合する。SBOM は生成のたびに変わるため照合から外す |
| Immutable Release | 同じバージョンの作り直しが不要になったとき | Immutable Releases と Tag Rulesets を有効にし、post-verify と利用者向けの手順に `gh release verify` を加える |
| deb / rpm / apk | 配布を求められたとき | GoReleaser の `nfpms` を第一候補とし、build が生成したバイナリを再コンパイルせずに格納する |
| Homebrew Formula | 配布を求められたとき | 別リポジトリへの書き込み権限と認証情報の設計が必要になる |

次の項目は、拡張の予定もありません。

- **コンテナイメージの脆弱性検査**：ベースイメージは Distroless static で、格納するのは Go バイナリ1つだけである
- **公開途中の自動復旧**：手動で削除してやり直す運用で足りる
- **コンテナイメージ全体のバイト単位の再現**：内部のバイナリの一致とベースイメージの固定で足りる
