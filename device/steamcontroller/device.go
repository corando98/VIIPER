package steamcontroller

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/device/xboxelite2"
	elite2state "github.com/Alia5/VIIPER/internal/inputstate/elite2"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

// SteamController emulates a Valve Steam Controller-family device over
// USBIP. The wire input format is the shared Elite-2-derived layout
// defined in internal/inputstate/elite2 (Xbox profiles ignore the
// touchpad/IMU tail). The host-facing HID report is the 64-byte Steam
// Deck vendor input report.
type SteamController struct {
	inputState *elite2state.InputState
	stateMu    sync.Mutex
	outputFunc func(elite2state.OutputState)
	descriptor usb.Descriptor
	profile    string
	frame      uint32

	// Steam IMU timing: zero stale gyro during keepalive gaps so the
	// host doesn't continue rotating when the client stops sending new
	// frames (this matches the real Deck's behaviour during disconnects).
	inputVersion  uint32 // bumped in UpdateInputState
	lastReportedV uint32 // version at last HandleTransfer
	lastInputNano int64  // UnixNano of last new input data
	pollCount     int64  // total polls for rate measurement
	pollLogNano   int64  // start time for rate measurement
}

type steamControllerCreateOptions struct {
	Profile *string `json:"profile"`
}

func New(o *device.CreateOptions) (*SteamController, error) {
	d := &SteamController{
		descriptor: cloneDescriptor(defaultDescriptor),
		profile:    ProfileSteamDeck,
	}
	d.applyProfileDefaults(ProfileSteamDeck)
	profileExplicit := false

	if o != nil && o.DeviceSpecific != nil {
		var args steamControllerCreateOptions
		data, err := json.Marshal(o.DeviceSpecific)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON payload: %w", err)
		}
		if err := json.Unmarshal(data, &args); err != nil {
			return nil, fmt.Errorf("invalid JSON payload: %w", err)
		}
		if args.Profile != nil {
			profile, err := parseProfile(*args.Profile)
			if err != nil {
				return nil, err
			}
			d.applyProfileDefaults(profile)
			profileExplicit = true
		}
	}

	if o != nil {
		// Older clients may only expose VID/PID overrides.
		// If no explicit profile was requested, infer profile metadata for known IDs.
		if !profileExplicit {
			vid := d.descriptor.Device.IDVendor
			pid := d.descriptor.Device.IDProduct
			if o.IdVendor != nil {
				vid = *o.IdVendor
			}
			if o.IdProduct != nil {
				pid = *o.IdProduct
			}
			if inferred, ok := profileForIDs(vid, pid); ok {
				d.applyProfileDefaults(inferred)
			}
		}
		if o.IdVendor != nil {
			d.descriptor.Device.IDVendor = *o.IdVendor
		}
		if o.IdProduct != nil {
			d.descriptor.Device.IDProduct = *o.IdProduct
		}
	}

	return d, nil
}

func parseProfile(profile string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", ProfileSteamDeck, "steam-deck", "deck", "valve-steamdeck", "1205", "0x1205":
		return ProfileSteamDeck, nil
	case ProfileSteamGeneric, "steam-generic", "steam-controller", "deck-uhid", "12f0", "0x12f0":
		return ProfileSteamGeneric, nil
	default:
		return "", fmt.Errorf(
			"unsupported steamcontroller profile %q (supported: %q, %q)",
			profile,
			ProfileSteamDeck,
			ProfileSteamGeneric,
		)
	}
}

func profileForIDs(vid, pid uint16) (string, bool) {
	if vid != DefaultVIDSteam {
		return "", false
	}
	switch pid {
	case DefaultPIDSteamDeck:
		return ProfileSteamDeck, true
	case DefaultPIDSteamGeneric,
		DefaultPIDSteamMsiClaw,
		DefaultPIDSteamLenovoLegionGo2,
		DefaultPIDSteamZotacZone,
		DefaultPIDSteamAsusRogAlly,
		DefaultPIDSteamLenovoLegionGo,
		DefaultPIDSteamLenovoLegionGoS:
		return ProfileSteamGeneric, true
	default:
		return "", false
	}
}

