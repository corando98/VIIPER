package dualsenseedge

const (
	DefaultVID = 0x054C // Sony Interactive Entertainment
	DefaultPID = 0x0DF2 // DualSense Edge
)

const (
	EndpointIn  = 0x84
	EndpointOut = 0x03
)

const (
	ReportIDInput  = 0x01
	ReportIDOutput = 0x02
)

const (
	InputReportSize  = 64
	OutputReportSize = 64
)

// Button constants for the wire protocol (u32 bitmask).
// These map to DS5 HID buttons 1-15 plus vendor bits.
const (
	ButtonSquare   uint32 = 0x0010
	ButtonCross    uint32 = 0x0020
	ButtonCircle   uint32 = 0x0040
	ButtonTriangle uint32 = 0x0080

	ButtonL1      uint32 = 0x0100
	ButtonR1      uint32 = 0x0200
	ButtonL2      uint32 = 0x0400
	ButtonR2      uint32 = 0x0800
	ButtonCreate  uint32 = 0x1000
	ButtonOptions uint32 = 0x2000
	ButtonL3      uint32 = 0x4000
	ButtonR3      uint32 = 0x8000

	ButtonPS       uint32 = 0x00010000
	ButtonTouchpad uint32 = 0x00020000
	ButtonMute     uint32 = 0x00040000

	// Edge-specific paddles
	ButtonExtraR2 uint32 = 0x00080000
	ButtonExtraL2 uint32 = 0x00100000
	ButtonExtraR1 uint32 = 0x00200000
	ButtonExtraL1 uint32 = 0x00400000
	ButtonExtraL3 uint32 = 0x00800000
)

// DPad wire values (bitmask).
const (
	DPadUp    = 0x01
	DPadDown  = 0x02
	DPadLeft  = 0x04
	DPadRight = 0x08
)

// DPad USB hat switch values (0-8).
const (
	DPadUSBUp        = 0x00
	DPadUSBUpRight   = 0x01
	DPadUSBRight     = 0x02
	DPadUSBDownRight = 0x03
	DPadUSBDown      = 0x04
	DPadUSBDownLeft  = 0x05
	DPadUSBLeft      = 0x06
	DPadUSBUpLeft    = 0x07
	DPadUSBNeutral   = 0x08
)

const DPadMask uint8 = 0x0F

// Gyro/Accel scale factors matching the USB report domain.
//   Gyro: BMI323 ±2000 dps passthrough = 16.384 LSB/dps
//   Accel: BMI323 4096 LSB/g × ScaleAccel(×2) = 8192 LSB/g = 835.07 LSB/(m/s²)
const (
	GyroCountsPerDps  = 16.384
	AccelCountsPerMS2 = 835.07 // 8192 / 9.81
)

// Default accelerometer values (controller flat on table, before C# input starts).
const (
	DefaultAccelXRaw int16 = 0
	DefaultAccelYRaw int16 = 0
	DefaultAccelZRaw int16 = -8192 // -1g (8192 counts/g)
)

// Touchpad dimensions.
const (
	TouchpadMaxX uint16 = 1920
	TouchpadMaxY uint16 = 1080

	TouchInactiveMask uint8 = 0x80
)

// Timestamp resolution in nanoseconds.
const DeltaTimeNS = 333

// Battery constants.
const (
	BatteryFullyCharged = 0x2A // Status=0x2 (Full), Level=0xA (100%)
)

// Output report offsets (relative to report data, after report ID).
const (
	OutOffsetReportID    = 0
	OutOffsetFlag0       = 1
	OutOffsetFlag1       = 2
	OutOffsetRumbleSmall = 3 // Right motor (weak)
	OutOffsetRumbleLarge = 4 // Left motor (strong)
	OutOffsetFlag2       = 39
	OutOffsetLightbar    = 42
	OutOffsetLEDBright   = 43
	OutOffsetPlayerLEDs  = 44
	OutOffsetLedRed      = 45
	OutOffsetLedGreen    = 46
	OutOffsetLedBlue     = 47
)
