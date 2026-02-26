package xboxelite2

import "github.com/Alia5/VIIPER/usb"

const (
	DefaultVID uint16 = 0x045E // Microsoft
	DefaultVIDSteam uint16 = 0x28DE // Valve
	// Keep the default PID on a non-GIP value so Windows binds to HID instead of xboxgip.sys.
	DefaultPID uint16 = 0x0B02

	DefaultPIDElite2 uint16 = DefaultPID
	DefaultPIDElite2GIP uint16 = 0x0B00
	DefaultPIDXboxOneElite uint16 = 0x02E3
	DefaultPIDXboxSeries uint16 = 0x0B12
	DefaultPIDSteamDeck uint16 = 0x1205
	DefaultPIDSteamGeneric uint16 = 0x12F0
	DefaultPIDSteamMsiClaw uint16 = 0x12FA
	DefaultPIDSteamLenovoLegionGo2 uint16 = 0x12FB
	DefaultPIDSteamZotacZone uint16 = 0x12FC
	DefaultPIDSteamAsusRogAlly uint16 = 0x12FD
	DefaultPIDSteamLenovoLegionGo uint16 = 0x12FE
	DefaultPIDSteamLenovoLegionGoS uint16 = 0x12FF
)

const (
	EndpointIn  = 0x81
	EndpointOut = 0x01
)

const (
	ReportIDInput  = 0x01
	ReportIDOutput = 0x02
)

const (
	SteamDeckInputMajorVersion = 0x01
	SteamDeckInputMinorVersion = 0x00
	SteamDeckInputReportType   = 0x09
	SteamDeckRumbleCommandType = 0xEB
)

const (
	// 1 byte report ID + 16 bytes payload:
	// 4x16-bit sticks + 2x10-bit triggers + hat + 17 buttons + padding.
	InputReportSize = 17
	// Steam Deck input report is a 64-byte vendor report.
	InputReportSizeSteamDeck = 64
	// 1 byte report ID + 4 bytes rumble payload.
	OutputReportSize = 5
	// Steam Deck rumble command reports are 64 bytes.
	OutputReportSizeSteamDeck = 64
)

// Button constants for the wire protocol (u16 bitmask, XInput-compatible order).
const (
	ButtonA      uint16 = 0x0001
	ButtonB      uint16 = 0x0002
	ButtonX      uint16 = 0x0004
	ButtonY      uint16 = 0x0008
	ButtonLB     uint16 = 0x0010
	ButtonRB     uint16 = 0x0020
	ButtonBack   uint16 = 0x0040
	ButtonStart  uint16 = 0x0080
	ButtonLThumb uint16 = 0x0100
	ButtonRThumb uint16 = 0x0200
	ButtonGuide  uint16 = 0x0400

	// Elite 2 paddle buttons
	ButtonP1 uint16 = 0x1000
	ButtonP2 uint16 = 0x2000
	ButtonP3 uint16 = 0x4000
	ButtonP4 uint16 = 0x8000
)

// Reserved byte bitmask used by InputState.Reserved.
const (
	ReservedShare uint8 = 0x01 // Share / Capture
)

// DPad wire values (bitmask).
const (
	DPadUp    = 0x01
	DPadDown  = 0x02
	DPadLeft  = 0x04
	DPadRight = 0x08
)

// DPad USB hat values.
// 0 = neutral, 1..8 = Up, UpRight, ..., UpLeft.
const (
	DPadUSBNeutral   = 0x00
	DPadUSBUp        = 0x01
	DPadUSBUpRight   = 0x02
	DPadUSBRight     = 0x03
	DPadUSBDownRight = 0x04
	DPadUSBDown      = 0x05
	DPadUSBDownLeft  = 0x06
	DPadUSBLeft      = 0x07
	DPadUSBUpLeft    = 0x08
)

const DPadMask uint8 = 0x0F

const (
	ProfileElite2       = "elite2"
	ProfileElite2GIP    = "elite2-gip"
	ProfileXboxOneElite = "xbox-one-elite"
	ProfileXboxSeries   = "xbox-series"
	ProfileSteamDeck    = "steamdeck"
	ProfileSteamGeneric = "steamdeck-generic"
)

