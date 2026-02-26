package xboxelite2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

type XboxElite2 struct {
	inputState *InputState
	stateMu    sync.Mutex
	outputFunc func(OutputState)
	descriptor usb.Descriptor
	profile    string
	frame      uint32
}

type xboxElite2CreateOptions struct {
	Profile *string `json:"profile"`
}

func New(o *device.CreateOptions) (*XboxElite2, error) {
	d := &XboxElite2{
		descriptor: cloneDescriptor(defaultDescriptor),
		profile:    ProfileElite2,
	}
	d.applyProfileDefaults(ProfileElite2)
	profileExplicit := false

	if o != nil && o.DeviceSpecific != nil {
		var args xboxElite2CreateOptions
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
	case "", ProfileElite2, "xboxelite2", "xbox-elite2", "series2":
		return ProfileElite2, nil
	case ProfileElite2GIP, "xboxelite2-gip", "xbox-elite2-gip", "gip", "0x0b00", "0b00":
		return ProfileElite2GIP, nil
	case ProfileXboxOneElite, "xbox-elite", "xboxoneelite", "xbox-one-elite-1", "elite1":
		return ProfileXboxOneElite, nil
	case ProfileXboxSeries, "xboxseries", "series":
		return ProfileXboxSeries, nil
	case ProfileSteamDeck, "steam-deck", "deck", "valve-steamdeck", "1205", "0x1205":
		return ProfileSteamDeck, nil
	case ProfileSteamGeneric, "steam-generic", "steam-controller", "deck-uhid", "12f0", "0x12f0":
		return ProfileSteamGeneric, nil
	default:
		return "", fmt.Errorf(
			"unsupported xboxelite2 profile %q (supported: %q, %q, %q, %q, %q, %q)",
			profile,
			ProfileElite2,
			ProfileElite2GIP,
			ProfileXboxOneElite,
			ProfileXboxSeries,
			ProfileSteamDeck,
			ProfileSteamGeneric,
		)
	}
}

func profileForIDs(vid, pid uint16) (string, bool) {
	switch {
	case vid == DefaultVID && pid == DefaultPIDElite2:
		return ProfileElite2, true
	case vid == DefaultVID && pid == DefaultPIDElite2GIP:
		return ProfileElite2GIP, true
	case vid == DefaultVID && pid == DefaultPIDXboxOneElite:
		return ProfileXboxOneElite, true
	case vid == DefaultVID && pid == DefaultPIDXboxSeries:
		return ProfileXboxSeries, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamDeck:
		return ProfileSteamDeck, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamGeneric:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamMsiClaw:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamLenovoLegionGo2:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamZotacZone:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamAsusRogAlly:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamLenovoLegionGo:
		return ProfileSteamGeneric, true
	case vid == DefaultVIDSteam && pid == DefaultPIDSteamLenovoLegionGoS:
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

func (x *XboxElite2) applyProfileDefaults(profile string) {
	x.profile = profile
	x.descriptor = cloneDescriptor(defaultDescriptor)

	switch profile {
	case ProfileElite2GIP:
		x.descriptor.Device.IDVendor = DefaultVID
		x.descriptor.Device.IDProduct = DefaultPIDElite2GIP
		x.descriptor.Strings[2] = "Microsoft X-Box One Elite 2 pad"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), xboxElite2HIDDescriptor...)
	case ProfileXboxOneElite:
		x.descriptor.Device.IDVendor = DefaultVID
		x.descriptor.Device.IDProduct = DefaultPIDXboxOneElite
		x.descriptor.Strings[2] = "Xbox One Elite Controller"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), xboxOneEliteHIDDescriptor...)
	case ProfileXboxSeries:
		x.descriptor.Device.IDVendor = DefaultVID
		x.descriptor.Device.IDProduct = DefaultPIDXboxSeries
		x.descriptor.Strings[2] = "Xbox Series X Controller"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), xboxSeriesHIDDescriptor...)
	case ProfileSteamDeck:
		x.descriptor.Device.IDVendor = DefaultVIDSteam
		x.descriptor.Device.IDProduct = DefaultPIDSteamDeck
		x.descriptor.Strings[1] = "Valve Software"
		x.descriptor.Strings[2] = "Steam Deck Controller"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), steamDeckControllerHIDDescriptor...)
		x.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize = 64
		x.descriptor.Interfaces[0].Endpoints[1].WMaxPacketSize = 64
	case ProfileSteamGeneric:
		x.descriptor.Device.IDVendor = DefaultVIDSteam
		x.descriptor.Device.IDProduct = DefaultPIDSteamGeneric
		x.descriptor.Strings[1] = "Valve Software"
		x.descriptor.Strings[2] = "Generic Steam Controller"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), steamDeckControllerHIDDescriptor...)
		x.descriptor.Interfaces[0].Endpoints[0].WMaxPacketSize = 64
		x.descriptor.Interfaces[0].Endpoints[1].WMaxPacketSize = 64
	default:
		x.descriptor.Device.IDVendor = DefaultVID
		x.descriptor.Device.IDProduct = DefaultPIDElite2
		x.descriptor.Strings[2] = "Xbox Elite Wireless Controller Series 2"
		x.descriptor.Interfaces[0].HID.ReportRaw = append([]byte(nil), xboxElite2HIDDescriptor...)
	}
}

