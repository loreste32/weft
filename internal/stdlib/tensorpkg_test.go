//go:build !js

package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestTensorToRuntimeRejectsLossyUInt64(t *testing.T) {
	value, err := tensor.FromList(tensor.UInt64, []int{1}, []any{uint64(^uint64(0))})
	if err != nil {
		t.Fatal(err)
	}
	tensorValue, err := tensorToRuntime(value)
	if err == nil {
		t.Fatalf("tensorToRuntime returned %v for an unrepresentable uint64", tensorValue)
	}
	tensor.Release(value)
}
