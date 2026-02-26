package xboxelite2

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Alia5/VIIPER/device"
)

func TestBuildUSBInputReport_PaddleOrdering(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	cases := []struct {
		name    string
		buttons uint16
		wantBit uint32
	}{
		{name: "P2", buttons: ButtonP2, wantBit: 11},
		{name: "P1", buttons: ButtonP1, wantBit: 12},
		{name: "P4", buttons: ButtonP4, wantBit: 13},
		{name: "P3", buttons: ButtonP3, wantBit: 14},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := dev.buildUSBInputReport(InputState{Buttons: tc.buttons})
			bits := buttonBits(report)
			if bits != (1 << tc.wantBit) {
				t.Fatalf("unexpected button bits: got 0x%06X want 0x%06X", bits, 1<<tc.wantBit)
			}
		})
	}
}

func TestBuildUSBInputReport_AxesAndTriggers(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	state := InputState{
		LX: -32768,
		LY: 0,
		RX: 32767,
		RY: -1,
		LT: 255,
		RT: 128,
	}
	report := dev.buildUSBInputReport(state)

	if got := binary.LittleEndian.Uint16(report[1:3]); got != 0 {
		t.Fatalf("LX mismatch: got %d want 0", got)
	}
	if got := binary.LittleEndian.Uint16(report[3:5]); got != 32768 {
		t.Fatalf("LY mismatch: got %d want 32768", got)
	}
	if got := binary.LittleEndian.Uint16(report[5:7]); got != 65535 {
		t.Fatalf("RX mismatch: got %d want 65535", got)
	}
	if got := binary.LittleEndian.Uint16(report[7:9]); got != 32767 {
		t.Fatalf("RY mismatch: got %d want 32767", got)
	}

	if got := binary.LittleEndian.Uint16(report[9:11]); got != triggerU8ToU10(state.LT) {
		t.Fatalf("LT mismatch: got %d want %d", got, triggerU8ToU10(state.LT))
	}
	if got := binary.LittleEndian.Uint16(report[11:13]); got != triggerU8ToU10(state.RT) {
		t.Fatalf("RT mismatch: got %d want %d", got, triggerU8ToU10(state.RT))
	}
}

func TestBuildUSBInputReport_ShareAndHat(t *testing.T) {
	dev, err := New(nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	report := dev.buildUSBInputReport(InputState{
		DPad:     DPadUp | DPadRight,
		Reserved: ReservedShare,
	})

	if got := report[13] & DPadMask; got != DPadUSBUpRight {
		t.Fatalf("hat mismatch: got %d want %d", got, DPadUSBUpRight)
	}

	if bits := buttonBits(report); bits != (1 << 16) {
		t.Fatalf("share bit mismatch: got 0x%06X want 0x010000", bits)
	}
}

func TestNew_ProfileDefaultsAndOverrides(t *testing.T) {
	elite, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if got := elite.descriptor.Device.IDProduct; got != DefaultPIDElite2 {
		t.Fatalf("default PID mismatch: got 0x%04X want 0x%04X", got, DefaultPIDElite2)
	}

	one, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxOneElite},
	})
	if err != nil {
		t.Fatalf("New(xbox-one-elite) returned error: %v", err)
	}
	if got := one.descriptor.Device.IDProduct; got != DefaultPIDXboxOneElite {
		t.Fatalf("xbox-one-elite PID mismatch: got 0x%04X want 0x%04X", got, DefaultPIDXboxOneElite)
	}

	gip, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileElite2GIP},
	})
	if err != nil {
		t.Fatalf("New(elite2-gip) returned error: %v", err)
	}
	if got := gip.descriptor.Device.IDProduct; got != DefaultPIDElite2GIP {
		t.Fatalf("elite2-gip PID mismatch: got 0x%04X want 0x%04X", got, DefaultPIDElite2GIP)
	}
	if got := gip.GetDeviceSpecificArgs()["profile"]; got != ProfileElite2GIP {
		t.Fatalf("elite2-gip profile export mismatch: got %v want %s", got, ProfileElite2GIP)
	}

	inferred, err := New(&device.CreateOptions{
		IdVendor:  ptr(DefaultVID),
		IdProduct: ptr(DefaultPIDXboxOneElite),
	})
	if err != nil {
		t.Fatalf("New(vid/pid infer) returned error: %v", err)
	}
	if got := inferred.GetDeviceSpecificArgs()["profile"]; got != ProfileXboxOneElite {
		t.Fatalf("vid/pid profile infer mismatch: got %v want %s", got, ProfileXboxOneElite)
	}

	steam, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileSteamDeck},
	})
	if err != nil {
		t.Fatalf("New(steamdeck) returned error: %v", err)
	}
	if got := steam.descriptor.Device.IDVendor; got != DefaultVIDSteam {
		t.Fatalf("steamdeck VID mismatch: got 0x%04X want 0x%04X", got, DefaultVIDSteam)
	}
	if got := steam.descriptor.Device.IDProduct; got != DefaultPIDSteamDeck {
		t.Fatalf("steamdeck PID mismatch: got 0x%04X want 0x%04X", got, DefaultPIDSteamDeck)
	}

	steamGenericInferred, err := New(&device.CreateOptions{
		IdVendor:  ptr(DefaultVIDSteam),
		IdProduct: ptr(DefaultPIDSteamGeneric),
	})
	if err != nil {
		t.Fatalf("New(steam generic infer) returned error: %v", err)
	}
	if got := steamGenericInferred.GetDeviceSpecificArgs()["profile"]; got != ProfileSteamGeneric {
		t.Fatalf("steam generic profile infer mismatch: got %v want %s", got, ProfileSteamGeneric)
	}
	for _, pid := range []uint16{
		DefaultPIDSteamMsiClaw,
		DefaultPIDSteamLenovoLegionGo2,
		DefaultPIDSteamZotacZone,
		DefaultPIDSteamAsusRogAlly,
		DefaultPIDSteamLenovoLegionGo,
		DefaultPIDSteamLenovoLegionGoS,
	} {
		inferred, err := New(&device.CreateOptions{
			IdVendor:  ptr(DefaultVIDSteam),
			IdProduct: ptr(pid),
		})
		if err != nil {
			t.Fatalf("New(steam family infer pid=0x%04X) returned error: %v", pid, err)
		}
		if got := inferred.GetDeviceSpecificArgs()["profile"]; got != ProfileSteamGeneric {
			t.Fatalf("steam family profile infer mismatch for pid=0x%04X: got %v want %s", pid, got, ProfileSteamGeneric)
		}
		if got := inferred.descriptor.Device.IDProduct; got != pid {
			t.Fatalf("steam family PID override mismatch for pid=0x%04X: got 0x%04X", pid, got)
		}
	}

	series, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxSeries},
		IdProduct:      ptr(uint16(0xCAFE)),
	})
	if err != nil {
		t.Fatalf("New(xbox-series) returned error: %v", err)
	}
	if got := series.descriptor.Device.IDProduct; got != 0xCAFE {
		t.Fatalf("IdProduct override mismatch: got 0x%04X want 0xCAFE", got)
	}
	if got := series.GetDeviceSpecificArgs()["profile"]; got != ProfileXboxSeries {
		t.Fatalf("profile export mismatch: got %v want %s", got, ProfileXboxSeries)
	}

	if _, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": "not-a-profile"},
	}); err == nil {
		t.Fatal("expected unsupported profile error, got nil")
	}
}