func (x *XboxElite2) isSteamProfile() bool {
	return x.profile == ProfileSteamDeck || x.profile == ProfileSteamGeneric
}

func (x *XboxElite2) buttonLayout() (includePaddles bool, shareBit int) {
	switch x.profile {
	case ProfileXboxOneElite:
		return true, -1
	case ProfileXboxSeries:
		return false, 11
	case ProfileElite2, ProfileElite2GIP:
		fallthrough
	default:
		return true, 16
	}
}

func (x *XboxElite2) SetOutputCallback(f func(OutputState)) {
	x.outputFunc = f
}

func (x *XboxElite2) UpdateInputState(state *InputState) {
	x.stateMu.Lock()
	defer x.stateMu.Unlock()
	x.inputState = state
}

func (x *XboxElite2) HandleTransfer(ep uint32, dir uint32, out []byte) []byte {
	if dir == usbip.DirIn {
		switch ep {
		case 1: // 0x81 - main input reports
			x.stateMu.Lock()
			var st InputState
			if x.inputState != nil {
				st = *x.inputState
			}
			x.stateMu.Unlock()
			return x.buildUSBInputReport(st)
		default:
			return nil
		}
	}

	if dir == usbip.DirOut && ep == 1 {
		if x.isSteamProfile() {
			x.parseSteamDeckOutputReport(out)
		} else if len(out) >= OutputReportSize && out[0] == ReportIDOutput {
			x.parseXboxOutputReport(out)
		}
	}

	return nil
}

func (x *XboxElite2) parseXboxOutputReport(out []byte) {
	feedback := OutputState{
		RumbleLeft:         out[1],
		RumbleRight:        out[2],
		RumbleTriggerLeft:  out[3],
		RumbleTriggerRight: out[4],
	}
	if x.outputFunc != nil {
		x.outputFunc(feedback)
	}
}

func (x *XboxElite2) parseSteamDeckOutputReport(out []byte) {
	start := 0
	// Some stacks prefix an extra report-id byte (0x00) before command payload.
	if len(out) >= 10 && out[0] == 0x00 {
		start = 1
	}
	if len(out) < start+9 {
		return
	}
	// Steam Deck rumble command (TriggerRumbleCommand).
	if out[start] != SteamDeckRumbleCommandType {
		return
	}

	left := binary.LittleEndian.Uint16(out[start+5 : start+7])
	right := binary.LittleEndian.Uint16(out[start+7 : start+9])
	feedback := OutputState{
		RumbleLeft:         uint8(left >> 8),
		RumbleRight:        uint8(right >> 8),
		RumbleTriggerLeft:  0,
		RumbleTriggerRight: 0,
	}
	if x.outputFunc != nil {
		x.outputFunc(feedback)
	}
}

