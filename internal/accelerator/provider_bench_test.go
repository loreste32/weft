package accelerator

import (
	"os"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func benchmarkTensor(b *testing.B, dtype tensor.DType, shape []int, value float64) *tensor.Tensor {
	b.Helper()
	count := 1
	for _, dimension := range shape {
		count *= dimension
	}
	values := make([]any, count)
	for i := range values {
		if dtype == tensor.Float32 {
			values[i] = float32(value)
		} else {
			values[i] = value
		}
	}
	result, err := tensor.FromList(dtype, shape, values)
	if err != nil {
		b.Fatal(err)
	}
	return result
}

func loadBenchmarkProvider(b *testing.B) *Plugin {
	b.Helper()
	path := os.Getenv("WEFT_ACCELERATOR_PLUGIN")
	if path == "" {
		b.Skip("WEFT_ACCELERATOR_PLUGIN is not configured")
	}
	plugin, err := Load(path)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = plugin.Close() })
	return plugin
}

func providerBenchmarkDType(plugin *Plugin) tensor.DType {
	if plugin.Manifest().Metadata["dtype"] == "float64" {
		return tensor.Float64
	}
	return tensor.Float32
}

func providerDeclares(plugin *Plugin, operation string) bool {
	for _, declared := range plugin.Manifest().Operations {
		if declared == operation {
			return true
		}
	}
	return false
}

func BenchmarkExternalProviderTensorMatmul(b *testing.B) {
	plugin := loadBenchmarkProvider(b)
	dtype := providerBenchmarkDType(plugin)
	left := benchmarkTensor(b, dtype, []int{32, 32}, 1.25)
	right := benchmarkTensor(b, dtype, []int{32, 32}, 0.75)
	b.Cleanup(func() {
		tensor.Release(left)
		tensor.Release(right)
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, info, err := plugin.RunTensor("tensor_matmul", left, right)
		if err != nil {
			b.Fatal(err)
		}
		if !info.Reported {
			b.Fatalf("tensor_matmul returned unreported execution info: %+v", info)
		}
		tensor.Release(result)
	}
}

func BenchmarkExternalProviderTensorSum(b *testing.B) {
	plugin := loadBenchmarkProvider(b)
	if !providerDeclares(plugin, "tensor_sum") {
		b.Skip("provider does not declare tensor_sum")
	}
	dtype := providerBenchmarkDType(plugin)
	input := benchmarkTensor(b, dtype, []int{4096}, 1.25)
	b.Cleanup(func() { tensor.Release(input) })
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, info, err := plugin.RunTensor("tensor_sum", input)
		if err != nil {
			b.Fatal(err)
		}
		if !info.Reported {
			b.Fatalf("tensor_sum returned unreported execution info: %+v", info)
		}
		tensor.Release(result)
	}
}
