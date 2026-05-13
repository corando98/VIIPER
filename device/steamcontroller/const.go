// Package steamcontroller emulates the Valve Steam Controller family
// (Steam Deck and the broader 0x28DE generic-handheld PID space used by
// Steam Input on third-party devices such as MSI Claw, ROG Ally, Zotac
// Zone, and Lenovo Legion Go / Legion Go S / Legion Go 2).
//
// Steam targets share the InputState wire format defined in
// internal/inputstate/elite2 with the xboxelite2 package
// (gyro + accel + right touchpad live in bytes 14..32). Xbox
// targets simply ignore those bytes; Steam targets pack them into the
// vendor-defined 64-byte Steam Deck input report.
package steamcontroller

import "github.com/Alia5/VIIPER/usb"

const (
	// DefaultVIDSteam is Valve Corporation.
	DefaultVIDSteam uint16 = 0x28DE

	// Default PIDs Steam Input recognises as a "Steam Controller" family.
	// 0x1205 is the real Steam Deck; the 0x12Fx range was carved out for
	// third-party handhelds shipping with Steam Input support.
	DefaultPIDSteamDeck            uint16 = 0x1205
	DefaultPIDSteamGeneric         uint16 = 0x12F0
	DefaultPIDSteamMsiClaw         uint16 = 0x12FA
	DefaultPIDSteamLenovoLegionGo2 uint16 = 0x12FB
	DefaultPIDSteamZotacZone       uint16 = 0x12FC
	DefaultPIDSteamAsusRogAlly     uint16 = 0x12FD
	DefaultPIDSteamLenovoLegionGo  uint16 = 0x12FE
	DefaultPIDSteamLenovoLegionGoS uint16 = 0x12FF
)

const (
	EndpointIn  = 0x81
	EndpointOut = 0x01
)

const (
	// Steam Deck-style vendor input/output report identifiers.
	SteamDeckInputMajorVersion = 0x01
	SteamDeckInputMinorVersion = 0x00
	SteamDeckInputReportType   = 0x09
	SteamDeckRumbleCommandType = 0xEB

	// Steam Deck vendor input/output reports are 64 bytes.
	InputReportSizeSteamDeck  = 64
	OutputReportSizeSteamDeck = 64
)

// Profile identifiers for the Steam Controller family.
const (
	ProfileSteamDeck    = "steamdeck"
	ProfileSteamGeneric = "steamdeck-generic"
)

// steamDeckControllerHIDDescriptor mirrors InputPlumber / HID captures of
// the real Steam Deck controller. It is a vendor-defined collection that
// carries 64-byte input and 64-byte feature reports — Steam Input parses
// the payload itself, the HID descriptor only declares the byte stream.
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

// defaultDescriptor is the baseline USB descriptor before profile-specific
// overrides are applied (VID/PID/strings). All Steam profiles use the same
// vendor HID descriptor and 64-byte endpoint packet size.
var defaultDescriptor = usb.Descriptor{
	Device: usb.DeviceDescriptor{
		BcdUSB:             0x0200,
		BDeviceClass:       0x00,
		BDeviceSubClass:    0x00,
		BDeviceProtocol:    0x00,
		BMaxPacketSize0:    0x40,
		IDVendor:           DefaultVIDSteam,
		IDProduct:          DefaultPIDSteamDeck,
		BcdDevice:          0x0509,
		IManufacturer:      0x01,
		IProduct:           0x02,
		ISerialNumber:      0x03,
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
				ReportRaw: steamDeckControllerHIDDescriptor,
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
		0: "\x04\x09",
		1: "Valve Software",
		2: "Steam Deck Controller",
		3: "VIIPER-SD-05",
	},
}
