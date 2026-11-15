#!/usr/bin/env bash
# origin の最新のリリースタグから、対話で選んだ bump level でバージョンを上げ、main の先頭に注釈付きタグを作って origin へプッシュする。
# リリースワークフロー (.github/workflows/release.yml) はタグの push で起動する。設計は docs/contributing/release.md を参照。
# 使い方: scripts/release/tag.sh
# プッシュ後、GH のコマンド (既定は gh) があれば、起動したワークフローの実行をブラウザで開く。GH が空なら開かない。
set -euo pipefail

readonly REMOTE=origin
readonly WORKFLOW=release.yml
GH="${GH-gh}"

fail() {
  echo "Error: $*" >&2
  exit 1
}

confirm() {
  local answer=""
  read -r -p "$1 (y/N): " answer || true
  [[ "${answer}" == [yY]* ]]
}

# タグはプッシュした時点でリリースが始まるため、タグを付けるコミットが origin の main の先頭であることを確かめる
[[ -z "$(git status --porcelain)" ]] || fail "working tree has uncommitted changes"
[[ "$(git branch --show-current)" == main ]] || fail "current branch must be main"
git fetch --quiet "${REMOTE}" main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse FETCH_HEAD)" ]] || fail "HEAD differs from ${REMOTE}/main"

# preflight が見るのは origin のタグのため、ローカルのタグは使わない。
# 取得の失敗を「タグがない」と扱わないよう、取得とフィルタを分けて取得の失敗で止める
tags="$(git ls-remote --tags --refs --sort=-version:refname "${REMOTE}" 'v*')"
current="$(sed -n -E 's#^.*refs/tags/(v[0-9]+\.[0-9]+\.[0-9]+)$#\1#p' <<<"${tags}")"
current="${current%%$'\n'*}"
current="${current:-v0.0.0}"
echo "Current version: ${current}"

read -r -p "Bump up to (patch / minor / major): " level || fail "no bump level given"
IFS=. read -r major minor patch <<<"${current#v}"
case "${level}" in
  patch) patch=$((patch + 1)) ;;
  minor) minor=$((minor + 1)) patch=0 ;;
  major)
    confirm "Confirm major version upgrade?" || fail "Cancelled"
    major=$((major + 1)) minor=0 patch=0
    ;;
  *) fail "invalid bump level: ${level}" ;;
esac
next="v${major}.${minor}.${patch}"

confirm "Push tag ${next} to ${REMOTE} and start the release?" || fail "Cancelled"
git tag -a "${next}" -m "${next}"
git push "${REMOTE}" "refs/tags/${next}"

# リリースはプッシュで始まっているため、実行を開けなくてもスクリプトを失敗させない
if [[ -n "${GH}" ]] && command -v "${GH}" >/dev/null; then
  sleep 5
  id="$("${GH}" run list --workflow "${WORKFLOW}" --branch "${next}" --limit 1 --json databaseId --jq '.[0].databaseId // empty' || true)"
  if [[ -z "${id}" ]] || ! "${GH}" run view --web "${id}"; then
    echo "Warning: could not open the release run; check 'gh run list --workflow ${WORKFLOW}'" >&2
  fi
fi