func cloneDescriptor(src usb.Descriptor) usb.Descriptor {
	dst := src

	dst.Interfaces = make([]usb.InterfaceConfig, len(src.Interfaces))
	copy(dst.Interfaces, src.Interfaces)
	for i := range src.Interfaces {
		if src.Interfaces[i].HID != nil {
			hid := *src.Interfaces[i].HID
			hid.Descriptor.Descriptors = append([]usb.HIDSubDescriptor(nil), src.Interfaces[i].HID.Descriptor.Descriptors...)
			hid.ReportRaw = append([]byte(nil), src.Interfaces[i].HID.ReportRaw...)
			dst.Interfaces[i].HID = &hid
		}
		dst.Interfaces[i].Endpoints = append([]usb.EndpointDescriptor(nil), src.Interfaces[i].Endpoints...)
		dst.Interfaces[i].ClassDescriptors = append([]usb.ClassSpecificDescriptor(nil), src.Interfaces[i].ClassDescriptors...)
	}

	dst.Strings = make(map[uint8]string, len(src.Strings))
	for k, v := range src.Strings {
		dst.Strings[k] = v
	}

	return dst
}

func (x *SteamController) applyProfileDefaults(profile string) {
	x.profile = profile
	x.descriptor = cloneDescriptor(defaultDescriptor)

	switch profile {
	case ProfileSteamDeck:
		x.descriptor.Device.IDVendor = DefaultVIDSteam
		x.descriptor.Device.IDProduct = DefaultPIDSteamDeck
		x.descriptor.Strings[1] = "Valve Software"
		x.descriptor.Strings[2] = "Steam Deck Controller"
		x.descriptor.Strings[3] = "VIIPER-SD-05"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), steamDeckControllerHIDDescriptor...)
		x.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize = 64
		x.descriptor.Interfaces[0].Endpoints[1].WMaxPacketSize = 64
	case ProfileSteamGeneric:
		x.descriptor.Device.IDVendor = DefaultVIDSteam
		x.descriptor.Device.IDProduct = DefaultPIDSteamGeneric
		x.descriptor.Strings[1] = "Valve Software"
		x.descriptor.Strings[2] = "Generic Steam Controller"
		x.descriptor.Strings[3] = "VIIPER-SGEN-05"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), steamDeckControllerHIDDescriptor...)
		x.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize = 64
		x.descriptor.Interfaces[0].Endpoints[1].WMaxPacketSize = 64
	default:
		x.descriptor.Device.IDVendor = DefaultVIDSteam
		x.descriptor.Device.IDProduct = DefaultPIDSteamDeck
		x.descriptor.Strings[1] = "Valve Software"
		x.descriptor.Strings[2] = "Steam Deck Controller"
		x.descriptor.Strings[3] = "VIIPER-SD-05"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), steamDeckControllerHIDDescriptor...)
		x.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize = 64
		x.descriptor.Interfaces[0].Endpoints[1].WMaxPacketSize = 64
	}
}

func (x *SteamController) SetOutputCallback(f func(elite2state.OutputState)) {
	x.outputFunc = f
}

func (x *SteamController) UpdateInputState(state *elite2state.InputState) {
	x.stateMu.Lock()
	defer x.stateMu.Unlock()
	x.inputState = state
	atomic.AddUint32(&x.inputVersion, 1)
}

func (x *SteamController) HandleTransfer(ep uint32, dir uint32, out []byte) []byte {
	if dir == usbip.DirIn {
		switch ep {
		case 1: // 0x81 - main input reports
			x.stateMu.Lock()
			var st elite2state.InputState
			if x.inputState != nil {
				st = *x.inputState
			}
			x.stateMu.Unlock()

			now := time.Now().UnixNano()

			// One-shot poll rate measurement: log after 250 polls.
			if atomic.LoadInt64(&x.pollCount) == 0 {
				atomic.StoreInt64(&x.pollLogNano, now)
			}
			cnt := atomic.AddInt64(&x.pollCount, 1)
			if cnt == 250 {
				logStart := atomic.LoadInt64(&x.pollLogNano)
				elapsedS := float64(now-logStart) / 1e9
				if elapsedS > 0 {
					rate := 249.0 / elapsedS
					slog.Info("Steam USB poll rate measured", "hz", fmt.Sprintf("%.1f", rate))
				}
			}

			// Track when new input data arrives (for stale gyro zeroing).
			v := atomic.LoadUint32(&x.inputVersion)
			if v != x.lastReportedV {
				x.lastReportedV = v
				atomic.StoreInt64(&x.lastInputNano, now)
			}

			// Zero gyro when input is stale for >20ms (keepalive gap).
			if now-atomic.LoadInt64(&x.lastInputNano) > 20_000_000 {
				st.GyroX = 0
				st.GyroY = 0
				st.GyroZ = 0
			}

			return x.buildSteamDeckInputReport(st)
		default:
			return nil
		}
	}

	if dir == usbip.DirOut && ep == 1 {
		x.parseSteamDeckOutputReport(out)
	}

	return nil
}