func TestProfileDescriptorVariants(t *testing.T) {
	elite, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	one, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxOneElite},
	})
	if err != nil {
		t.Fatalf("New(xbox-one-elite) returned error: %v", err)
	}
	series, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxSeries},
	})
	if err != nil {
		t.Fatalf("New(xbox-series) returned error: %v", err)
	}
	steam, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileSteamDeck},
	})
	if err != nil {
		t.Fatalf("New(steamdeck) returned error: %v", err)
	}

	if bytes.Equal(elite.descriptor.Interfaces[0].HID.ReportRaw, one.descriptor.Interfaces[0].HID.ReportRaw) {
		t.Fatal("elite2 and xbox-one-elite descriptors should differ")
	}
	if bytes.Equal(one.descriptor.Interfaces[0].HID.ReportRaw, series.descriptor.Interfaces[0].HID.ReportRaw) {
		t.Fatal("xbox-one-elite and xbox-series descriptors should differ")
	}
	if got := len(steam.descriptor.Interfaces[0].HID.ReportRaw); got != len(steamDeckControllerHIDDescriptor) {
		t.Fatalf("steam descriptor len mismatch: got %d want %d", got, len(steamDeckControllerHIDDescriptor))
	}
	if got := steam.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize; got != 64 {
		t.Fatalf("steam endpoint packet size mismatch: got %d want 64", got)
	}
	if got := steam.descriptor.Strings[1]; got != "Valve Software" {
		t.Fatalf("steam manufacturer mismatch: got %q", got)
	}

	report := steam.buildUSBInputReport(InputState{})
	if len(report) != InputReportSizeSteamDeck {
		t.Fatalf("steam report len mismatch: got %d want %d", len(report), InputReportSizeSteamDeck)
	}
	if report[2] != SteamDeckInputReportType {
		t.Fatalf("steam report type mismatch: got 0x%02X want 0x%02X", report[2], SteamDeckInputReportType)
	}
}

