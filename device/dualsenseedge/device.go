package dualsenseedge

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

type DualSenseEdge struct {
	gate *device.InputGate
	// inputState is stored by value: retaining a caller pointer forced a
	// heap allocation per UpdateInputState call at input rate.
	inputState InputState
	stateMu    sync.Mutex
	outputFunc func(OutputState)
	descriptor usb.Descriptor

	seqCounter         uint8
	usbReportTimestamp uint32

	// Last known LED state — preserved across rumble-only reports.
	lastLedRed, lastLedGreen, lastLedBlue uint8
	lastPlayerLeds                         uint8
}

func New(o *device.CreateOptions) (*DualSenseEdge, error) {
	d := &DualSenseEdge{
		gate: device.NewInputGate(),
		descriptor: defaultDescriptor,
	}
	if o != nil {
		if o.IdVendor != nil {
			d.descriptor.Device.IDVendor = *o.IdVendor
		}
		if o.IdProduct != nil {
			d.descriptor.Device.IDProduct = *o.IdProduct
		}
	}

	d.inputState = InputState{
		AccelX: DefaultAccelXRaw,
		AccelY: DefaultAccelYRaw,
		AccelZ: DefaultAccelZRaw,
	}

	return d, nil
}

func (d *DualSenseEdge) SetOutputCallback(f func(OutputState)) {
	d.outputFunc = f
}

func (d *DualSenseEdge) UpdateInputState(state *InputState) {
	d.stateMu.Lock()
	if state == nil {
		d.inputState = InputState{}
	} else {
		d.inputState = *state
	}
	d.stateMu.Unlock()
	d.gate.Signal()
}

func (d *DualSenseEdge) HandleTransfer(ctx context.Context, ep uint32, dir uint32, out []byte) []byte {
	if dir == usbip.DirIn {
		switch ep {
		case 4:
			if device.GateCancelled == d.gate.Wait(ctx) {
				return nil
			}
			d.stateMu.Lock()
			st := d.inputState
			d.stateMu.Unlock()
			return d.buildUSBInputReport(st)
		default:
			return nil
		}
	}

	if dir == usbip.DirOut && ep == 3 {
		if len(out) >= 48 && out[OutOffsetReportID] == ReportIDOutput {
			d.parseOutputReport(out)
		}
	}

	return nil
}

func (d *DualSenseEdge) parseOutputReport(out []byte) {
	feedback := OutputState{
		RumbleSmall: out[OutOffsetRumbleSmall],
		RumbleLarge: out[OutOffsetRumbleLarge],
		// Start with last known LED state so rumble-only reports
		// don't zero out the color on the physical controller.
		LedRed:     d.lastLedRed,
		LedGreen:   d.lastLedGreen,
		LedBlue:    d.lastLedBlue,
		PlayerLeds: d.lastPlayerLeds,
	}
	// Flag1 (byte 2) controls which LED fields are valid:
	//   bit 0x04 = lightbar RGB control enable
	//   bit 0x10 = player indicator LEDs control enable
	if len(out) > OutOffsetFlag1 {
		flag1 := out[OutOffsetFlag1]
		if flag1&0x04 != 0 && len(out) > OutOffsetLedBlue {
			feedback.LedRed = out[OutOffsetLedRed]
			feedback.LedGreen = out[OutOffsetLedGreen]
			feedback.LedBlue = out[OutOffsetLedBlue]
			d.lastLedRed = feedback.LedRed
			d.lastLedGreen = feedback.LedGreen
			d.lastLedBlue = feedback.LedBlue
		}
		if flag1&0x10 != 0 && len(out) > OutOffsetPlayerLEDs {
			feedback.PlayerLeds = out[OutOffsetPlayerLEDs]
			d.lastPlayerLeds = feedback.PlayerLeds
		}
	}
	if d.outputFunc != nil {
		d.outputFunc(feedback)
	}
}

