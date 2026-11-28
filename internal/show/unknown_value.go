package show

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
)

// unknownValueText は、GitHub が reco の知らない値を返した設定の、値の位置に示す文言。
// 知らない値そのものは、reco の検証していない文字列のため示さない。
const unknownValueText = "unavailable (GitHub returned a value that reco does not know)"

// requireUnknownValue は、観測不能の理由が知らない値であることを確かめる。
// 観測は知らない値だけをその設定の観測不能にするため、ほかの理由を知らない値の文言で黙って示さないよう、不変条件の破れとして panic する。
func requireUnknownValue(key string, reason github.UnavailableReason) {
	if reason != github.UnavailableUnknownValue {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for %s", int(reason), key))
	}
}