func (x *SteamController) parseSteamDeckOutputReport(out []byte) {
	start := 0
	// Some stacks prefix an extra report-id byte (0x00) before command payload.
	if len(out) >= 10 && out[0] == 0x00 {
		start = 1
	}
	if len(out) < start+2 {
		return
	}
	cmdType := out[start]

	// Log all incoming Steam Deck commands for protocol analysis.
	if cmdType != SteamDeckRumbleCommandType {
		hex := fmt.Sprintf("%02X", out[start:])
		if len(hex) > 128 {
			hex = hex[:128] + "..."
		}
		slog.Info("SteamDeck: recv command", "type", fmt.Sprintf("0x%02X", cmdType), "len", len(out)-start, "hex", hex)
	}

	if len(out) < start+9 || cmdType != SteamDeckRumbleCommandType {
		return
	}

	left := binary.LittleEndian.Uint16(out[start+5 : start+7])
	right := binary.LittleEndian.Uint16(out[start+7 : start+9])
	feedback := elite2state.OutputState{
		RumbleLeft:         uint8(left >> 8),
		RumbleRight:        uint8(right >> 8),
		RumbleTriggerLeft:  0,
		RumbleTriggerRight: 0,
	}
	if x.outputFunc != nil {
		x.outputFunc(feedback)
	}
}

func (x *SteamController) HandleControl(bmRequestType, bRequest uint8, wValue, _ uint16, wLength uint16, data []byte) ([]byte, bool) {
	const (
		hidGetReport = 0x01
		hidSetReport = 0x09
	)

	const (
		reportTypeInput   = 0x01
		reportTypeOutput  = 0x02
		reportTypeFeature = 0x03
	)

	reportType := uint8(wValue >> 8)
	reportID := uint8(wValue & 0xFF)

	if bmRequestType == 0xA1 && bRequest == hidGetReport {
		if reportType == reportTypeInput {
			x.stateMu.Lock()
			var st elite2state.InputState
			if x.inputState != nil {
				st = *x.inputState
			}
			x.stateMu.Unlock()
			report := x.buildSteamDeckInputReport(st)
			if wLength > 0 && int(wLength) < len(report) {
				return report[:wLength], true
			}
			return report, true
		}

		if reportType == reportTypeFeature {
			slog.Info("SteamDeck: GetFeature", "reportID", fmt.Sprintf("0x%02X", reportID), "wLength", wLength)
			size := int(wLength)
			if size <= 0 {
				size = InputReportSizeSteamDeck
			}
			buf := make([]byte, size)
			return buf, true
		}
	}

	if bmRequestType == 0x21 && bRequest == hidSetReport {
		if reportType == reportTypeOutput || reportType == reportTypeFeature {
			x.parseSteamDeckOutputReport(data)
			return nil, true
		}
	}

	slog.Warn("SteamController: unsupported control request",
		"bmRequestType", bmRequestType,
		"bRequest", bRequest,
		"reportType", reportType,
		"reportID", reportID)

	return nil, false
}

func (x *SteamController) GetDescriptor() *usb.Descriptor {
	return &x.descriptor
}

func (x *SteamController) GetDeviceSpecificArgs() map[string]any {
	return map[string]any{
		"profile": x.profile,
	}
}