func (d *DualSenseEdge) HandleControl(bmRequestType, bRequest uint8, wValue, _ uint16, wLength uint16, data []byte) ([]byte, bool) {
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
		if reportType == reportTypeInput && reportID == ReportIDInput {
			d.stateMu.Lock()
			st := d.inputState
			d.stateMu.Unlock()
			report := d.buildUSBInputReport(st)
			if wLength > 0 && int(wLength) < len(report) {
				return report[:wLength], true
			}
			return report, true
		}

		if reportType == reportTypeFeature {
			return d.handleFeatureReport(reportID, wLength), true
		}
	}

	if bmRequestType == 0x21 && bRequest == hidSetReport {
		if reportType == reportTypeOutput && reportID == ReportIDOutput && len(data) >= 48 {
			d.parseOutputReport(data)
			return nil, true
		}
		// Accept SET_REPORT for any feature report without error.
		if reportType == reportTypeFeature {
			return nil, true
		}
	}

	slog.Warn("DualSenseEdge: unsupported control request",
		"bmRequestType", bmRequestType,
		"bRequest", bRequest,
		"reportType", reportType,
		"reportID", reportID)

	return nil, false
}

func (d *DualSenseEdge) handleFeatureReport(reportID uint8, wLength uint16) []byte {
	switch reportID {
	case 0x05: // Calibration data
		return featureReport05[:]
	case 0x09: // Pairing info / MAC address
		return featureReport09[:]
	case 0x20: // Firmware info
		return featureReport20[:]
	default:
		// Return a zeroed report of the expected size for any other feature report.
		size := int(wLength)
		if size <= 0 {
			size = 64
		}
		buf := make([]byte, size)
		buf[0] = reportID
		return buf
	}
}

func (d *DualSenseEdge) GetDescriptor() *usb.Descriptor {
	return &d.descriptor
}

func (d *DualSenseEdge) GetDeviceSpecificArgs() map[string]any {
	return map[string]any{}
}

