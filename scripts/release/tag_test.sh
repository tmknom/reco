#!/usr/bin/env bash
# scripts/release/tag.sh のテスト。一時ディレクトリに bare リポジトリと clone を作り、標準入力で対話の回答を渡す。
# 使い方: scripts/release/tag_test.sh
set -euo pipefail

readonly SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/tag.sh"

# 利用者の git 設定 (署名や既定ブランチ名など) に結果を左右されないようにする
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.com
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.com
# ワークフローの実行を開くための gh を呼ばない
export GH=

# 一時ディレクトリは削除せず、OS の一時領域の掃除に任せる
WORK="$(mktemp -d "${TMPDIR:-/tmp}/tag_test.XXXXXX")"
readonly WORK
failures=0

# 新しい origin (bare) と、main に 1 コミットを持つ clone を作り、clone に移動する
setup() {
  local dir="${WORK}/$1"
  git init --quiet --bare --initial-branch=main "${dir}/origin.git"
  git clone --quiet "${dir}/origin.git" "${dir}/clone" 2>/dev/null
  cd "${dir}/clone"
  git switch --quiet -c main
  git commit --quiet --allow-empty -m initial
  git push --quiet origin main
}

# origin にタグを作る (clone のローカルタグは作らない)
remote_tag() {
  git push --quiet origin "HEAD:refs/tags/$1"
}

run_tag() {
  printf '%b' "$1" | "${SCRIPT}" >"${WORK}/out" 2>&1
}

remote_tags() {
  git ls-remote --tags --refs origin | awk '{ sub(/^refs\/tags\//, "", $2); print $2 }' | sort
}

pass() { echo "ok   $1"; }
fail() {
  echo "FAIL $1: $2"
  sed 's/^/     | /' "${WORK}/out"
  failures=$((failures + 1))
}

expect_pushed() {
  local name="$1" tag="$2"
  if ! git ls-remote --exit-code --tags origin "refs/tags/${tag}" >/dev/null; then
    fail "${name}" "${tag} is not pushed"
  elif [[ "$(git cat-file -t "refs/tags/${tag}")" != "tag" ]]; then
    fail "${name}" "${tag} is not an annotated tag"
  elif [[ "$(git rev-parse "refs/tags/${tag}^{commit}")" != "$(git rev-parse origin/main)" ]]; then
    fail "${name}" "${tag} does not point to origin/main"
  else
    pass "${name}"
  fi
}

# 失敗したうえで、origin のタグが変わらず、ローカルにもタグが増えていないことを確かめる
expect_rejected() {
  local name="$1" status="$2" before_remote="$3" before_local="$4" message="$5"
  if [[ "${status}" -eq 0 ]]; then
    fail "${name}" "expected failure, but succeeded"
  elif [[ "$(remote_tags)" != "${before_remote}" ]]; then
    fail "${name}" "remote tags changed"
  elif [[ "$(git tag --list)" != "${before_local}" ]]; then
    fail "${name}" "local tags changed"
  elif ! grep -qF -- "${message}" "${WORK}/out"; then
    fail "${name}" "output does not contain: ${message}"
  else
    pass "${name}"
  fi
}

# 回答を渡してスクリプトを実行し、拒否されたことを確かめる
check_rejected() {
  local name="$1" input="$2" message="$3" before_remote before_local status=0
  before_remote="$(remote_tags)"
  before_local="$(git tag --list)"
  run_tag "${input}" || status=$?
  expect_rejected "${name}" "${status}" "${before_remote}" "${before_local}" "${message}"
}

