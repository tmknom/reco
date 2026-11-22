package github

import (
	"testing"
)

func TestMustMarshalJSONPanicsOnUnsupportedType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustMarshalJSON() did not panic for a channel")
		}
	}()
	_ = mustMarshalJSON(make(chan int))
}