// buildUSBInputReport builds a 64-byte DualSense Edge USB input report (Report ID 0x01).
//
// Layout (USB mode, ofs=1 from byte 0):
//
//	b[0]       : Report ID (0x01)
//	b[1]       : Left Stick X   (u8, 0x00=left, 0x80=center, 0xFF=right)
//	b[2]       : Left Stick Y   (u8, 0x00=up,   0x80=center, 0xFF=down)
//	b[3]       : Right Stick X
//	b[4]       : Right Stick Y
//	b[5]       : Left Trigger  (L2, Z axis, u8 0x00=released, 0xFF=full)
//	b[6]       : Right Trigger (R2, Rz axis)
//	b[7]       : Sequence Counter
//	b[8]       : DPad[3:0] | Square[4] | Cross[5] | Circle[6] | Triangle[7]
//	b[9]       : L1[0] | R1[1] | L2dig[2] | R2dig[3] | Create[4] | Options[5] | L3[6] | R3[7]
//	b[10]      : PS[0] | Touchpad[1] | Mute[2] | ExtraR2[3] | ExtraL2[4] | ExtraR1[5] | ExtraL1[6] | ExtraL3[7]
//	b[11]      : Vendor padding (remaining 8 bits of 13 vendor bits)
//	b[12-15]   : Reserved
//	b[16-17]   : Gyro X  (i16 LE)
//	b[18-19]   : Gyro Y  (i16 LE)
//	b[20-21]   : Gyro Z  (i16 LE)
//	b[22-23]   : Accel X (i16 LE)
//	b[24-25]   : Accel Y (i16 LE)
//	b[26-27]   : Accel Z (i16 LE)
//	b[28-31]   : Timestamp (u32 LE, units of 333ns)
//	b[32]      : Reserved
//	b[33-36]   : Touch Point 1 (contact_flag, Xlo, Xhi4|Ylo4, Yhi)
//	b[37-40]   : Touch Point 2
//	b[41-52]   : Reserved
//	b[53]      : Battery status
//	b[54-63]   : Reserved
func (d *DualSenseEdge) buildUSBInputReport(s InputState) []byte {
	b := make([]byte, InputReportSize)

	b[0] = ReportIDInput

	// Sticks: convert signed i8 (-128..127) to unsigned u8 (0..255) centered at 128.
	b[1] = uint8(int16(s.LX) + 128)
	b[2] = uint8(int16(s.LY) + 128)
	b[3] = uint8(int16(s.RX) + 128)
	b[4] = uint8(int16(s.RY) + 128)

	// Triggers: b[5]=L2 (Z axis), b[6]=R2 (Rz axis)
	b[5] = s.L2
	b[6] = s.R2

	// Sequence counter
	d.seqCounter++
	b[7] = d.seqCounter

	// DPad hat switch conversion (bitmask → 0-8 hat value)
	usbDPad := uint8(DPadUSBNeutral)
	switch {
	case s.DPad&DPadUp != 0 && s.DPad&DPadRight != 0:
		usbDPad = DPadUSBUpRight
	case s.DPad&DPadUp != 0 && s.DPad&DPadLeft != 0:
		usbDPad = DPadUSBUpLeft
	case s.DPad&DPadDown != 0 && s.DPad&DPadRight != 0:
		usbDPad = DPadUSBDownRight
	case s.DPad&DPadDown != 0 && s.DPad&DPadLeft != 0:
		usbDPad = DPadUSBDownLeft
	case s.DPad&DPadUp != 0:
		usbDPad = DPadUSBUp
	case s.DPad&DPadDown != 0:
		usbDPad = DPadUSBDown
	case s.DPad&DPadLeft != 0:
		usbDPad = DPadUSBLeft
	case s.DPad&DPadRight != 0:
		usbDPad = DPadUSBRight
	}

	// b[8]: DPad[3:0] | face buttons[7:4]
	var faceBits uint8
	if s.Buttons&ButtonSquare != 0 {
		faceBits |= 0x10
	}
	if s.Buttons&ButtonCross != 0 {
		faceBits |= 0x20
	}
	if s.Buttons&ButtonCircle != 0 {
		faceBits |= 0x40
	}
	if s.Buttons&ButtonTriangle != 0 {
		faceBits |= 0x80
	}
	b[8] = (usbDPad & DPadMask) | faceBits

	// b[9]: L1[0] R1[1] L2dig[2] R2dig[3] Create[4] Options[5] L3[6] R3[7]
	var btn1 uint8
	if s.Buttons&ButtonL1 != 0 {
		btn1 |= 0x01
	}
	if s.Buttons&ButtonR1 != 0 {
		btn1 |= 0x02
	}
	if s.Buttons&ButtonL2 != 0 {
		btn1 |= 0x04
	}
	if s.Buttons&ButtonR2 != 0 {
		btn1 |= 0x08
	}
	if s.Buttons&ButtonCreate != 0 {
		btn1 |= 0x10
	}
	if s.Buttons&ButtonOptions != 0 {
		btn1 |= 0x20
	}
	if s.Buttons&ButtonL3 != 0 {
		btn1 |= 0x40
	}
	if s.Buttons&ButtonR3 != 0 {
		btn1 |= 0x80
	}
	b[9] = btn1

	// b[10]: PS[0] Touchpad[1] Mute[2] ExtraR2[3] ExtraL2[4] ExtraR1[5] ExtraL1[6] ExtraL3[7]
	var btn2 uint8
	if s.Buttons&ButtonPS != 0 {
		btn2 |= 0x01
	}
	if s.Buttons&ButtonTouchpad != 0 {
		btn2 |= 0x02
	}
	if s.Buttons&ButtonMute != 0 {
		btn2 |= 0x04
	}
	if s.Buttons&ButtonExtraR2 != 0 {
		btn2 |= 0x08
	}
	if s.Buttons&ButtonExtraL2 != 0 {
		btn2 |= 0x10
	}
	if s.Buttons&ButtonExtraR1 != 0 {
		btn2 |= 0x20
	}
	if s.Buttons&ButtonExtraL1 != 0 {
		btn2 |= 0x40
	}
	if s.Buttons&ButtonExtraL3 != 0 {
		btn2 |= 0x80
	}
	b[10] = btn2

	// b[11]: remaining vendor bits (padding)
	b[11] = 0x00

	// b[12-15]: reserved
	// b[16-21]: Gyro XYZ (i16 LE)
	binary.LittleEndian.PutUint16(b[16:18], uint16(s.GyroX))
	binary.LittleEndian.PutUint16(b[18:20], uint16(s.GyroY))
	binary.LittleEndian.PutUint16(b[20:22], uint16(s.GyroZ))

	// b[22-27]: Accel XYZ (i16 LE)
	binary.LittleEndian.PutUint16(b[22:24], uint16(s.AccelX))
	binary.LittleEndian.PutUint16(b[24:26], uint16(s.AccelY))
	binary.LittleEndian.PutUint16(b[26:28], uint16(s.AccelZ))

	// b[28-31]: Timestamp (u32 LE)
	ts := atomic.AddUint32(&d.usbReportTimestamp, 1)
	binary.LittleEndian.PutUint32(b[28:32], ts)

	// b[32]: reserved

	// b[33-36]: Touch point 1
	if s.Touch1Active {
		b[33] = 0x00 // bit 7 = 0 means touching
	} else {
		b[33] = TouchInactiveMask // bit 7 = 1 means not touching
	}
	encodeTouchCoords(b[34:37], s.Touch1X, s.Touch1Y)

	// b[37-40]: Touch point 2
	if s.Touch2Active {
		b[37] = 0x00
	} else {
		b[37] = TouchInactiveMask
	}
	encodeTouchCoords(b[38:41], s.Touch2X, s.Touch2Y)

	// b[49]: Active profile (DSE) — upper nibble = profile index (1-4),
	//        lower 2 bits = mode (0x00 = normal). Must be non-zero for
	//        tester sites to detect normal mode (JS truthiness check).
	b[49] = 0x10 // Profile 1, normal mode

	// b[53]: Battery (fully charged)
	b[53] = BatteryFullyCharged

	return b
}

