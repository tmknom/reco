#!/usr/bin/env bash
# go test -coverprofile の出力から main.go を除いた集計を表示し、internal/ 配下に未実行のブロックがあれば失敗する。
# 使い方: scripts/coverage/check.sh <coverage profile>
set -euo pipefail

profile="${1:?usage: $0 <coverage profile>}"
filtered="${profile%.out}.filtered.out"

# main.go は起動だけのため、カバレッジの計測から外す
grep -Ev '/main\.go:' "${profile}" >"${filtered}"
go tool cover -func="${filtered}"

# go tool cover -func は小数第 1 位に丸めて 100% 未満でも 100.0% と表示し得るため、ブロックの実行回数で判定する。
# 行末の 2 列はステートメント数と実行回数で、ステートメントを含まないブロック (空の case 等) は実行されなくても未到達に数えない
if grep '/internal/' "${filtered}" | grep -E ' [1-9][0-9]* 0$'; then
  echo "coverage under internal/ must be 100%: the blocks above are not executed" >&2
  exit 1
fi
