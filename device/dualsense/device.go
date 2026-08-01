package dualsense

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usb/hid"
	"github.com/Alia5/VIIPER/usbip"
)

type DualSense struct {
	gate *device.InputGate
	// inputState is stored by value: retaining a caller pointer forced a
	// heap allocation per UpdateInputState call at input rate.
	inputState InputState
	stateMu    sync.Mutex
	timeMu     sync.Mutex
	outputFunc func(OutputState)
	descriptor usb.Descriptor
	now        func() time.Time

	usbSensorTimestamp uint32
	usbPacketCounter   uint32
	lastUSBReportAt    time.Time
}

const usbSensorTimestampStep = 12000

func New(o *device.CreateOptions) (*DualSense, error) {
	d := &DualSense{
		gate: device.NewInputGate(),
		descriptor: defaultDescriptor,
		now:        time.Now,
	}
	if o != nil {
		if o.IdVendor != nil {
			d.descriptor.Device.IDVendor = *o.IdVendor
		}
		if o.IdProduct != nil {
			d.descriptor.Device.IDProduct = *o.IdProduct
		}
	}
	return d, nil
}

func (d *DualSense) SetOutputCallback(f func(OutputState)) {
	d.outputFunc = f
}

func (d *DualSense) UpdateInputState(state *InputState) {
	d.stateMu.Lock()
	if state == nil {
		d.inputState = InputState{}
	} else {
		d.inputState = *state
	}
	d.stateMu.Unlock()
	d.gate.Signal()
}

func (d *DualSense) HandleTransfer(ctx context.Context, ep uint32, dir uint32, out []byte) []byte {
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
		d.handleOutputReport(out)
	}

	return nil
}

func (d *DualSense) HandleControl(bmRequestType, bRequest uint8, wValue, _ uint16, wLength uint16, data []byte) ([]byte, bool) {
	const (
		hidGetReport = 0x01
		hidSetReport = 0x09

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
			resp := d.featureResponse(reportID)
			if resp == nil {
				return nil, false
			}
			if wLength > 0 && int(wLength) < len(resp) {
				return resp[:wLength], true
			}
			return resp, true
		}
	}

	if bmRequestType == 0x21 && bRequest == hidSetReport {
		if reportType == reportTypeOutput && reportID == ReportIDOutput {
			d.handleOutputReport(data)
			return nil, true
		}
	}

	slog.Warn("Unsupported control request",
		"bmRequestType", bmRequestType,
		"bRequest", bRequest)

	return nil, false
}

func (d *DualSense) GetDescriptor() *usb.Descriptor {
	return &d.descriptor
}

func (d *DualSense) GetDeviceSpecificArgs() map[string]any {
	return map[string]any{}
}

func (d *DualSense) nextSensorTimestamp() uint32 {
	d.timeMu.Lock()
	defer d.timeMu.Unlock()

	now := d.now()
	step := uint32(usbSensorTimestampStep)
	if !d.lastUSBReportAt.IsZero() {
		elapsed := now.Sub(d.lastUSBReportAt)
		if elapsed < 0 {
			elapsed = 0
		}
		computed := uint32((uint64(elapsed.Nanoseconds()) * 3) / 1000)
		if computed > 0 {
			step = computed
		}
	}
	d.lastUSBReportAt = now

	return atomic.AddUint32(&d.usbSensorTimestamp, step)
}