// Stock feature report responses.
// Calibration data must match the actual sensor output in the USB report:
//   Gyro: BMI323 ±2000 dps = 16.384 LSB/dps (passthrough) → at 500 dps ref = 8192 counts
//   Accel: BMI323 4096 LSB/g × ScaleAccel(×2) = 8192 counts/g
var featureReport05 = [41]byte{
	0x05, // Report ID
	0x00, 0x00, // Gyro Pitch Bias
	0x00, 0x00, // Gyro Yaw Bias
	0x00, 0x00, // Gyro Roll Bias
	0x00, 0x20, // Gyro Pitch Plus  (8192)
	0x00, 0xE0, // Gyro Pitch Minus (-8192)
	0x00, 0x20, // Gyro Yaw Plus    (8192)
	0x00, 0xE0, // Gyro Yaw Minus   (-8192)
	0x00, 0x20, // Gyro Roll Plus   (8192)
	0x00, 0xE0, // Gyro Roll Minus  (-8192)
	0xF4, 0x01, // Gyro Speed Plus  (500 dps)
	0xF4, 0x01, // Gyro Speed Minus (500 dps)
	0x00, 0x20, // Accel X Plus     (8192 = +1g)
	0x00, 0xE0, // Accel X Minus    (-8192 = -1g)
	0x00, 0x20, // Accel Y Plus     (8192)
	0x00, 0xE0, // Accel Y Minus    (-8192)
	0x00, 0x20, // Accel Z Plus     (8192)
	0x00, 0xE0, // Accel Z Minus    (-8192)
	0x0B, 0x00,
	0x00, 0x00,
	0x00, 0x00,
}

var featureReport09 = [20]byte{
	0x09, // Report ID
	0x74, 0xE7, 0xD6, 0x3A, 0x53, 0x35, // MAC address
	0x08, 0x25,
	0x00, 0x1E, 0x00,
	0xEE, 0x74, 0xD0, 0xBC,
	0x00, 0x00, 0x00, 0x00,
}

