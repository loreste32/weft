//go:build !js

package stdlib

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/tensor"
)

// Host-side tensor registry. Warp uses these handles as primary packed storage
// for supported numeric dtypes; values remain inspectable via to_list.

var hostTensors = struct {
	sync.RWMutex
	next  uint64
	items map[string]*tensor.Tensor
}{items: make(map[string]*tensor.Tensor)}

func packageTensor() runtime.Value {
	p := pkg()

	set(p, "supported", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Bool(true), nil
	}, 0)

	set(p, "from_list", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 || args[0].Kind != runtime.KindStr || args[1].Kind != runtime.KindList || args[2].Kind != runtime.KindList {
			return errRes("tensor.from_list(dtype, shape, data)", "tensor"), nil
		}
		dtype, err := tensor.ParseDType(args[0].String())
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		shape, err := intList(args[1], "shape")
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		values, err := anyList(args[2])
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		ten, err := tensor.FromList(dtype, shape, values)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(tensorHandle(ten)), nil
	}, 3)

	set(p, "to_list", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		values, err := tensor.ToList(ten)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(goToValue(values)), nil
	}, 1)

	set(p, "shape", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(goToValue(ten.Shape())), nil
	}, 1)

	set(p, "dtype", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(runtime.Str(string(ten.DType()))), nil
	}, 1)

	set(p, "numel", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(runtime.Int(ten.Numel())), nil
	}, 1)

	set(p, "add", func(args []runtime.Value) (runtime.Value, error) {
		return binaryTensor(args, tensor.Add)
	}, 2)
	set(p, "sub", func(args []runtime.Value) (runtime.Value, error) {
		return binaryTensor(args, tensor.Sub)
	}, 2)
	set(p, "mul", func(args []runtime.Value) (runtime.Value, error) {
		return binaryTensor(args, tensor.Mul)
	}, 2)
	set(p, "div", func(args []runtime.Value) (runtime.Value, error) {
		return binaryTensor(args, tensor.Div)
	}, 2)
	set(p, "matmul", func(args []runtime.Value) (runtime.Value, error) {
		return binaryTensor(args, tensor.MatMul)
	}, 2)
	set(p, "sum", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		out, err := tensor.Sum(ten)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		return runtime.Ok(tensorHandle(out)), nil
	}, 1)

	set(p, "contiguous", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		out, err := ten.Contiguous()
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		// Contiguous() returns the original tensor when no copy is needed.
		// Returning a second owning handle would let freeing either alias clear
		// storage still referenced by the other handle.
		if out == ten {
			return runtime.Ok(args[0]), nil
		}
		return runtime.Ok(tensorHandle(out)), nil
	}, 1)

	set(p, "free", func(args []runtime.Value) (runtime.Value, error) {
		id, err := tensorID(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		hostTensors.Lock()
		ten := hostTensors.items[id]
		delete(hostTensors.items, id)
		hostTensors.Unlock()
		// Return owned storage to the tensor free-list pool when possible.
		tensor.Release(ten)
		return runtime.Ok(runtime.Bool(true)), nil
	}, 1)

	set(p, "info", func(args []runtime.Value) (runtime.Value, error) {
		ten, err := lookupTensor(args)
		if err != nil {
			return errRes(err.Error(), "tensor"), nil
		}
		id, _ := tensorID(args)
		return runtime.Ok(goToValue(map[string]any{
			"id":         id,
			"dtype":      string(ten.DType()),
			"shape":      ten.Shape(),
			"strides":    ten.Strides(),
			"offset":     ten.Offset(),
			"numel":      ten.Numel(),
			"contiguous": ten.IsContiguous(),
		})), nil
	}, 1)

	return p
}

func binaryTensor(args []runtime.Value, op func(*tensor.Tensor, *tensor.Tensor) (*tensor.Tensor, error)) (runtime.Value, error) {
	if len(args) < 2 {
		return errRes("tensor binary op expects two handles", "tensor"), nil
	}
	left, err := tensorFromHandle(args[0])
	if err != nil {
		return errRes(err.Error(), "tensor"), nil
	}
	right, err := tensorFromHandle(args[1])
	if err != nil {
		return errRes(err.Error(), "tensor"), nil
	}
	out, err := op(left, right)
	if err != nil {
		return errRes(err.Error(), "tensor"), nil
	}
	return runtime.Ok(tensorHandle(out)), nil
}

func tensorHandle(ten *tensor.Tensor) runtime.Value {
	id := fmt.Sprintf("tensor-%d", atomic.AddUint64(&hostTensors.next, 1))
	hostTensors.Lock()
	hostTensors.items[id] = ten
	hostTensors.Unlock()
	result := runtime.NewMap()
	mo := result.Obj.(*runtime.MapObj)
	mo.Keys = []string{"_tensor", "id", "dtype", "shape", "numel"}
	mo.Vals["_tensor"] = runtime.Bool(true)
	mo.Vals["id"] = runtime.Str(id)
	mo.Vals["dtype"] = runtime.Str(string(ten.DType()))
	shapeVals := make([]runtime.Value, len(ten.Shape()))
	for i, dim := range ten.Shape() {
		shapeVals[i] = runtime.Int(int64(dim))
	}
	mo.Vals["shape"] = runtime.List(shapeVals...)
	mo.Vals["numel"] = runtime.Int(ten.Numel())
	return result
}

func lookupTensor(args []runtime.Value) (*tensor.Tensor, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("tensor handle required")
	}
	return tensorFromHandle(args[0])
}

func tensorID(args []runtime.Value) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("tensor handle required")
	}
	return tensorHandleID(args[0])
}

func tensorHandleID(value runtime.Value) (string, error) {
	if value.Kind != runtime.KindMap {
		return "", fmt.Errorf("tensor handle must be a map from tensor.from_list")
	}
	mo := value.Obj.(*runtime.MapObj)
	id, ok := mo.Vals["id"]
	if !ok || id.Kind != runtime.KindStr || id.String() == "" {
		return "", fmt.Errorf("tensor handle id is invalid")
	}
	return id.String(), nil
}

func tensorFromHandle(value runtime.Value) (*tensor.Tensor, error) {
	id, err := tensorHandleID(value)
	if err != nil {
		return nil, err
	}
	hostTensors.RLock()
	ten := hostTensors.items[id]
	hostTensors.RUnlock()
	if ten == nil {
		return nil, fmt.Errorf("tensor handle %q is not loaded", id)
	}
	return ten, nil
}

func intList(value runtime.Value, label string) ([]int, error) {
	if value.Kind != runtime.KindList {
		return nil, fmt.Errorf("tensor %s must be a list", label)
	}
	items := value.Obj.(*runtime.ListObj).Items
	out := make([]int, len(items))
	for i, item := range items {
		if item.Kind != runtime.KindInt || item.I < 0 || item.I > int64(^uint(0)>>1) {
			return nil, fmt.Errorf("tensor %s[%d] must be a non-negative int", label, i)
		}
		out[i] = int(item.I)
	}
	return out, nil
}

func anyList(value runtime.Value) ([]any, error) {
	if value.Kind != runtime.KindList {
		return nil, fmt.Errorf("tensor data must be a list")
	}
	items := value.Obj.(*runtime.ListObj).Items
	out := make([]any, len(items))
	for i, item := range items {
		switch item.Kind {
		case runtime.KindBool:
			out[i] = item.B
		case runtime.KindInt:
			out[i] = item.I
		case runtime.KindFloat:
			out[i] = item.F
		default:
			return nil, fmt.Errorf("tensor data[%d] must be bool, int, or float", i)
		}
	}
	return out, nil
}