func (x *XboxElite2) HandleControl(bmRequestType, bRequest uint8, wValue, _ uint16, wLength uint16, data []byte) ([]byte, bool) {
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
		if reportType == reportTypeInput && (x.isSteamProfile() || reportID == ReportIDInput) {
			x.stateMu.Lock()
			var st InputState
			if x.inputState != nil {
				st = *x.inputState
			}
			x.stateMu.Unlock()
			report := x.buildUSBInputReport(st)
			if wLength > 0 && int(wLength) < len(report) {
				return report[:wLength], true
			}
			return report, true
		}

		if reportType == reportTypeFeature && !x.isSteamProfile() && reportID == 0x03 {
			buf := make([]byte, 5)
			buf[0] = 0x03
			// Firmware version placeholder.
			buf[1] = 0x05
			buf[2] = 0x13
			buf[3] = 0x00
			buf[4] = 0x01
			return buf, true
		}

		if reportType == reportTypeFeature {
			size := int(wLength)
			if size <= 0 {
				if x.isSteamProfile() {
					size = InputReportSizeSteamDeck
				} else {
					size = 16
				}
			}
			buf := make([]byte, size)
			if !x.isSteamProfile() {
				buf[0] = reportID
			}
			return buf, true
		}
	}

	if bmRequestType == 0x21 && bRequest == hidSetReport {
		if x.isSteamProfile() {
			if reportType == reportTypeOutput || reportType == reportTypeFeature {
				x.parseSteamDeckOutputReport(data)
				return nil, true
			}
		} else if reportType == reportTypeOutput && reportID == ReportIDOutput && len(data) >= OutputReportSize {
			x.parseXboxOutputReport(data)
			return nil, true
		} else if reportType == reportTypeFeature {
			return nil, true
		}
	}

	slog.Warn("XboxElite2: unsupported control request",
		"bmRequestType", bmRequestType,
		"bRequest", bRequest,
		"reportType", reportType,
		"reportID", reportID)

	return nil, false
}

func (x *XboxElite2) GetDescriptor() *usb.Descriptor {
	return &x.descriptor
}

func (x *XboxElite2) GetDeviceSpecificArgs() map[string]any {
	return map[string]any{
		"profile": x.profile,
	}
}

// buildUSBInputReport builds the host-facing HID report for the currently selected profile.
func (x *XboxElite2) buildUSBInputReport(s InputState) []byte {
	if x.isSteamProfile() {
		return x.buildSteamDeckInputReport(s)
	}
	return x.buildXboxInputReport(s)
}

