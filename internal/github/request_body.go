package github

import (
	"encoding/json"
	"fmt"
)

// mustMarshalJSON は v を JSON にする。
// 本文の型は bool、string、[]string と、string を値に持つ map だけを持ち、json.Marshal がどの値でも失敗しないため、失敗を不変条件の破れとして panic する。
// encoding/json に Must の関数がないため、この包みを置く。
func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("github: marshal request body: %v", err))
	}
	return data
}