// Xbox HID descriptor builder:
// report payload is always 16 bytes (17 including report ID), while the exposed button
// count changes by profile to alter host-visible capability shape.
func makeXboxHIDDescriptor(buttonCount uint8) []byte {
	if buttonCount == 0 || buttonCount > 24 {
		buttonCount = 17
	}
	paddingCount := uint8(24 - buttonCount)

	return []byte{
		0x05, 0x01, // Usage Page (Generic Desktop)
		0x09, 0x05, // Usage (Game Pad)
		0xA1, 0x01, // Collection (Application)

		// Input report ID 0x01.
		0x85, 0x01, // Report ID (1)

		// Sticks: X, Y, Z, Rz as 16-bit unsigned.
		0x09, 0x30, // Usage (X)
		0x09, 0x31, // Usage (Y)
		0x09, 0x32, // Usage (Z)
		0x09, 0x35, // Usage (Rz)
		0x15, 0x00, // Logical Minimum (0)
		0x27, 0xFF, 0xFF, 0x00, 0x00, // Logical Maximum (65535)
		0x75, 0x10, // Report Size (16)
		0x95, 0x04, // Report Count (4)
		0x81, 0x02, // Input (Data,Var,Abs)

		// Triggers: LT/RT as 10-bit values (0..1023), each padded to 16 bits.
		0x05, 0x02, // Usage Page (Simulation Controls)
		0x09, 0xC5, // Usage (Brake)      - LT
		0x15, 0x00, // Logical Minimum (0)
		0x26, 0xFF, 0x03, // Logical Maximum (1023)
		0x75, 0x0A, // Report Size (10)
		0x95, 0x01, // Report Count (1)
		0x81, 0x02, // Input (Data,Var,Abs)
		0x75, 0x06, // Report Size (6)
		0x95, 0x01, // Report Count (1)
		0x81, 0x03, // Input (Const,Var,Abs)

		0x09, 0xC4, // Usage (Accelerator) - RT
		0x15, 0x00, // Logical Minimum (0)
		0x26, 0xFF, 0x03, // Logical Maximum (1023)
		0x75, 0x0A, // Report Size (10)
		0x95, 0x01, // Report Count (1)
		0x81, 0x02, // Input (Data,Var,Abs)
		0x75, 0x06, // Report Size (6)
		0x95, 0x01, // Report Count (1)
		0x81, 0x03, // Input (Const,Var,Abs)

		// Hat switch (DPad): 0 = neutral, 1..8 = directions.
		0x05, 0x01, // Usage Page (Generic Desktop)
		0x09, 0x39, // Usage (Hat switch)
		0x15, 0x00, // Logical Minimum (0)
		0x25, 0x08, // Logical Maximum (8)
		0x35, 0x00, // Physical Minimum (0)
		0x46, 0x3B, 0x01, // Physical Maximum (315)
		0x65, 0x14, // Unit (Eng Rotation)
		0x75, 0x04, // Report Size (4)
		0x95, 0x01, // Report Count (1)
		0x81, 0x42, // Input (Data,Var,Abs,Null)
		0x75, 0x04, // Report Size (4)
		0x95, 0x01, // Report Count (1)
		0x81, 0x03, // Input (Const,Var,Abs)

		// Profile-specific button count.
		0x05, 0x09, // Usage Page (Button)
		0x19, 0x01, // Usage Minimum (1)
		0x29, buttonCount, // Usage Maximum
		0x15, 0x00, // Logical Minimum (0)
		0x25, 0x01, // Logical Maximum (1)
		0x75, 0x01, // Report Size (1)
		0x95, buttonCount, // Report Count (buttons)
		0x81, 0x02, // Input (Data,Var,Abs)
		0x75, 0x01, // Report Size (1)
		0x95, paddingCount, // Report Count (padding)
		0x81, 0x03, // Input (Const,Var,Abs)

		// Output report ID 0x02 (rumble).
		0x85, 0x02, // Report ID (2)
		0x06, 0x00, 0xFF, // Usage Page (Vendor Defined)
		0x09, 0x01, // Usage (Vendor Usage 1)
		0x15, 0x00, // Logical Minimum (0)
		0x26, 0xFF, 0x00, // Logical Maximum (255)
		0x75, 0x08, // Report Size (8)
		0x95, 0x04, // Report Count (4)
		0x91, 0x02, // Output (Data,Var,Abs)

		// Feature report ID 0x03 (firmware info placeholder).
		0x85, 0x03, // Report ID (3)
		0x06, 0x00, 0xFF, // Usage Page (Vendor Defined)
		0x09, 0x02, // Usage (Vendor Usage 2)
		0x75, 0x08, // Report Size (8)
		0x95, 0x04, // Report Count (4)
		0xB1, 0x02, // Feature (Data,Var,Abs)

		0xC0, // End Collection
	}
}

var xboxElite2HIDDescriptor = makeXboxHIDDescriptor(17)
var xboxOneEliteHIDDescriptor = makeXboxHIDDescriptor(15)
var xboxSeriesHIDDescriptor = makeXboxHIDDescriptor(12)

// Vendor-defined Steam Deck controller descriptor (mirrors InputPlumber/HID captures).
var steamDeckControllerHIDDescriptor = []byte{
	0x06, 0xff, 0xff, // Usage Page (Vendor Usage Page 0xffff)
	0x09, 0x01, // Usage (Vendor Usage 0x01)
	0xa1, 0x01, // Collection (Application)
	0x09, 0x02, //  Usage (Vendor Usage 0x02)
	0x09, 0x03, //  Usage (Vendor Usage 0x03)
	0x15, 0x00, //  Logical Minimum (0)
	0x26, 0xff, 0x00, //  Logical Maximum (255)
	0x75, 0x08, //  Report Size (8)
	0x95, 0x40, //  Report Count (64)
	0x81, 0x02, //  Input (Data,Var,Abs)
	0x09, 0x06, //  Usage (Vendor Usage 0x06)
	0x09, 0x07, //  Usage (Vendor Usage 0x07)
	0x15, 0x00, //  Logical Minimum (0)
	0x26, 0xff, 0x00, //  Logical Maximum (255)
	0x75, 0x08, //  Report Size (8)
	0x95, 0x40, //  Report Count (64)
	0xb1, 0x02, //  Feature (Data,Var,Abs)
	0xc0, // End Collection
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
				ReportRaw: xboxElite2HIDDescriptor,
			},
			Endpoints: []usb.EndpointDescriptor{
				{
					BEndpointAddress: EndpointIn,
					BMAttributes:     0x03, // Interrupt
					WMaxPacketSize:   32,
					BInterval:        4, // 4ms = 250Hz
				},
				{
					BEndpointAddress: EndpointOut,
					BMAttributes:     0x03, // Interrupt
					WMaxPacketSize:   32,
					BInterval:        4,
				},
			},
		},
	},
	Strings: map[uint8]string{
		0: "\x04\x09",
		1: "Microsoft",
		2: "Xbox Elite Wireless Controller Series 2",
	},
}