func (x *XboxElite2) buildXboxInputReport(s InputState) []byte {
	b := make([]byte, InputReportSize)
	b[0] = ReportIDInput

	// Sticks: signed i16 -> unsigned u16 centered at 32768.
	binary.LittleEndian.PutUint16(b[1:3], stickI16ToU16(s.LX))
	binary.LittleEndian.PutUint16(b[3:5], stickI16ToU16(s.LY))
	binary.LittleEndian.PutUint16(b[5:7], stickI16ToU16(s.RX))
	binary.LittleEndian.PutUint16(b[7:9], stickI16ToU16(s.RY))

	// Triggers: u8 -> u10 packed into 16-bit fields.
	binary.LittleEndian.PutUint16(b[9:11], triggerU8ToU10(s.LT))
	binary.LittleEndian.PutUint16(b[11:13], triggerU8ToU10(s.RT))

	// Hat switch.
	b[13] = dpadBitmaskToHat(s.DPad) & DPadMask

	// Button packing layout differs per profile.
	includePaddles, shareBit := x.buttonLayout()
	var bits uint32
	if s.Buttons&ButtonA != 0 {
		bits |= 1 << 0
	}
	if s.Buttons&ButtonB != 0 {
		bits |= 1 << 1
	}
	if s.Buttons&ButtonX != 0 {
		bits |= 1 << 2
	}
	if s.Buttons&ButtonY != 0 {
		bits |= 1 << 3
	}
	if s.Buttons&ButtonLB != 0 {
		bits |= 1 << 4
	}
	if s.Buttons&ButtonRB != 0 {
		bits |= 1 << 5
	}
	if s.Buttons&ButtonBack != 0 {
		bits |= 1 << 6
	}
	if s.Buttons&ButtonStart != 0 {
		bits |= 1 << 7
	}
	if s.Buttons&ButtonGuide != 0 {
		bits |= 1 << 8
	}
	if s.Buttons&ButtonLThumb != 0 {
		bits |= 1 << 9
	}
	if s.Buttons&ButtonRThumb != 0 {
		bits |= 1 << 10
	}
	if includePaddles {
		// Paddles use the HHD/InputPlumber order: P2,P1,P4,P3.
		if s.Buttons&ButtonP2 != 0 {
			bits |= 1 << 11
		}
		if s.Buttons&ButtonP1 != 0 {
			bits |= 1 << 12
		}
		if s.Buttons&ButtonP4 != 0 {
			bits |= 1 << 13
		}
		if s.Buttons&ButtonP3 != 0 {
			bits |= 1 << 14
		}
	}
	if shareBit >= 0 && s.Reserved&ReservedShare != 0 {
		bits |= 1 << shareBit
	}

	b[14] = byte(bits & 0xFF)
	b[15] = byte((bits >> 8) & 0xFF)
	b[16] = byte((bits >> 16) & 0xFF)

	return b
}

func (x *XboxElite2) buildSteamDeckInputReport(s InputState) []byte {
	b := make([]byte, InputReportSizeSteamDeck)
	b[0] = SteamDeckInputMajorVersion
	b[1] = SteamDeckInputMinorVersion
	b[2] = SteamDeckInputReportType
	b[3] = InputReportSizeSteamDeck
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
	setBit(8, 0, s.Buttons&ButtonA != 0)
	setBit(8, 1, s.Buttons&ButtonX != 0)
	setBit(8, 2, s.Buttons&ButtonB != 0)
	setBit(8, 3, s.Buttons&ButtonY != 0)
	setBit(8, 4, s.Buttons&ButtonLB != 0)
	setBit(8, 5, s.Buttons&ButtonRB != 0)
	// Match SteamDeck target behavior: digital trigger bits should only activate
	// close to full pull to avoid noisy analog jitter being interpreted as button chords.
	const steamDigitalTriggerThreshold = 204 // ~80% of 255
	setBit(8, 6, s.LT >= steamDigitalTriggerThreshold)
	setBit(8, 7, s.RT >= steamDigitalTriggerThreshold)

	// Byte 9
	setBit(9, 0, s.Buttons&ButtonP2 != 0) // L5
	setBit(9, 1, s.Buttons&ButtonStart != 0) // menu
	setBit(9, 2, s.Buttons&ButtonGuide != 0) // steam
	setBit(9, 3, s.Buttons&ButtonBack != 0) // options
	setBit(9, 4, s.DPad&DPadDown != 0)
	setBit(9, 5, s.DPad&DPadLeft != 0)
	setBit(9, 6, s.DPad&DPadRight != 0)
	setBit(9, 7, s.DPad&DPadUp != 0)

	// Byte 10 / 11 / 13 / 14
	setBit(10, 1, s.Buttons&ButtonLThumb != 0)
	setBit(10, 3, s.TouchFlags&TouchFlagRightPadTouch != 0) // r_pad_touch
	setBit(10, 5, s.TouchFlags&TouchFlagRightPadPress != 0) // r_pad_press
	setBit(10, 7, s.Buttons&ButtonP1 != 0) // R5
	setBit(11, 5, s.Buttons&ButtonRThumb != 0)
	setBit(13, 5, s.Buttons&ButtonP3 != 0) // R4
	setBit(13, 6, s.Buttons&ButtonP4 != 0) // L4
	setBit(14, 5, s.Reserved&ReservedShare != 0) // quick_access

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