func TestBuildUSBInputReport_ProfileButtonLayouts(t *testing.T) {
	series, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxSeries},
	})
	if err != nil {
		t.Fatalf("New(xbox-series) returned error: %v", err)
	}
	seriesBits := buttonBits(series.buildUSBInputReport(InputState{
		Buttons:  ButtonA | ButtonP1 | ButtonP2 | ButtonP3 | ButtonP4,
		Reserved: ReservedShare,
	}))
	if want := uint32((1 << 0) | (1 << 11)); seriesBits != want {
		t.Fatalf("xbox-series bits mismatch: got 0x%06X want 0x%06X", seriesBits, want)
	}

	one, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileXboxOneElite},
	})
	if err != nil {
		t.Fatalf("New(xbox-one-elite) returned error: %v", err)
	}
	oneBits := buttonBits(one.buildUSBInputReport(InputState{
		Buttons:  ButtonA | ButtonP1,
		Reserved: ReservedShare,
	}))
	if oneBits&(1<<16) != 0 {
		t.Fatalf("xbox-one-elite should not expose share bit, got 0x%06X", oneBits)
	}
	if oneBits&(1<<12) == 0 {
		t.Fatalf("xbox-one-elite should expose P1 bit, got 0x%06X", oneBits)
	}
}

func TestBuildUSBInputReport_SteamDeckBitPacking(t *testing.T) {
	steam, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileSteamGeneric},
	})
	if err != nil {
		t.Fatalf("New(steamdeck-generic) returned error: %v", err)
	}

	// Steam Deck input bytes use msb0 numbering.
	report := steam.buildUSBInputReport(InputState{Buttons: ButtonA})
	if got := report[8]; got != 0x80 {
		t.Fatalf("A bit mismatch: got 0x%02X want 0x80", got)
	}

	report = steam.buildUSBInputReport(InputState{Buttons: ButtonX})
	if got := report[8]; got != 0x40 {
		t.Fatalf("X bit mismatch: got 0x%02X want 0x40", got)
	}

	report = steam.buildUSBInputReport(InputState{Buttons: ButtonB})
	if got := report[8]; got != 0x20 {
		t.Fatalf("B bit mismatch: got 0x%02X want 0x20", got)
	}

	report = steam.buildUSBInputReport(InputState{Buttons: ButtonY})
	if got := report[8]; got != 0x10 {
		t.Fatalf("Y bit mismatch: got 0x%02X want 0x10", got)
	}

	report = steam.buildUSBInputReport(InputState{Buttons: ButtonP2})
	if got := report[9]; got != 0x80 {
		t.Fatalf("L5 bit mismatch: got 0x%02X want 0x80", got)
	}

	report = steam.buildUSBInputReport(InputState{DPad: DPadUp | DPadRight})
	if got := report[9]; got != 0x03 {
		t.Fatalf("DPad up+right bits mismatch: got 0x%02X want 0x03", got)
	}

	report = steam.buildUSBInputReport(InputState{Reserved: ReservedShare})
	if got := report[14]; got != 0x04 {
		t.Fatalf("quick access bit mismatch: got 0x%02X want 0x04", got)
	}

	report = steam.buildUSBInputReport(InputState{
		LY: -1234, RY: 2345,
		GyroX: 111, GyroY: -222, GyroZ: 333,
		AccelX: -444, AccelY: 555, AccelZ: -666,
	})
	if got := int16(binary.LittleEndian.Uint16(report[50:52])); got != -1234 {
		t.Fatalf("steam LY mismatch: got %d want -1234", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[54:56])); got != 2345 {
		t.Fatalf("steam RY mismatch: got %d want 2345", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[24:26])); got != -444 {
		t.Fatalf("steam accelX mismatch: got %d want -444", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[26:28])); got != 555 {
		t.Fatalf("steam accelY mismatch: got %d want 555", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[28:30])); got != -666 {
		t.Fatalf("steam accelZ mismatch: got %d want -666", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[30:32])); got != 111 {
		t.Fatalf("steam gyroX mismatch: got %d want 111", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[32:34])); got != -222 {
		t.Fatalf("steam gyroY mismatch: got %d want -222", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[34:36])); got != 333 {
		t.Fatalf("steam gyroZ mismatch: got %d want 333", got)
	}

	report = steam.buildUSBInputReport(InputState{
		TouchFlags: TouchFlagRightPadTouch | TouchFlagRightPadPress,
		RPadX:      1234,
		RPadY:      -2345,
		RPadForce:  9000,
	})
	if got := report[10]; got != 0x14 {
		t.Fatalf("right touch/press bits mismatch: got 0x%02X want 0x14", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[20:22])); got != 1234 {
		t.Fatalf("steam r_pad_x mismatch: got %d want 1234", got)
	}
	if got := int16(binary.LittleEndian.Uint16(report[22:24])); got != -2345 {
		t.Fatalf("steam r_pad_y mismatch: got %d want -2345", got)
	}
	if got := binary.LittleEndian.Uint16(report[58:60]); got != 9000 {
		t.Fatalf("steam r_pad_force mismatch: got %d want 9000", got)
	}
}

func buttonBits(report []byte) uint32 {
	return uint32(report[14]) | (uint32(report[15]) << 8) | (uint32(report[16]) << 16)
}

func ptr[T any](v T) *T {
	return &v
}
