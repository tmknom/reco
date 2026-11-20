# Installation

reco は、配布バイナリかコンテナイメージでインストールします。どちらで動かす場合も、GitHub にアクセスするためのトークンを環境変数 `GH_TOKEN` で渡します。本ドキュメントの例では、GitHub CLI（`gh`）の `gh auth token` が出力するトークンを使います。

## 配布物の種類

| 配布物 | 対象 | 置き場所 |
|---|---|---|
| 配布バイナリ | Linux、macOS、Windows の、それぞれ amd64 と arm64 | [GitHub Releases](https://github.com/tmknom/reco/releases) |
| コンテナイメージ | `linux/amd64`、`linux/arm64` | `ghcr.io/tmknom/reco` |

## 配布バイナリからインストールする

自分の OS とアーキテクチャに合うコードブロックを、そのまま端末に貼り付けて実行します。どのコードブロックも、配布バイナリをカレントディレクトリにダウンロードし、`reco`（Windows では `reco.exe`）という名前で `PATH` の通ったディレクトリへコピーします。ダウンロードしたファイルには実行権限が付かないため、Linux と macOS では `install -m 0755` で実行権限を付けてコピーします。

コードブロックは `v0.1.1` をインストールします。別のバージョンを入れるときは、URL とファイル名に含まれる `v0.1.1` と `0.1.1` を、入れたいバージョンに置き換えてください。配布バイナリの名前は `reco_<バージョン>_<OS>_<アーキテクチャ>`（Windows 向けだけ末尾に `.exe`）の形で、`<バージョン>` にはタグの先頭の `v` を除いた値が入ります。

### macOS

arm64（Apple シリコン）の場合

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_darwin_arm64
sudo install -m 0755 reco_0.1.1_darwin_arm64 /usr/local/bin/reco
```

amd64（Intel）の場合

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_darwin_amd64
sudo install -m 0755 reco_0.1.1_darwin_amd64 /usr/local/bin/reco
```

### Windows

PowerShell で実行します。PowerShell のバージョンによっては `curl` が `Invoke-WebRequest` の別名になっているため、`curl.exe` と書きます。

amd64 の場合

```powershell
curl.exe -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_windows_amd64.exe
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\reco" | Out-Null
Copy-Item -Force reco_0.1.1_windows_amd64.exe "$env:LOCALAPPDATA\Programs\reco\reco.exe"
```

arm64 の場合

```powershell
curl.exe -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_windows_arm64.exe
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\reco" | Out-Null
Copy-Item -Force reco_0.1.1_windows_arm64.exe "$env:LOCALAPPDATA\Programs\reco\reco.exe"
```

初めてインストールするときだけ、置いたディレクトリを利用者の `PATH` に加えます。変更した `PATH` は、新しく開いた PowerShell から有効になります。

```powershell
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$env:LOCALAPPDATA\Programs\reco", "User")
```

### Linux

amd64 の場合

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_linux_amd64
sudo install -m 0755 reco_0.1.1_linux_amd64 /usr/local/bin/reco
```

arm64 の場合

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_linux_arm64
sudo install -m 0755 reco_0.1.1_linux_arm64 /usr/local/bin/reco
```

### インストールした reco が動くことの確認

バージョンを表示します。`0.1.1` のように、インストールしたバージョンが表示されれば、インストールは完了です。

```bash
reco --version
```

続けて、トークンを渡して、リポジトリの現在の設定を表示します。`OWNER/REPO` は自分のリポジトリに読み替えてください。

```bash
export GH_TOKEN=$(gh auth token)
reco show --repo OWNER/REPO
```

Windows の PowerShell では、1行目を `$env:GH_TOKEN = gh auth token` に置き換えます。

## コンテナイメージで動かす

コンテナイメージは、`v0.1.1` のようなバージョンのタグで指定して取得します。

```bash
docker pull ghcr.io/tmknom/reco:v0.1.1
```

設定ファイルのあるディレクトリで、次のように実行します。

```bash
GH_TOKEN=$(gh auth token) docker run --rm -it -e GH_TOKEN \
  -v "$PWD":/work -w /work \
  ghcr.io/tmknom/reco:v0.1.1 plan
```

## References

- [Verification](verification.md)：配布バイナリとコンテナイメージが reco のリリース用ワークフローで作られたものかを、GitHub Attestations で検証する手順
- [Authentication](authentication.md)：`gh auth token` のトークンを渡す理由と、トークンの持ち主に要るロール
