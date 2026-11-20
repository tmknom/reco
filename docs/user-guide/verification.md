# Verification

GitHub CLI（`gh`）の `gh attestation verify` で、アーティファクトが reco のリポジトリのリリース用ワークフローによって、指定したタグから作られたものかを検証できます。

## 実行ファイルを検証する

取得した実行ファイルを、実行する前に次のように検証します。

```bash
gh attestation verify reco_0.1.1_linux_amd64 \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/v0.1.1 \
  --deny-self-hosted-runners
```

終了コード `0` で終われば検証に成功しています。`--source-ref` のタグは、取得したアーティファクトのバージョンに合わせます。

## コンテナイメージを検証する

コンテナイメージは、同じリリースの `container-image.txt` に書かれた、digest 付きの参照で検証します。

```bash
gh release download v0.1.1 --repo tmknom/reco --pattern container-image.txt
gh attestation verify "oci://$(cat container-image.txt)" \
  --repo tmknom/reco \
  --signer-workflow tmknom/reco/.github/workflows/release.yml \
  --source-ref refs/tags/v0.1.1 \
  --deny-self-hosted-runners
```

終了コード `0` で終われば検証に成功しています。

検証したイメージそのものを使うには、`docker run` で `ghcr.io/tmknom/reco:v0.1.1` の代わりに `container-image.txt` の参照（`ghcr.io/tmknom/reco@sha256:...` の形）を指定します。

## References

- [リリースワークフロー設計](../contributing/release.md)：`gh attestation verify` の期待値（「利用者による検証」）
- [reco のリリース](https://github.com/tmknom/reco/releases)：`container-image.txt` の置き場所
