//go:build !js

package stdlib

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/loreste/weft/internal/accelerator"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/tensor"
)

var nativeAccelerators = struct {
	sync.RWMutex
	next  uint64
	items map[string]*accelerator.Plugin
}{items: make(map[string]*accelerator.Plugin)}

// packageAccelerator exposes the versioned native ABI. It intentionally does
// not auto-discover libraries: loading native code is an explicit capability
// decision by the application.
func packageAccelerator() runtime.Value {
	p := pkg()

	set(p, "supported", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(accelerator.Supported()), nil
	}, 0)

	set(p, "load", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindStr {
			return errRes("accelerator.load(path)", "accelerator"), nil
		}
		plugin, err := accelerator.Load(args[0].String())
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		id := fmt.Sprintf("accel-%d", atomic.AddUint64(&nativeAccelerators.next, 1))
		nativeAccelerators.Lock()
		nativeAccelerators.items[id] = plugin
		nativeAccelerators.Unlock()
		return runtime.Ok(goToValue(map[string]any{
			"id":       id,
			"path":     plugin.Path(),
			"manifest": plugin.Manifest(),
		})), nil
	}, 1)

	set(p, "run", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[1].Kind != runtime.KindStr {
			return errRes("accelerator.run(plugin, operation, input)", "accelerator"), nil
		}
		id, err := acceleratorPluginID(args[0])
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		input, err := json.Marshal(valueToGo(args[2]))
		if err != nil {
			return errRes("accelerator input: "+err.Error(), "accelerator"), nil
		}
		nativeAccelerators.RLock()
		plugin := nativeAccelerators.items[id]
		if plugin == nil {
			nativeAccelerators.RUnlock()
			return errRes("accelerator plugin is not loaded", "accelerator"), nil
		}
		output, err := plugin.RunJSON(args[1].String(), input)
		nativeAccelerators.RUnlock()
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		var value any
		if err := json.Unmarshal(output, &value); err != nil {
			return errRes("accelerator output: "+err.Error(), "accelerator"), nil
		}
		return runtime.Ok(goToValue(value)), nil
	}, 3)

	set(p, "close", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("accelerator.close(plugin)", "accelerator"), nil
		}
		id, err := acceleratorPluginID(args[0])
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		nativeAccelerators.Lock()
		plugin := nativeAccelerators.items[id]
		delete(nativeAccelerators.items, id)
		nativeAccelerators.Unlock()
		if plugin != nil {
			if err := plugin.Close(); err != nil {
				return errRes(err.Error(), "accelerator"), nil
			}
		}
		return runtime.Ok(runtime.Bool(true)), nil
	}, 1)

	set(p, "run_tensor", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[1].Kind != runtime.KindStr || args[2].Kind != runtime.KindList {
			return errRes("accelerator.run_tensor(plugin, operation, inputs)", "accelerator"), nil
		}
		id, err := acceleratorPluginID(args[0])
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		inputValues := args[2].Obj.(*runtime.ListObj).Items
		inputs := make([]*tensor.Tensor, len(inputValues))
		for i, value := range inputValues {
			inputs[i], err = tensorFromRuntime(value)
			if err != nil {
				return errRes(fmt.Sprintf("accelerator tensor input %d: %v", i, err), "accelerator"), nil
			}
		}
		nativeAccelerators.RLock()
		plugin := nativeAccelerators.items[id]
		if plugin == nil {
			nativeAccelerators.RUnlock()
			return errRes("accelerator plugin is not loaded", "accelerator"), nil
		}
		output, err := plugin.RunTensor(args[1].String(), inputs...)
		nativeAccelerators.RUnlock()
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		result, err := tensorToRuntime(output)
		if err != nil {
			return errRes(err.Error(), "accelerator"), nil
		}
		return runtime.Ok(result), nil
	}, 3)

	set(p, "backends", func(args []runtime.Value) (runtime.Value, error) {
		nativeAccelerators.RLock()
		defer nativeAccelerators.RUnlock()
		result := make([]any, 0, len(nativeAccelerators.items))
		for id, plugin := range nativeAccelerators.items {
			result = append(result, map[string]any{
				"id":       id,
				"path":     plugin.Path(),
				"manifest": plugin.Manifest(),
			})
		}
		return goToValue(result), nil
	}, 0)

	return p
}

