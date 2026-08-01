package main

/*
#include <stdint.h>
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/Alia5/VIIPER/device/dualshock4"
	"github.com/Alia5/VIIPER/device/xbox360"
)

// Typed fast path for high-frequency xbox360 input updates.
//
// The generic viiper_device_set_input takes the global mutex, does a map
// lookup, copies the buffer via C.GoBytes (one heap allocation per call) and
// type-switches on every report. For callers submitting at 125-1000 Hz that
// overhead dominates. The fast path resolves the device once into a handle;
// per-report cost is then a single atomic load plus the state update.
//
// Handles are 1-based indices into a copy-on-write slice guarded by the
// global mu on the (cold) open path and read lock-free on the (hot) submit
// path. Handles are NOT invalidated by viiper_device_remove: submitting to a
// removed device is safe but the reports go nowhere. viiper_shutdown clears
// the table; handles must be re-opened after re-init.

var (
	x360Handles atomic.Pointer[[]*xbox360.Xbox360]
	ds4Handles  atomic.Pointer[[]*dualshock4.DualShock4]
	fastHandles atomic.Pointer[[]*deviceInfo]
)

// clearX360Handles drops all fast-path handles (all device types). Called
// from viiper_shutdown with the global mu held.
func clearX360Handles() {
	x360Handles.Store(nil)
	ds4Handles.Store(nil)
	fastHandles.Store(nil)
}

// viiper_device_open_fast resolves any device into a fast-path handle for
// viiper_device_set_input_fast. Unlike the typed x360/ds4 variants it works
// for every device type: the submit call takes the same wire format as the
// generic viiper_device_set_input, but skips the global mutex, map lookup
// and buffer copy. Same lifetime rules as the typed handles.
//
//export viiper_device_open_fast
func viiper_device_open_fast(busID C.uint32_t, deviceID C.uint32_t, outHandle *C.uint32_t) C.int {
	mu.Lock()
	defer mu.Unlock()

	if server == nil {
		return setError(fmt.Errorf("not initialized"))
	}

	info, ok := devices[deviceKey{busID: uint32(busID), devID: uint32(deviceID)}]
	if !ok {
		return setError(fmt.Errorf("device %d-%d not found", busID, deviceID))
	}

	var s []*deviceInfo
	if old := fastHandles.Load(); old != nil {
		s = append(s, *old...)
	}
	s = append(s, info)
	fastHandles.Store(&s)

	if outHandle != nil {
		*outHandle = C.uint32_t(len(s))
	}
	return 0
}

// viiper_device_set_input_fast is the type-agnostic hot path: a zero-copy
// view of the caller's buffer is decoded via the same dispatch as the
// generic call. Never touches lastError; returns -1 for an invalid handle,
// -2 for a decode/apply error.
//
//export viiper_device_set_input_fast
func viiper_device_set_input_fast(handle C.uint32_t, data *C.uint8_t, length C.int) C.int {
	s := fastHandles.Load()
	if s == nil || handle == 0 || int(handle) > len(*s) {
		return -1
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))
	if err := applyInput((*s)[handle-1], buf); err != nil {
		return -2
	}
	return 0
}

//export viiper_device_open_x360
func viiper_device_open_x360(busID C.uint32_t, deviceID C.uint32_t, outHandle *C.uint32_t) C.int {
	mu.Lock()
	defer mu.Unlock()

	if server == nil {
		return setError(fmt.Errorf("not initialized"))
	}

	info, ok := devices[deviceKey{busID: uint32(busID), devID: uint32(deviceID)}]
	if !ok {
		return setError(fmt.Errorf("device %d-%d not found", busID, deviceID))
	}
	xdev, ok := info.dev.(*xbox360.Xbox360)
	if !ok {
		return setError(fmt.Errorf("device %d-%d is %q, not xbox360", busID, deviceID, info.typeName))
	}

	var s []*xbox360.Xbox360
	if old := x360Handles.Load(); old != nil {
		s = append(s, *old...)
	}
	s = append(s, xdev)
	x360Handles.Store(&s)

	if outHandle != nil {
		*outHandle = C.uint32_t(len(s))
	}
	return 0
}

// viiper_device_set_input_x360 is the allocation-free hot path. It does not
// take the global mutex and therefore never touches lastError; it returns -1
// for an invalid handle and 0 on success.
//
//export viiper_device_set_input_x360
func viiper_device_set_input_x360(handle C.uint32_t, buttons C.uint32_t, lt C.uint8_t, rt C.uint8_t, lx C.int16_t, ly C.int16_t, rx C.int16_t, ry C.int16_t) C.int {
	s := x360Handles.Load()
	if s == nil || handle == 0 || int(handle) > len(*s) {
		return -1
	}
	(*s)[handle-1].UpdateInputState(xbox360.InputState{
		Buttons: uint32(buttons),
		LT:      uint8(lt),
		RT:      uint8(rt),
		LX:      int16(lx),
		LY:      int16(ly),
		RX:      int16(rx),
		RY:      int16(ry),
	})
	return 0
}

//export viiper_device_open_ds4
func viiper_device_open_ds4(busID C.uint32_t, deviceID C.uint32_t, outHandle *C.uint32_t) C.int {
	mu.Lock()
	defer mu.Unlock()

	if server == nil {
		return setError(fmt.Errorf("not initialized"))
	}

	info, ok := devices[deviceKey{busID: uint32(busID), devID: uint32(deviceID)}]
	if !ok {
		return setError(fmt.Errorf("device %d-%d not found", busID, deviceID))
	}
	ds4, ok := info.dev.(*dualshock4.DualShock4)
	if !ok {
		return setError(fmt.Errorf("device %d-%d is %q, not dualshock4", busID, deviceID, info.typeName))
	}

	var s []*dualshock4.DualShock4
	if old := ds4Handles.Load(); old != nil {
		s = append(s, *old...)
	}
	s = append(s, ds4)
	ds4Handles.Store(&s)

	if outHandle != nil {
		*outHandle = C.uint32_t(len(s))
	}
	return 0
}

// viiper_device_set_input_ds4 is the ds4 hot path: same 31-byte wire format
// as the generic call, but no global mutex, no map lookup, and a zero-copy
// view of the caller's buffer (parsed before return, never retained). Like
// the x360 fast path it never touches lastError; returns -1 on an invalid
// handle, -2 on a malformed buffer.
//
//export viiper_device_set_input_ds4
func viiper_device_set_input_ds4(handle C.uint32_t, data *C.uint8_t, length C.int) C.int {
	s := ds4Handles.Load()
	if s == nil || handle == 0 || int(handle) > len(*s) {
		return -1
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))
	var state dualshock4.InputState
	if err := state.UnmarshalBinary(buf); err != nil {
		return -2
	}
	(*s)[handle-1].UpdateInputState(&state)
	return 0
}
