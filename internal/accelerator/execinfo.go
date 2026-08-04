package accelerator

import (
	"encoding/json"
	"fmt"
)

// Execution status vocabulary reported by providers and surfaced to callers.
const (
	// StatusDevice means the operation ran on the requested device.
	StatusDevice = "device"
	// StatusFallback means the operation fell back to host CPU compute.
	StatusFallback = "fallback"
	// StatusUnavailable means the plugin or device is unavailable; this is
	// distinguishable from an operation failure (which is a Go error).
	StatusUnavailable = "unavailable"
	// StatusUnreported means the provider returned no execution reporting.
	StatusUnreported = "unreported"
)

// ExecInfo describes where one provider operation actually executed.
// Providers must report truthfully: a contradictory report (a device other
// than the requested one with no fallback flag) is a host-side error, and a
// missing report fails conformance.
type ExecInfo struct {
	Status          string `json:"status"`
	Device          string `json:"device,omitempty"`
	RequestedDevice string `json:"requested_device,omitempty"`
	Fallback        bool   `json:"fallback"`
	// Reported is true when the provider supplied device + fallback fields.
	Reported bool `json:"reported"`
}

// UnavailableExecInfo reports that no provider ran the operation at all
// (plugin disabled, unsupported build, or handle closed).
func UnavailableExecInfo() ExecInfo {
	return ExecInfo{Status: StatusUnavailable}
}

// parseExecInfo extracts execution reporting from one provider result
// (a run JSON payload or a weft_accel_exec_info document). A result without
// explicit device and fallback fields is unreported, never silently
// interpreted as device execution.
func parseExecInfo(raw []byte) ExecInfo {
	unreported := ExecInfo{Status: StatusUnreported}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return unreported
	}
	deviceRaw, ok := obj["device"]
	if !ok {
		return unreported
	}
	fallbackRaw, ok := obj["fallback"]
	if !ok {
		return unreported
	}
	var device string
	if err := json.Unmarshal(deviceRaw, &device); err != nil || device == "" {
		return unreported
	}
	var fallback bool
	if err := json.Unmarshal(fallbackRaw, &fallback); err != nil {
		return unreported
	}
	info := ExecInfo{
		Device:          device,
		RequestedDevice: device,
		Fallback:        fallback,
		Reported:        true,
	}
	if requestedRaw, ok := obj["requested_device"]; ok {
		var requested string
		if err := json.Unmarshal(requestedRaw, &requested); err == nil && requested != "" {
			info.RequestedDevice = requested
		}
	}
	if fallback {
		info.Status = StatusFallback
	} else {
		info.Status = StatusDevice
	}
	// A provider-supplied status is honored only when it agrees with the
	// fallback flag; disagreement is caught by validateExecInfo.
	if statusRaw, ok := obj["status"]; ok {
		var status string
		if err := json.Unmarshal(statusRaw, &status); err == nil {
			switch status {
			case StatusDevice, StatusFallback, StatusUnavailable:
				info.Status = status
			}
		}
	}
	return info
}

// validateExecInfo rejects contradictory provider reports. A provider that
// claims it ran on a device other than the requested one without setting the
// fallback flag is lying (or silently falling back); both are errors.
func validateExecInfo(provider string, info ExecInfo) error {
	if !info.Reported {
		return nil
	}
	if !info.Fallback && info.RequestedDevice != "" && info.Device != info.RequestedDevice {
		return fmt.Errorf("accelerator: provider %q reports device %q but no fallback for requested device %q",
			provider, info.Device, info.RequestedDevice)
	}
	if info.Status == StatusFallback && !info.Fallback {
		return fmt.Errorf("accelerator: provider %q reports status %q but fallback=false",
			provider, info.Status)
	}
	if info.Status == StatusDevice && info.Fallback {
		return fmt.Errorf("accelerator: provider %q reports status %q but fallback=true",
			provider, info.Status)
	}
	return nil
}
