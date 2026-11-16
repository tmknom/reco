# インストールと認証の準備

reco を手元で動かすには、reco の実行ファイルかコンテナイメージと、GitHub にアクセスするためのトークンが要ります。用語の意味は [用語集](glossary.md) にあります。

## 配布物の種類

reco は、GitHub Releases と GitHub Container Registry（GHCR）で配布しています。

| 配布物 | 対象 | 置き場所 |
|---|---|---|
| 配布アーカイブ | Linux、macOS、Windows の、それぞれ amd64 と arm64 | GitHub Releases（`tmknom/reco` のリリース） |
| コンテナイメージ | `linux/amd64`、`linux/arm64` | `ghcr.io/tmknom/reco` |

コンテナイメージには `latest` タグがありません。`v0.1.1` のようなバージョンのタグか、digest（`ghcr.io/tmknom/reco@sha256:...` の形）で指定します。

Homebrew と OS のパッケージ（deb、rpm、apk）は配布していません。`go install` で入れた reco はバージョンを持たず（`reco --version` が `dev` を表示する）、[配布物を検証する](#配布物を検証する) の手順も使えないため、このガイドでは案内しません。

## 配布アーカイブから入れる

配布アーカイブの名前は `reco_<バージョン>_<OS>_<アーキテクチャ>.tar.gz` の形です。`<OS>` は `linux`、`darwin`（macOS）、`windows` のどれかです。Windows 向けだけは拡張子が `.zip` です。`<バージョン>` には、タグの先頭の `v` を除いた値（タグ `v0.1.1` なら `0.1.1`）が入ります。

次は、Linux の amd64 向けに `0.1.1` を入れる例です。バージョン、OS、アーキテクチャは自分の環境に合わせて読み替えてください。

1. 配布アーカイブを取得する。GitHub Releases のページからダウンロードしてもよい

    ```bash
    gh release download v0.1.1 --repo tmknom/reco --pattern reco_0.1.1_linux_amd64.tar.gz
    ```

2. 取得した配布アーカイブを検証する（[配布物を検証する](#配布物を検証する)）
3. 展開する。アーカイブの直下には、実行ファイル `reco`（Windows では `reco.exe`）と `LICENSE` がある

    ```bash
    tar -xzf reco_0.1.1_linux_amd64.tar.gz
    ```

4. `reco` を、`PATH` の通ったディレクトリに置く
5. バージョンを表示して、入れた reco が動くことを確かめる。`0.1.1` のように、入れたバージョンが表示されればよい

    ```bash
    reco --version
    ```

## 配布物を検証する

GitHub CLI（`gh`）の `gh attestation verify` で、配布物が reco のリポジトリのリリース用ワークフローによって、指定したタグから作られたものかを検証できます。

配布アーカイブは次のように検証します。

```bash
gh attestation verify reco_0.1.1_linux_amd64.tar.gz \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/v0.1.1 \
  --deny-self-hosted-runners
```

コンテナイメージは、同じリリースの `container-image.txt` に書かれた、digest 付きの参照で検証します。

```bash
gh release download v0.1.1 --repo tmknom/reco --pattern container-image.txt
gh attestation verify "oci://$(cat container-image.txt)" \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/v0.1.1 \
  --deny-self-hosted-runners
```

どちらも、終了コード `0` で終われば検証に成功しています。`--source-ref` のタグは、取得した配布物のバージョンに合わせます。

リリースには `SHA256SUMS` もありますが、これとの照合だけでは、配布アーカイブと `SHA256SUMS` が一緒に差し替えられた場合を見分けられないため、`gh attestation verify` で検証します。

## コンテナイメージで動かす

コンテナイメージで reco を動かすときは、次のものを `docker run` に渡します。

- **トークン**：`-e GH_TOKEN` で、手元の環境変数 `GH_TOKEN` をコンテナへ渡す（[トークンを用意する](#トークンを用意する)）
- **設定ファイルのあるディレクトリ**（`plan` と `apply`）：`-v` でコンテナにマウントし、`-w` で同じパスを作業ディレクトリにする。`-w` を省くと、reco は設定ファイル `reco.yaml` を見つけられずに止まる
- **端末**（`apply` だけ）：`-it` で端末を割り当てる。`apply` は端末でしか承認を受け付けない（[承認](commands.md#承認)）

`show` は設定ファイルを使わないため、マウントは要りません。

```bash
GH_TOKEN=$(gh auth token) docker run --rm -e GH_TOKEN \
  ghcr.io/tmknom/reco:v0.1.1 show --repo OWNER/REPO
```

`plan` と `apply` は、設定ファイルのあるディレクトリで次のように実行します。

```bash
GH_TOKEN=$(gh auth token) docker run --rm -e GH_TOKEN \
  -v "$PWD":/work -w /work \
  ghcr.io/tmknom/reco:v0.1.1 plan

GH_TOKEN=$(gh auth token) docker run --rm -it -e GH_TOKEN \
  -v "$PWD":/work -w /work \
  ghcr.io/tmknom/reco:v0.1.1 apply
```

検証したイメージそのものを使うには、`ghcr.io/tmknom/reco:v0.1.1` の代わりに `container-image.txt` の参照（`ghcr.io/tmknom/reco@sha256:...`）を指定します。

## トークンを用意する

reco は、環境変数 `GH_TOKEN` か `GITHUB_TOKEN` で渡されたトークンで GitHub にアクセスします。どちらを使うかの規則は [コマンドリファレンス](commands.md#認証情報の環境変数) にあります。

主な使い方は、GitHub CLI（`gh`）にログインしておき、`gh auth token` が出力するトークンを `GH_TOKEN` に渡すことです。reco 自身は `gh` からトークンを取り出さないため、環境変数で渡す必要があります。

```bash
export GH_TOKEN=$(gh auth token)
reco show --repo OWNER/REPO
```

`show` が対象のリポジトリの設定を表示すれば、トークンを用意できています。

トークンの持ち主（`gh` にログインしたアカウント）が対象のリポジトリで持つロールと reco の動作の関係は次のとおりです。

| トークンの持ち主のロール | reco の動作 |
|---|---|
| Admin | 動作を確かめている |
| Write、Maintain | 動作を確かめていない |
| Write より下（Read など） | GitHub が一般設定（`general`）の値を応答に含めないため、`show`、`plan`、`apply` のどれもエラーで止まる |

fine-grained personal access token など、`gh auth token` 以外のトークンも同じ環境変数で渡せますが、reco で動作を確かめていません。

reco が渡されたトークンをどう扱うかは、[認証情報を表に出さない](safety.md#認証情報を表に出さない) にあります。

準備ができたら、[はじめての適用](getting-started.md) で、一つのリポジトリの一つの設定を reco で変えてみます。

## References

- [CLI 仕様](../spec/cli.md)：`--version` の表示（2.2節）、認証情報の選択、`gh` から取得しないこと、動作を確かめたロール（3章）、承認と端末（5.1節）
- [リリースワークフロー設計](../contributing/release.md)：配布物の種類（「対象と前提」）、配布アーカイブの名前と中身（「成果物とジョブ間の受け渡し」）、イメージのタグと作業ディレクトリ（「コンテナイメージ」）、`gh attestation verify` の期待値と、`SHA256SUMS` だけでは足りない理由（「利用者による検証」）
- [reco のリリース](https://github.com/tmknom/reco/releases)：配布アーカイブ、`SHA256SUMS`、`container-image.txt` の置き場所