test_patch_from_no_tags() {
  setup patch_from_no_tags
  if run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.0.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_patch() {
  setup patch
  remote_tag v0.1.0
  if run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.1.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_minor_compares_numerically() {
  setup minor_compares_numerically
  remote_tag v0.2.0
  remote_tag v0.10.0
  remote_tag v0.9.5
  if run_tag 'minor\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.11.0; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_ignores_non_release_tags() {
  setup ignores_non_release_tags
  remote_tag v0.1.0
  remote_tag v0
  remote_tag v0.2.0-rc.1
  remote_tag x9.9.9
  if run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.1.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_uses_remote_tags_not_local() {
  setup uses_remote_tags_not_local
  remote_tag v0.1.0
  git tag v0.5.0
  if run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.1.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_major_confirmed() {
  setup major_confirmed
  remote_tag v0.3.2
  if run_tag 'major\ny\ny\n'; then expect_pushed "${FUNCNAME[0]}" v1.0.0; else fail "${FUNCNAME[0]}" "script failed"; fi
}

# y を 1 回だけ渡す。major の追加の確認がなければ、その y でプッシュまで進んでしまう
test_major_needs_extra_confirmation() {
  setup major_needs_extra_confirmation
  remote_tag v0.3.2
  check_rejected "${FUNCNAME[0]}" 'major\ny\n' "Cancelled"
}

test_push_cancelled() {
  setup push_cancelled
  check_rejected "${FUNCNAME[0]}" 'patch\nn\n' "Cancelled"
}

test_no_answer() {
  setup no_answer
  check_rejected "${FUNCNAME[0]}" '' "no bump level"
}

test_invalid_level() {
  setup invalid_level
  check_rejected "${FUNCNAME[0]}" 'auto\ny\n' "invalid bump level: auto"
}

test_dirty_worktree() {
  setup dirty_worktree
  touch untracked
  check_rejected "${FUNCNAME[0]}" 'patch\ny\n' "uncommitted changes"
}

test_not_main_branch() {
  setup not_main_branch
  git switch --quiet -c topic
  check_rejected "${FUNCNAME[0]}" 'patch\ny\n' "current branch must be main"
}

test_head_ahead_of_remote() {
  setup head_ahead_of_remote
  git commit --quiet --allow-empty -m unpushed
  check_rejected "${FUNCNAME[0]}" 'patch\ny\n' "differs from origin/main"
}

test_head_behind_remote() {
  setup head_behind_remote
  git commit --quiet --allow-empty -m pushed
  git push --quiet origin main
  git reset --quiet --hard HEAD~1
  check_rejected "${FUNCNAME[0]}" 'patch\ny\n' "differs from origin/main"
}

test_local_tag_exists() {
  setup local_tag_exists
  remote_tag v0.1.0
  git tag v0.1.1
  check_rejected "${FUNCNAME[0]}" 'patch\ny\n' "already exists"
}

test_remote_unreachable() {
  setup remote_unreachable
  git remote set-url origin "${WORK}/remote_unreachable/missing.git"
  local status=0
  run_tag 'patch\ny\n' || status=$?
  if [[ "${status}" -eq 0 ]]; then
    fail "${FUNCNAME[0]}" "expected failure, but succeeded"
  elif [[ -n "$(git tag --list)" ]]; then
    fail "${FUNCNAME[0]}" "local tags changed"
  else
    pass "${FUNCNAME[0]}"
  fi
}

# fetch は通り、タグの一覧の取得だけが失敗する場合に、「タグがない」と扱って v0.0.1 をプッシュしないことを確かめる
test_ls_remote_fails() {
  setup ls_remote_fails
  remote_tag v0.1.0
  local bin="${WORK}/ls_remote_fails/bin" before_remote before_local status=0
  mkdir -p "${bin}"
  printf '#!/usr/bin/env bash\nif [[ "$1" == ls-remote ]]; then echo "simulated ls-remote failure" >&2; exit 128; fi\nexec %q "$@"\n' \
    "$(command -v git)" >"${bin}/git"
  chmod +x "${bin}/git"
  before_remote="$(remote_tags)"
  before_local="$(git tag --list)"
  PATH="${bin}:${PATH}" run_tag 'patch\ny\n' || status=$?
  expect_rejected "${FUNCNAME[0]}" "${status}" "${before_remote}" "${before_local}" "simulated ls-remote failure"
}

# タグのプッシュ後は、gh がない場合も失敗する場合も、スクリプトを失敗させない
test_gh_missing() {
  setup gh_missing
  if GH=missing-gh-command run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.0.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

test_gh_fails() {
  setup gh_fails
  if GH=false run_tag 'patch\ny\n'; then expect_pushed "${FUNCNAME[0]}" v0.0.1; else fail "${FUNCNAME[0]}" "script failed"; fi
}

for t in $(declare -F | awk '$3 ~ /^test_/ { print $3 }'); do
  "${t}"
done

if [[ "${failures}" -gt 0 ]]; then
  echo "${failures} test(s) failed" >&2
  exit 1
fi