// buildSteamDeckInputReport packs an InputState into a 64-byte Steam Deck
// vendor input report. Bit layout / byte offsets mirror InputPlumber's
// captures of the real Deck protocol.
func (x *SteamController) buildSteamDeckInputReport(s elite2state.InputState) []byte {
	b := make([]byte, InputReportSizeSteamDeck)
	b[0] = SteamDeckInputMajorVersion
	b[1] = SteamDeckInputMinorVersion
	b[2] = SteamDeckInputReportType
	b[3] = InputReportSizeSteamDeck
	// Monotonic frame counter: increment every poll so each report is unique.
	// SDL uses counter for duplicate detection; duplicates cause instability.
	binary.LittleEndian.PutUint32(b[4:8], atomic.AddUint32(&x.frame, 1))

	setBit := func(byteIndex int, bit uint8, on bool) {
		if on {
			// Steam Deck button bytes are specified with msb0 bit numbering
			// (bit 0 is 0x80, bit 7 is 0x01).
			if bit < 8 {
				b[byteIndex] |= 1 << (7 - bit)
			}
		}
	}

	// Byte 8
	setBit(8, 0, s.Buttons&xboxelite2.ButtonA != 0)
	setBit(8, 1, s.Buttons&xboxelite2.ButtonX != 0)
	setBit(8, 2, s.Buttons&xboxelite2.ButtonB != 0)
	setBit(8, 3, s.Buttons&xboxelite2.ButtonY != 0)
	setBit(8, 4, s.Buttons&xboxelite2.ButtonLB != 0)
	setBit(8, 5, s.Buttons&xboxelite2.ButtonRB != 0)
	// Match SteamDeck target behavior: digital trigger bits should only activate
	// close to full pull to avoid noisy analog jitter being interpreted as button chords.
	const steamDigitalTriggerThreshold = 204 // ~80% of 255
	setBit(8, 6, s.LT >= steamDigitalTriggerThreshold)
	setBit(8, 7, s.RT >= steamDigitalTriggerThreshold)

	// Byte 9
	setBit(9, 0, s.Buttons&xboxelite2.ButtonP2 != 0)    // L5
	setBit(9, 1, s.Buttons&xboxelite2.ButtonStart != 0) // menu
	setBit(9, 2, s.Buttons&xboxelite2.ButtonGuide != 0) // steam
	setBit(9, 3, s.Buttons&xboxelite2.ButtonBack != 0)  // options
	setBit(9, 4, s.DPad&xboxelite2.DPadDown != 0)
	setBit(9, 5, s.DPad&xboxelite2.DPadLeft != 0)
	setBit(9, 6, s.DPad&xboxelite2.DPadRight != 0)
	setBit(9, 7, s.DPad&xboxelite2.DPadUp != 0)

	// Byte 10 / 11 / 13 / 14
	setBit(10, 1, s.Buttons&xboxelite2.ButtonLThumb != 0)
	setBit(10, 3, s.TouchFlags&elite2state.TouchFlagRightPadTouch != 0) // r_pad_touch
	setBit(10, 5, s.TouchFlags&elite2state.TouchFlagRightPadPress != 0) // r_pad_press
	setBit(10, 7, s.Buttons&xboxelite2.ButtonP1 != 0)                  // R5
	setBit(11, 5, s.Buttons&xboxelite2.ButtonRThumb != 0)
	setBit(13, 5, s.Buttons&xboxelite2.ButtonP3 != 0)       // R4
	setBit(13, 6, s.Buttons&xboxelite2.ButtonP4 != 0)       // L4
	setBit(14, 5, s.Reserved&xboxelite2.ReservedShare != 0) // quick_access

	// Right touchpad coordinates (signed i16, Steam format).
	binary.LittleEndian.PutUint16(b[20:22], uint16(s.RPadX))
	binary.LittleEndian.PutUint16(b[22:24], uint16(s.RPadY))

	// Analog triggers (0..32767 on Deck reports)
	binary.LittleEndian.PutUint16(b[44:46], uint16(s.LT)*128)
	binary.LittleEndian.PutUint16(b[46:48], uint16(s.RT)*128)

	// IMU (raw i16 values).
	binary.LittleEndian.PutUint16(b[24:26], uint16(s.AccelX))
	binary.LittleEndian.PutUint16(b[26:28], uint16(s.AccelY))
	binary.LittleEndian.PutUint16(b[28:30], uint16(s.AccelZ))
	binary.LittleEndian.PutUint16(b[30:32], uint16(s.GyroX)) // pitch
	binary.LittleEndian.PutUint16(b[32:34], uint16(s.GyroY)) // yaw
	binary.LittleEndian.PutUint16(b[34:36], uint16(s.GyroZ)) // roll

	// Sticks.
	binary.LittleEndian.PutUint16(b[48:50], uint16(s.LX))
	binary.LittleEndian.PutUint16(b[50:52], uint16(s.LY))
	binary.LittleEndian.PutUint16(b[52:54], uint16(s.RX))
	binary.LittleEndian.PutUint16(b[54:56], uint16(s.RY))

	// Right touchpad force.
	rPadForce := s.RPadForce
	if rPadForce > 32767 {
		rPadForce = 32767
	}
	binary.LittleEndian.PutUint16(b[58:60], rPadForce)

	return b
}
