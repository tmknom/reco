# Installation

reco は、実行ファイルかコンテナイメージでインストールします。

## アーティファクトの種類

| アーティファクト | 対象 | 置き場所 |
|---|---|---|
| 実行ファイル | Linux、macOS、Windows の、それぞれ amd64 と arm64 | [GitHub Releases](https://github.com/tmknom/reco/releases) |
| コンテナイメージ | `linux/amd64`、`linux/arm64` | `ghcr.io/tmknom/reco` |

## 実行ファイルからインストールする

自分の OS と CPU アーキテクチャに合致するコマンドを実行します。本ドキュメントでは `v0.1.1` をインストールする場合の手順を掲載します。

### macOS

#### arm64（Apple シリコン）

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_darwin_arm64
sudo install -m 0755 reco_0.1.1_darwin_arm64 /usr/local/bin/reco
```

#### amd64（Intel）

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_darwin_amd64
sudo install -m 0755 reco_0.1.1_darwin_amd64 /usr/local/bin/reco
```

### Linux

#### amd64

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_linux_amd64
sudo install -m 0755 reco_0.1.1_linux_amd64 /usr/local/bin/reco
```

#### arm64

```bash
curl -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_linux_arm64
sudo install -m 0755 reco_0.1.1_linux_arm64 /usr/local/bin/reco
```

### Windows

PowerShell で実行します。PowerShell のバージョンによっては `curl` が `Invoke-WebRequest` の別名になっているため、`curl.exe` と書きます。

#### amd64

```powershell
curl.exe -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_windows_amd64.exe
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\reco" | Out-Null
Copy-Item -Force reco_0.1.1_windows_amd64.exe "$env:LOCALAPPDATA\Programs\reco\reco.exe"
```

#### arm64

```powershell
curl.exe -fsSLO https://github.com/tmknom/reco/releases/download/v0.1.1/reco_0.1.1_windows_arm64.exe
New-Item -ItemType Directory -Force "$env:LOCALAPPDATA\Programs\reco" | Out-Null
Copy-Item -Force reco_0.1.1_windows_arm64.exe "$env:LOCALAPPDATA\Programs\reco\reco.exe"
```

初めてインストールするときだけ、置いたディレクトリを利用者の `PATH` に加えます。変更した `PATH` は、新しく開いた PowerShell から有効になります。

```powershell
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$env:LOCALAPPDATA\Programs\reco", "User")
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

- [Verification](verification.md)：実行ファイルとコンテナイメージが reco のリリース用ワークフローで作られたものかを、GitHub Attestations で検証する手順
- [Authentication](authentication.md)：`gh auth token` のトークンを渡す理由と、トークンの持ち主に要るロール
