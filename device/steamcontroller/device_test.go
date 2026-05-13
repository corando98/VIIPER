package steamcontroller

import (
	"encoding/binary"
	"testing"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/xboxelite2"
	elite2state "github.com/Alia5/VIIPER/internal/inputstate/elite2"
)

func TestNew_DefaultProfileIsSteamDeck(t *testing.T) {
	steam, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if got := steam.descriptor.Device.IDVendor; got != DefaultVIDSteam {
		t.Fatalf("default VID mismatch: got 0x%04X want 0x%04X", got, DefaultVIDSteam)
	}
	if got := steam.descriptor.Device.IDProduct; got != DefaultPIDSteamDeck {
		t.Fatalf("default PID mismatch: got 0x%04X want 0x%04X", got, DefaultPIDSteamDeck)
	}
	if got := steam.GetDeviceSpecificArgs()["profile"]; got != ProfileSteamDeck {
		t.Fatalf("default profile mismatch: got %v want %s", got, ProfileSteamDeck)
	}
}

func TestNew_ProfileDefaultsAndOverrides(t *testing.T) {
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

	if _, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": "not-a-profile"},
	}); err == nil {
		t.Fatal("expected unsupported profile error, got nil")
	}
}

func TestProfileDescriptorVariants(t *testing.T) {
	steam, err := New(&device.CreateOptions{
		DeviceSpecific: map[string]any{"profile": ProfileSteamDeck},
	})
	if err != nil {
		t.Fatalf("New(steamdeck) returned error: %v", err)
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

	report := steam.buildSteamDeckInputReport(elite2state.InputState{})
	if len(report) != InputReportSizeSteamDeck {
		t.Fatalf("steam report len mismatch: got %d want %d", len(report), InputReportSizeSteamDeck)
	}
	if report[2] != SteamDeckInputReportType {
		t.Fatalf("steam report type mismatch: got 0x%02X want 0x%02X", report[2], SteamDeckInputReportType)
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
	report := steam.buildSteamDeckInputReport(elite2state.InputState{Buttons: xboxelite2.ButtonA})
	if got := report[8]; got != 0x80 {
		t.Fatalf("A bit mismatch: got 0x%02X want 0x80", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{Buttons: xboxelite2.ButtonX})
	if got := report[8]; got != 0x40 {
		t.Fatalf("X bit mismatch: got 0x%02X want 0x40", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{Buttons: xboxelite2.ButtonB})
	if got := report[8]; got != 0x20 {
		t.Fatalf("B bit mismatch: got 0x%02X want 0x20", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{Buttons: xboxelite2.ButtonY})
	if got := report[8]; got != 0x10 {
		t.Fatalf("Y bit mismatch: got 0x%02X want 0x10", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{Buttons: xboxelite2.ButtonP2})
	if got := report[9]; got != 0x80 {
		t.Fatalf("L5 bit mismatch: got 0x%02X want 0x80", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{DPad: xboxelite2.DPadUp | xboxelite2.DPadRight})
	if got := report[9]; got != 0x03 {
		t.Fatalf("DPad up+right bits mismatch: got 0x%02X want 0x03", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{Reserved: xboxelite2.ReservedShare})
	if got := report[14]; got != 0x04 {
		t.Fatalf("quick access bit mismatch: got 0x%02X want 0x04", got)
	}

	report = steam.buildSteamDeckInputReport(elite2state.InputState{
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

	report = steam.buildSteamDeckInputReport(elite2state.InputState{
		TouchFlags: elite2state.TouchFlagRightPadTouch | elite2state.TouchFlagRightPadPress,
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

func TestParseProfile_Aliases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"steamdeck", ProfileSteamDeck},
		{"steam-deck", ProfileSteamDeck},
		{"deck", ProfileSteamDeck},
		{"valve-steamdeck", ProfileSteamDeck},
		{"1205", ProfileSteamDeck},
		{"0x1205", ProfileSteamDeck},
		{"", ProfileSteamDeck},
		{"steamdeck-generic", ProfileSteamGeneric},
		{"steam-generic", ProfileSteamGeneric},
		{"steam-controller", ProfileSteamGeneric},
		{"deck-uhid", ProfileSteamGeneric},
		{"12f0", ProfileSteamGeneric},
		{"0x12f0", ProfileSteamGeneric},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseProfile(c.in)
			if err != nil {
				t.Fatalf("parseProfile(%q) returned error: %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("parseProfile(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