func tensorFromRuntime(value runtime.Value) (*tensor.Tensor, error) {
	if value.Kind != runtime.KindMap {
		return nil, fmt.Errorf("tensor descriptor must be a map")
	}
	mo := value.Obj.(*runtime.MapObj)
	dtypeValue, ok := mo.Vals["dtype"]
	if !ok || dtypeValue.Kind != runtime.KindStr {
		return nil, fmt.Errorf("tensor descriptor dtype must be a string")
	}
	dtype, err := tensor.ParseDType(dtypeValue.String())
	if err != nil {
		return nil, err
	}
	shapeValue, ok := mo.Vals["shape"]
	if !ok || shapeValue.Kind != runtime.KindList {
		return nil, fmt.Errorf("tensor descriptor shape must be an integer list")
	}
	shapeItems := shapeValue.Obj.(*runtime.ListObj).Items
	shape := make([]int, len(shapeItems))
	for i, item := range shapeItems {
		if item.Kind != runtime.KindInt || item.I < 0 || item.I > int64(^uint(0)>>1) {
			return nil, fmt.Errorf("tensor descriptor shape[%d] must be non-negative", i)
		}
		shape[i] = int(item.I)
	}
	maxElements := int64(accelerator.MaxResultBytes / dtype.ItemSize())
	count := int64(1)
	for _, dimension := range shape {
		if dimension == 0 {
			count = 0
			break
		}
		if count > maxElements/int64(dimension) {
			return nil, fmt.Errorf("tensor descriptor exceeds %d bytes", accelerator.MaxResultBytes)
		}
		count *= int64(dimension)
	}
	dataValue, ok := mo.Vals["data"]
	if !ok || dataValue.Kind != runtime.KindList {
		return nil, fmt.Errorf("tensor descriptor data must be a flat list")
	}
	dataItems := dataValue.Obj.(*runtime.ListObj).Items
	result, err := tensor.New(dtype, shape)
	if err != nil {
		return nil, err
	}
	if int64(len(dataItems)) != result.Numel() {
		return nil, fmt.Errorf("tensor descriptor has %d values, want %d", len(dataItems), result.Numel())
	}
	bytes := make([]byte, len(result.Bytes()))
	for i, item := range dataItems {
		offset := i * dtype.ItemSize()
		switch dtype {
		case tensor.Bool:
			if item.Kind != runtime.KindBool {
				return nil, fmt.Errorf("tensor bool data[%d] must be boolean", i)
			}
			if item.B {
				bytes[offset] = 1
			}
		case tensor.Int64:
			if item.Kind != runtime.KindInt {
				return nil, fmt.Errorf("tensor int64 data[%d] must be integer", i)
			}
			binary.LittleEndian.PutUint64(bytes[offset:offset+8], uint64(item.I))
		case tensor.Float32:
			value, ok := runtimeNumber(item)
			if !ok {
				return nil, fmt.Errorf("tensor float32 data[%d] must be numeric", i)
			}
			binary.LittleEndian.PutUint32(bytes[offset:offset+4], math.Float32bits(float32(value)))
		case tensor.Float64:
			value, ok := runtimeNumber(item)
			if !ok {
				return nil, fmt.Errorf("tensor float64 data[%d] must be numeric", i)
			}
			binary.LittleEndian.PutUint64(bytes[offset:offset+8], math.Float64bits(value))
		}
	}
	return tensor.FromBytes(dtype, shape, bytes)
}

func runtimeNumber(value runtime.Value) (float64, bool) {
	switch value.Kind {
	case runtime.KindInt:
		return float64(value.I), true
	case runtime.KindFloat:
		return value.F, true
	default:
		return 0, false
	}
}

func tensorToRuntime(value *tensor.Tensor) (runtime.Value, error) {
	result := runtime.NewMap()
	mo := result.Obj.(*runtime.MapObj)
	mo.Keys = append(mo.Keys, "dtype", "shape", "data")
	mo.Vals["dtype"] = runtime.Str(string(value.DType()))
	shape := value.Shape()
	shapeValues := make([]runtime.Value, len(shape))
	for i, dimension := range shape {
		shapeValues[i] = runtime.Int(int64(dimension))
	}
	mo.Vals["shape"] = runtime.List(shapeValues...)
	data := make([]runtime.Value, 0, int(value.Numel()))
	for flat := int64(0); flat < value.Numel(); flat++ {
		indices := tensorUnravel(flat, shape)
		item, err := value.Value(indices...)
		if err != nil {
			return runtime.Null(), err
		}
		switch number := item.(type) {
		case bool:
			data = append(data, runtime.Bool(number))
		case int64:
			data = append(data, runtime.Int(number))
		case float32:
			data = append(data, runtime.Float(float64(number)))
		case float64:
			data = append(data, runtime.Float(number))
		}
	}
	mo.Vals["data"] = runtime.List(data...)
	return result, nil
}

func tensorUnravel(flat int64, shape []int) []int {
	indices := make([]int, len(shape))
	remaining := flat
	for i := range shape {
		stride := int64(1)
		for j := i + 1; j < len(shape); j++ {
			stride *= int64(shape[j])
		}
		if stride > 0 {
			indices[i] = int(remaining / stride)
			remaining %= stride
		}
	}
	return indices
}

func acceleratorPluginID(value runtime.Value) (string, error) {
	if value.Kind != runtime.KindMap {
		return "", fmt.Errorf("accelerator plugin must be the result of accelerator.load")
	}
	m := value.Obj.(*runtime.MapObj)
	id, ok := m.Vals["id"]
	if !ok || id.Kind != runtime.KindStr || id.String() == "" {
		return "", fmt.Errorf("accelerator plugin handle is invalid")
	}
	return id.String(), nil
}
