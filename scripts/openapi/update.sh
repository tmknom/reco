#!/usr/bin/env bash
# GitHub 公式の OpenAPI 記述 (github/rest-api-description) を固定したコミットから取得し、
# reco が利用する操作と、それらが参照する components だけを抽出して docs/research/openapi.json に保存する。
# 使い方: scripts/openapi/update.sh
set -euo pipefail

REF="8af2aca08a2386d9ce00cc8c61de3056b0453aba"
API_VERSION="2026-03-10"
SOURCE_URL="https://raw.githubusercontent.com/github/rest-api-description/${REF}/descriptions/api.github.com/api.github.com.${API_VERSION}.json"

# 抽出する操作。docs/research/ の文書が扱うエンドポイントと一致させる
OPERATIONS='{
  "/repos/{owner}/{repo}": ["get", "patch"],
  "/repos/{owner}/{repo}/actions/permissions": ["get", "put"],
  "/repos/{owner}/{repo}/actions/permissions/selected-actions": ["get", "put"],
  "/repos/{owner}/{repo}/actions/permissions/workflow": ["get", "put"]
}'

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
output="${repo_root}/docs/research/openapi.json"

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/reco-openapi.XXXXXX")"
trap 'rm -rf "${work_dir}"' EXIT

curl -sSfL -o "${work_dir}/full.json" "${SOURCE_URL}"
jq --argjson operations "${OPERATIONS}" -f "${script_dir}/extract.jq" "${work_dir}/full.json" >"${output}"
echo "wrote ${output}"
