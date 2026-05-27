package dualsense

const (
	DefaultVID = 0x054C
	DefaultPID = 0x0CE6
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
	FeatureReportCalibration = 0x05
	FeatureReportPairingInfo = 0x09
	FeatureReportFirmware    = 0x20
)

const (
	InputReportSize  = 64
	OutputReportSize = 63
)

const (
	ButtonSquare   uint32 = 0x00000010
	ButtonCross    uint32 = 0x00000020
	ButtonCircle   uint32 = 0x00000040
	ButtonTriangle uint32 = 0x00000080

	ButtonL1      uint32 = 0x00000100
	ButtonR1      uint32 = 0x00000200
	ButtonL2      uint32 = 0x00000400
	ButtonR2      uint32 = 0x00000800
	ButtonCreate  uint32 = 0x00001000
	ButtonOptions uint32 = 0x00002000
	ButtonL3      uint32 = 0x00004000
	ButtonR3      uint32 = 0x00008000

	ButtonPS            uint32 = 0x00010000
	ButtonTouchpadClick uint32 = 0x00020000
	ButtonMicMute       uint32 = 0x00040000
)

const (
	DPadUp    = 0x01
	DPadDown  = 0x02
	DPadLeft  = 0x04
	DPadRight = 0x08
)

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
	DPadMask         = 0x0F
)

const (
	TouchpadMinX uint16 = 0
	TouchpadMaxX uint16 = 1919
	TouchpadMinY uint16 = 0
	TouchpadMaxY uint16 = 1079

	TouchInactiveMask uint8 = 0x80
)

const (
	StatusBatteryFull uint8 = 0x2A
	StatusDefault     uint8 = StatusBatteryFull
)

const (
	OutOffsetReportID    = 0
	OutOffsetValidFlag0  = 1
	OutOffsetValidFlag1  = 2
	OutOffsetRumbleSmall = 3
	OutOffsetRumbleLarge = 4
	OutOffsetValidFlag2  = 39
	OutOffsetLedRed      = 45
	OutOffsetLedGreen    = 46
	OutOffsetLedBlue     = 47
)

const (
	DefaultLedRed   = 0x00
	DefaultLedGreen = 0x00
	DefaultLedBlue  = 0x80
)