func (d *DualSense) buildUSBInputReport(s InputState) []byte {
	b := make([]byte, InputReportSize)
	b[0] = ReportIDInput
	b[1] = uint8(int16(s.LX) + 128)
	b[2] = uint8(int16(s.LY) + 128)
	b[3] = uint8(int16(s.RX) + 128)
	b[4] = uint8(int16(s.RY) + 128)
	b[5] = s.L2
	b[6] = s.R2
	b[7] = uint8(atomic.AddUint32(&d.usbPacketCounter, 1))

	usbDPad := uint8(DPadUSBNeutral)
	if s.DPad&DPadUp != 0 && s.DPad&DPadRight != 0 {
		usbDPad = DPadUSBUpRight
	} else if s.DPad&DPadUp != 0 && s.DPad&DPadLeft != 0 {
		usbDPad = DPadUSBUpLeft
	} else if s.DPad&DPadDown != 0 && s.DPad&DPadRight != 0 {
		usbDPad = DPadUSBDownRight
	} else if s.DPad&DPadDown != 0 && s.DPad&DPadLeft != 0 {
		usbDPad = DPadUSBDownLeft
	} else if s.DPad&DPadUp != 0 {
		usbDPad = DPadUSBUp
	} else if s.DPad&DPadDown != 0 {
		usbDPad = DPadUSBDown
	} else if s.DPad&DPadLeft != 0 {
		usbDPad = DPadUSBLeft
	} else if s.DPad&DPadRight != 0 {
		usbDPad = DPadUSBRight
	}

	b[8] = (usbDPad & DPadMask) | (uint8(s.Buttons) & 0xF0)
	buttons1 := uint8(s.Buttons >> 8)
	if s.L2 > 0 {
		buttons1 |= uint8(ButtonL2 >> 8)
	}
	if s.R2 > 0 {
		buttons1 |= uint8(ButtonR2 >> 8)
	}
	b[9] = buttons1
	b[10] = uint8(s.Buttons >> 16)
	b[11] = uint8(s.Buttons >> 24)

	binary.LittleEndian.PutUint16(b[16:18], uint16(s.GyroX))
	binary.LittleEndian.PutUint16(b[18:20], uint16(s.GyroY))
	binary.LittleEndian.PutUint16(b[20:22], uint16(s.GyroZ))
	binary.LittleEndian.PutUint16(b[22:24], uint16(s.AccelX))
	binary.LittleEndian.PutUint16(b[24:26], uint16(s.AccelY))
	binary.LittleEndian.PutUint16(b[26:28], uint16(s.AccelZ))
	binary.LittleEndian.PutUint32(b[28:32], d.nextSensorTimestamp())

	b[53] = StatusDefault
	encodeTouchPoint(b[33:37], s.TouchX, s.TouchY, s.TouchActive)
	encodeTouchPoint(b[37:41], 0, 0, false)

	return b
}

func encodeTouchPoint(b []byte, x, y uint16, active bool) {
	if x > TouchpadMaxX {
		x = TouchpadMaxX
	}
	if y > TouchpadMaxY {
		y = TouchpadMaxY
	}
	b[0] = 0
	if !active {
		b[0] = TouchInactiveMask
	}
	b[1] = uint8(x & 0xFF)
	b[2] = uint8((x>>8)&0x0F) | uint8((y&0x0F)<<4)
	b[3] = uint8(y >> 4)
}

func (d *DualSense) handleOutputReport(data []byte) {
	if len(data) == 0 {
		return
	}
	if len(data) > 1 && data[0] == 0x00 {
		data = data[1:]
	}
	if len(data) < OutputReportSize || data[OutOffsetReportID] != ReportIDOutput {
		return
	}
	if d.outputFunc == nil {
		return
	}
	d.outputFunc(OutputState{
		RumbleSmall: data[OutOffsetRumbleSmall],
		RumbleLarge: data[OutOffsetRumbleLarge],
		LedRed:      data[OutOffsetLedRed],
		LedGreen:    data[OutOffsetLedGreen],
		LedBlue:     data[OutOffsetLedBlue],
	})
}

