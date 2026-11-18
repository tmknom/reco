# インストール

reco は、配布アーカイブかコンテナイメージで入れます。どちらで動かす場合も、GitHub にアクセスするためのトークンを環境変数 `GH_TOKEN` で渡します。

## 配布物の種類

| 配布物 | 対象 | 置き場所 |
|---|---|---|
| 配布アーカイブ | Linux、macOS、Windows の、それぞれ amd64 と arm64 | GitHub Releases（`tmknom/reco` のリリース） |
| コンテナイメージ | `linux/amd64`、`linux/arm64` | `ghcr.io/tmknom/reco` |

## 配布アーカイブから入れる

配布アーカイブの名前は `reco_<バージョン>_<OS>_<アーキテクチャ>.tar.gz` の形です。`<OS>` は `linux`、`darwin`（macOS）、`windows` のどれかです。Windows 向けだけは拡張子が `.zip` です。`<バージョン>` には、タグの先頭の `v` を除いた値（タグ `v0.1.1` なら `0.1.1`）が入ります。

次は、Linux の amd64 向けに `0.1.1` を入れる例です。バージョン、OS、アーキテクチャは自分の環境に合わせて読み替えてください。

1. 配布アーカイブを取得する。GitHub Releases のページからダウンロードしてもよい

    ```bash
    gh release download v0.1.1 --repo tmknom/reco --pattern reco_0.1.1_linux_amd64.tar.gz
    ```

2. 展開する。アーカイブの直下には、実行ファイル `reco`（Windows では `reco.exe`）と `LICENSE` がある

    ```bash
    tar -xzf reco_0.1.1_linux_amd64.tar.gz
    ```

3. `reco` を、`PATH` の通ったディレクトリに置く
4. バージョンを表示して、入れた reco が動くことを確かめる。`0.1.1` のように、入れたバージョンが表示されればよい

    ```bash
    reco --version
    ```

トークンを渡して、リポジトリの現在の設定を表示します。`OWNER/REPO` は自分のリポジトリに読み替えてください。

```bash
export GH_TOKEN=$(gh auth token)
reco show --repo OWNER/REPO
```

## コンテナイメージで動かす

コンテナイメージは、`v0.1.1` のようなバージョンのタグで指定します。

`show` は設定ファイルを使わないため、トークンを渡すだけで動きます。

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

それぞれのオプションの役割は次のとおりです。

- **`-e GH_TOKEN`**：手元の環境変数 `GH_TOKEN` をコンテナへ渡す
- **`-v` と `-w`**：設定ファイルのあるディレクトリをコンテナの `/work` にマウントし、`/work` をカレントディレクトリにする。`-w` を省くと、reco は設定ファイル `reco.yaml` を見つけられずに止まる
- **`-it`**（`apply` だけ）：端末を割り当てる。`apply` は端末でしか承認を受け付けない

## 次にすること

トークンの持ち主に要るロールは [トークンの用意](authentication.md) に、一つのリポジトリの一つの設定を reco で変えてみる手順は [はじめての適用](getting-started.md) にあります。配布物がリリース用ワークフローで作られたものかを確かめたい場合は、[配布物の検証](verification.md) の手順を使えます。

## References

- [CLI 仕様](../spec/cli.md)：`--version` の表示（2.2節）、承認と端末（5.1節）
- [リリースワークフロー設計](../contributing/release.md)：配布物の種類（「対象と前提」）、配布アーカイブの名前と中身（「成果物とジョブ間の受け渡し」）、イメージのタグとカレントディレクトリ（「コンテナイメージ」）
- [reco のリリース](https://github.com/tmknom/reco/releases)：配布アーカイブの置き場所
