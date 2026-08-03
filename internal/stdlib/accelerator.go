//go:build !js

package stdlib

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/loreste/weft/internal/accelerator"
	"github.com/loreste/weft/internal/runtime"
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
		nativeAccelerators.RLock()
		plugin := nativeAccelerators.items[id]
		nativeAccelerators.RUnlock()
		if plugin == nil {
			return errRes("accelerator plugin is not loaded", "accelerator"), nil
		}
		input, err := json.Marshal(valueToGo(args[2]))
		if err != nil {
			return errRes("accelerator input: "+err.Error(), "accelerator"), nil
		}
		output, err := plugin.RunJSON(args[1].String(), input)
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