func (d *DualSense) featureResponse(reportID uint8) []byte {
	switch reportID {
	case FeatureReportCalibration:
		resp := make([]byte, 41)
		resp[0] = FeatureReportCalibration
		return resp
	case FeatureReportPairingInfo:
		resp := make([]byte, 20)
		resp[0] = FeatureReportPairingInfo
		copy(resp[1:7], []byte{0x44, 0x53, 0x35, 0x00, 0x00, 0x01})
		return resp
	case FeatureReportFirmware:
		resp := make([]byte, 64)
		resp[0] = FeatureReportFirmware
		binary.LittleEndian.PutUint32(resp[24:28], 0x00010000)
		binary.LittleEndian.PutUint32(resp[28:32], 0x00010000)
		binary.LittleEndian.PutUint16(resp[44:46], 0x0221)
		return resp
	default:
		return nil
	}
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
		Speed:              2,
	},
	Interfaces: []usb.InterfaceConfig{{
		Descriptor: usb.InterfaceDescriptor{
			BInterfaceNumber:   0x00,
			BAlternateSetting:  0x00,
			BNumEndpoints:      0x02,
			BInterfaceClass:    0x03,
			BInterfaceSubClass: 0x00,
			BInterfaceProtocol: 0x00,
			IInterface:         0x00,
		},
		HID: &usb.HIDFunction{
			Descriptor: usb.HIDDescriptor{
				BcdHID:       0x0111,
				BCountryCode: 0x00,
				Descriptors:  []usb.HIDSubDescriptor{{Type: usb.ReportDescType}},
			},
			Report: hid.Report{Items: []hid.Item{
				hid.UsagePage{Page: hid.UsagePageGenericDesktop},
				hid.Usage{Usage: hid.UsageGamePad},
				hid.Collection{Kind: hid.CollectionApplication, Items: []hid.Item{
					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x08, Data: hid.Data{ReportIDInput}},
					hid.UsagePage{Page: hid.UsagePageGenericDesktop},
					hid.Usage{Usage: hid.UsageX},
					hid.Usage{Usage: hid.UsageY},
					hid.Usage{Usage: hid.UsageZ},
					hid.Usage{Usage: hid.UsageRz},
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 255},
					hid.ReportSize{Bits: 8},
					hid.ReportCount{Count: 4},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.UsagePage{Page: hid.UsagePageGenericDesktop},
					// L2 / R2 analog triggers. Must be Rx (0x33) / Ry (0x34) — the
					// usages a real DualSense uses — NOT Z (0x32) / Rz (0x35), which
					// duplicate the right-stick usages above. Duplicate usages in one
					// report confuse Windows' RawGameController axis enumeration, which
					// makes Xbox Game Bar (and other WGI RawGameController consumers)
					// mis-map the triggers — observed as "LT navigates the tab but RT
					// doesn't" + double-input. Steam is unaffected (reads raw HID).
					hid.Usage{Usage: 0x33}, // Rx = L2
					hid.Usage{Usage: 0x34}, // Ry = R2
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 255},
					hid.ReportSize{Bits: 8},
					hid.ReportCount{Count: 2},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.UsagePage{Page: 0xFF00},
					hid.Usage{Usage: 0x20},
					hid.ReportSize{Bits: 8},
					hid.ReportCount{Count: 1},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.UsagePage{Page: hid.UsagePageGenericDesktop},
					hid.Usage{Usage: 0x39},
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 7},
					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x3, Data: hid.Data{0x00}},
					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x4, Data: hid.Data{0x3B, 0x01}},
					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x6, Data: hid.Data{0x14}},
					hid.ReportSize{Bits: 4},
					hid.ReportCount{Count: 1},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs | hid.MainNullState},
					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x6, Data: hid.Data{0x00}},

					hid.UsagePage{Page: hid.UsagePageButton},
					hid.UsageMinimum{Min: 0x01},
					hid.UsageMaximum{Max: 0x0E},
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 1},
					hid.ReportCount{Count: 14},
					hid.ReportSize{Bits: 1},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.UsagePage{Page: 0xFF00},
					hid.Usage{Usage: 0x20},
					hid.ReportSize{Bits: 6},
					hid.ReportCount{Count: 1},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.UsagePage{Page: 0xFF00},
					hid.Usage{Usage: 0x20},
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 255},
					hid.ReportSize{Bits: 8},
					hid.ReportCount{Count: 53},
					hid.Input{Flags: hid.MainData | hid.MainVar | hid.MainAbs},

					hid.AnyItem{Type: hid.ItemTypeGlobal, Tag: 0x08, Data: hid.Data{ReportIDOutput}},
					hid.UsagePage{Page: 0xFF00},
					hid.Usage{Usage: 0x21},
					hid.LogicalMinimum{Min: 0},
					hid.LogicalMaximum{Max: 255},
					hid.ReportSize{Bits: 8},
					hid.ReportCount{Count: 63},
					hid.Output{Flags: hid.MainData | hid.MainVar | hid.MainAbs},
				}},
			}},
		},
		Endpoints: []usb.EndpointDescriptor{
			{BEndpointAddress: EndpointIn, BMAttributes: 0x03, WMaxPacketSize: 64, BInterval: 5},
			{BEndpointAddress: EndpointOut, BMAttributes: 0x03, WMaxPacketSize: 64, BInterval: 5},
		},
	}},
	Strings: map[uint8]string{
		0: "\x04\x09",
		1: "Sony Interactive Entertainment",
		2: "Wireless Controller",
	},
}