var featureReport20 = [64]byte{
	0x20, // Report ID
	0x4A, 0x75, 0x6E, 0x20, 0x31, 0x39, 0x20, // "Jun 19 "
	0x32, 0x30, 0x32, 0x33, // "2023"
	0x31, 0x34, 0x3A, 0x34, 0x37, 0x3A, 0x33, 0x34, // "14:47:34"
	0x03, 0x00, 0x44, 0x00, 0x08, 0x02, 0x00, 0x01,
	0x36, 0x00, 0x00, 0x01, 0xC1, 0xC8, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x54, 0x01, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00,
	0x0B, 0x00, 0x01, 0x00, 0x06, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

// HID report descriptor for DualSense Edge USB mode.
// Source: HHD DS5_EDGE_DESCRIPTOR_USB.
var ds5EdgeHIDDescriptor = []byte{
	0x05, 0x01, // Usage Page (Generic Desktop)
	0x09, 0x05, // Usage (Game Pad)
	0xA1, 0x01, // Collection (Application)

	0x85, 0x01, // Report ID (1) — Input
	0x09, 0x30, // Usage (X)  — Left Stick X
	0x09, 0x31, // Usage (Y)  — Left Stick Y
	0x09, 0x32, // Usage (Z)  — Right Stick X
	0x09, 0x35, // Usage (Rz) — Right Stick Y
	0x09, 0x33, // Usage (Rx) — Right Trigger
	0x09, 0x34, // Usage (Ry) — Left Trigger
	0x15, 0x00, // Logical Minimum (0)
	0x26, 0xFF, 0x00, // Logical Maximum (255)
	0x75, 0x08, // Report Size (8)
	0x95, 0x06, // Report Count (6)
	0x81, 0x02, // Input (Data,Var,Abs) — 6 bytes: axes

	0x06, 0x00, 0xFF, // Usage Page (Vendor Defined)
	0x09, 0x20, // Usage (0x20) — Sequence number
	0x95, 0x01, // Report Count (1)
	0x81, 0x02, // Input — 1 byte

	0x05, 0x01, // Usage Page (Generic Desktop)
	0x09, 0x39, // Usage (Hat switch) — DPad
	0x15, 0x00, // Logical Minimum (0)
	0x25, 0x07, // Logical Maximum (7)
	0x35, 0x00, // Physical Minimum (0)
	0x46, 0x3B, 0x01, // Physical Maximum (315)
	0x65, 0x14, // Unit (Eng Rotation: deg)
	0x75, 0x04, // Report Size (4)
	0x95, 0x01, // Report Count (1)
	0x81, 0x42, // Input (Data,Var,Abs,Null) — 4 bits

	0x65, 0x00, // Unit (None)
	0x05, 0x09, // Usage Page (Button)
	0x19, 0x01, // Usage Minimum (1)
	0x29, 0x0F, // Usage Maximum (15)
	0x15, 0x00, // Logical Minimum (0)
	0x25, 0x01, // Logical Maximum (1)
	0x75, 0x01, // Report Size (1)
	0x95, 0x0F, // Report Count (15)
	0x81, 0x02, // Input — 15 bits: buttons

	0x06, 0x00, 0xFF, // Usage Page (Vendor Defined)
	0x09, 0x21, // Usage (0x21) — vendor buttons / padding
	0x95, 0x0D, // Report Count (13)
	0x81, 0x02, // Input — 13 bits

	0x06, 0x00, 0xFF, // Usage Page (Vendor Defined)
	0x09, 0x22, // Usage (0x22) — remaining payload
	0x15, 0x00, // Logical Minimum (0)
	0x26, 0xFF, 0x00, // Logical Maximum (255)
	0x75, 0x08, // Report Size (8)
	0x95, 0x34, // Report Count (52)
	0x81, 0x02, // Input — 52 bytes

	// Output Report (ID 0x02, 63 bytes)
	0x85, 0x02, // Report ID (2)
	0x09, 0x23, // Usage (0x23)
	0x95, 0x3F, // Report Count (63)
	0x91, 0x02, // Output — 63 bytes

	// Feature Reports
	0x85, 0x05, 0x09, 0x33, 0x95, 0x28, 0xB1, 0x02, // ID=5, 40 bytes (calibration)
	0x85, 0x08, 0x09, 0x34, 0x95, 0x2F, 0xB1, 0x02, // ID=8, 47 bytes
	0x85, 0x09, 0x09, 0x24, 0x95, 0x13, 0xB1, 0x02, // ID=9, 19 bytes (pairing)
	0x85, 0x0A, 0x09, 0x25, 0x95, 0x1A, 0xB1, 0x02, // ID=10, 26 bytes
	0x85, 0x20, 0x09, 0x26, 0x95, 0x3F, 0xB1, 0x02, // ID=32, 63 bytes (firmware)
	0x85, 0x21, 0x09, 0x27, 0x95, 0x04, 0xB1, 0x02, // ID=33, 4 bytes
	0x85, 0x22, 0x09, 0x40, 0x95, 0x3F, 0xB1, 0x02, // ID=34, 63 bytes
	0x85, 0x80, 0x09, 0x28, 0x95, 0x3F, 0xB1, 0x02, // ID=128
	0x85, 0x81, 0x09, 0x29, 0x95, 0x3F, 0xB1, 0x02, // ID=129
	0x85, 0x82, 0x09, 0x2A, 0x95, 0x09, 0xB1, 0x02, // ID=130, 9 bytes
	0x85, 0x83, 0x09, 0x2B, 0x95, 0x3F, 0xB1, 0x02, // ID=131
	0x85, 0x84, 0x09, 0x2C, 0x95, 0x3F, 0xB1, 0x02, // ID=132
	0x85, 0x85, 0x09, 0x2D, 0x95, 0x02, 0xB1, 0x02, // ID=133, 2 bytes
	0x85, 0xA0, 0x09, 0x2E, 0x95, 0x01, 0xB1, 0x02, // ID=160, 1 byte
	0x85, 0xE0, 0x09, 0x2F, 0x95, 0x3F, 0xB1, 0x02, // ID=224
	0x85, 0xF0, 0x09, 0x30, 0x95, 0x3F, 0xB1, 0x02, // ID=240
	0x85, 0xF1, 0x09, 0x31, 0x95, 0x3F, 0xB1, 0x02, // ID=241
	0x85, 0xF2, 0x09, 0x32, 0x95, 0x34, 0xB1, 0x02, // ID=242, 52 bytes
	0x85, 0xF4, 0x09, 0x35, 0x95, 0x3F, 0xB1, 0x02, // ID=244
	0x85, 0xF5, 0x09, 0x36, 0x95, 0x03, 0xB1, 0x02, // ID=245, 3 bytes

	// Edge-specific feature reports (0x60-0x7B)
	0x85, 0x60, 0x09, 0x41, 0x95, 0x3F, 0xB1, 0x02,
	0x85, 0x61, 0x09, 0x42, 0xB1, 0x02,
	0x85, 0x62, 0x09, 0x43, 0xB1, 0x02,
	0x85, 0x63, 0x09, 0x44, 0xB1, 0x02,
	0x85, 0x64, 0x09, 0x45, 0xB1, 0x02,
	0x85, 0x65, 0x09, 0x46, 0xB1, 0x02,
	0x85, 0x68, 0x09, 0x47, 0xB1, 0x02,
	0x85, 0x70, 0x09, 0x48, 0xB1, 0x02,
	0x85, 0x71, 0x09, 0x49, 0xB1, 0x02,
	0x85, 0x72, 0x09, 0x4A, 0xB1, 0x02,
	0x85, 0x73, 0x09, 0x4B, 0xB1, 0x02,
	0x85, 0x74, 0x09, 0x4C, 0xB1, 0x02,
	0x85, 0x75, 0x09, 0x4D, 0xB1, 0x02,
	0x85, 0x76, 0x09, 0x4E, 0xB1, 0x02,
	0x85, 0x77, 0x09, 0x4F, 0xB1, 0x02,
	0x85, 0x78, 0x09, 0x50, 0xB1, 0x02,
	0x85, 0x79, 0x09, 0x51, 0xB1, 0x02,
	0x85, 0x7A, 0x09, 0x52, 0xB1, 0x02,
	0x85, 0x7B, 0x09, 0x53, 0xB1, 0x02,

	0xC0, // End Collection
}

var defaultDescriptor = usb.Descriptor{
	Device: usb.DeviceDescriptor{
		BcdUSB:             0x0200,
		BDeviceClass:       0x00,
		BDeviceSubClass:    0x00,
		BDeviceProtocol:    0x00,
		BMaxPacketSize0:    0x40,
		IDVendor:           DefaultVID,
		IDProduct:          DefaultPID,
		BcdDevice:          0x0100,
		IManufacturer:      0x01,
		IProduct:           0x02,
		ISerialNumber:      0x00,
		BNumConfigurations: 0x01,
		Speed:              2, // Full speed
	},
	Interfaces: []usb.InterfaceConfig{
		{
			Descriptor: usb.InterfaceDescriptor{
				BInterfaceNumber:   0x00,
				BAlternateSetting:  0x00,
				BNumEndpoints:      0x02,
				BInterfaceClass:    0x03, // HID
				BInterfaceSubClass: 0x00,
				BInterfaceProtocol: 0x00,
				IInterface:         0x00,
			},
			HID: &usb.HIDFunction{
				Descriptor: usb.HIDDescriptor{
					BcdHID:       0x0111,
					BCountryCode: 0x00,
					Descriptors: []usb.HIDSubDescriptor{
						{Type: usb.ReportDescType},
					},
				},
				ReportRaw: ds5EdgeHIDDescriptor,
			},
			Endpoints: []usb.EndpointDescriptor{
				{
					BEndpointAddress: EndpointIn,
					BMAttributes:     0x03, // Interrupt
					WMaxPacketSize:   64,
					BInterval:        4, // 4ms = 250Hz
				},
				{
					BEndpointAddress: EndpointOut,
					BMAttributes:     0x03, // Interrupt
					WMaxPacketSize:   64,
					BInterval:        4,
				},
			},
		},
	},
	Strings: map[uint8]string{
		0: "Љ", // LangID: en-US (0x0409)
		1: "Sony Interactive Entertainment",
		2: "DualSense Edge Wireless Controller",
	},
}
