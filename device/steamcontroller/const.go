package steamcontroller

const (
	DefaultVID = 0x28de
	DefaultPID = 0x1102
)

const (
	InputReportID   = 0x01
	InputReportLen  = 64
	InputPayloadLen = 60
)

const (
	FeatureSetDigitalMappings   = 0x80
	FeatureClearDigitalMappings = 0x81
	FeatureGetDigitalMappings   = 0x82
	FeatureGetAttributesValues  = 0x83
	FeatureGetAttributeLabel    = 0x84
	FeatureSetDefaultMappings   = 0x85
	FeatureFactoryReset         = 0x86
	FeatureSetSettingsValues    = 0x87
	FeatureClearSettingsValues  = 0x88
	FeatureGetSettingsValues    = 0x89
	FeatureGetSettingLabel      = 0x8a
	FeatureGetSettingsMaxs      = 0x8b
	FeatureGetSettingsDefaults  = 0x8c
	FeatureSetControllerMode    = 0x8d
	FeatureLoadDefaultSettings  = 0x8e
	FeatureTriggerHapticPulse   = 0x8f
	FeatureTurnOffController    = 0x9f
	FeatureGetDeviceInfo        = 0xa1
	FeatureGetStringAttribute   = 0xae
	FeatureRadioEraseRecords    = 0xaf
	FeatureRadioWriteRecord     = 0xb0
	FeatureSetDongleSetting     = 0xb1
	FeatureDongleDisconnect     = 0xb2
	FeatureDongleCommitDevice   = 0xb3
	FeatureGetWirelessState     = 0xb4
	FeatureCalibrateGyro        = 0xb5
	FeaturePlayAudio            = 0xb6
	FeatureAudioUpdateStart     = 0xb7
	FeatureAudioUpdateData      = 0xb8
	FeatureAudioUpdateComplete  = 0xb9
	FeatureGetChipID            = 0xba
	FeatureResetIMU             = 0xce
	FeatureTriggerHapticCommand = 0xea
	FeatureTriggerRumbleCommand = 0xeb
)

const (
	AttributeUniqueID             = 0x00
	AttributeProductID            = 0x01
	AttributeCapabilities         = 0x02
	AttributeFirmwareBuildTime    = 0x04
	AttributeBoardRevision        = 0x09
	AttributeConnectionIntervalUs = 0x0b
)

const (
	StringAttributeBoardSerial = 0x00
	StringAttributeUnitSerial  = 0x01
)

const (
	SettingLeftTrackpadMode    = 0x07
	SettingRightTrackpadMode   = 0x08
	SettingLizardMode          = 0x09
	SettingSmoothAbsoluteMouse = 0x18
	SettingEnableRawJoystick   = 0x2e
	SettingEnableFastScan      = 0x2f
	SettingIMUMode             = 0x30
	SettingWirelessPacketVer   = 0x31
)

const (
	GyroModeOff             = 0x00
	GyroModeSteering        = 0x01
	GyroModeTilt            = 0x02
	GyroModeSendOrientation = 0x04
	GyroModeSendRawAccel    = 0x08
	GyroModeSendRawGyro     = 0x10
)

const (
	TrackpadModeAbsoluteMouse = 0x00
	TrackpadModeNone          = 0x07
)

const (
	LizardModeOff = 0x00
	LizardModeOn  = 0x01
)

const (
	CapabilityKeyboard = 0x00000001
	CapabilityMouse    = 0x00000002
	CapabilityGamepad  = 0x00000004
	CapabilityAll      = CapabilityKeyboard | CapabilityMouse | CapabilityGamepad
)

const (
	buttonByte8R2 = 0x01
	buttonByte8L2 = 0x02
	buttonByte8R1 = 0x04
	buttonByte8L1 = 0x08
	buttonByte8Y  = 0x10
	buttonByte8B  = 0x20
	buttonByte8X  = 0x40
	buttonByte8A  = 0x80

	buttonByte9Up      = 0x01
	buttonByte9Right   = 0x02
	buttonByte9Left    = 0x04
	buttonByte9Down    = 0x08
	buttonByte9Menu    = 0x10
	buttonByte9Steam   = 0x20
	buttonByte9Options = 0x40
	buttonByte9LGrip   = 0x80

	buttonByte10RGrip      = 0x01
	buttonByte10LPadPress  = 0x02
	buttonByte10RPadPress  = 0x04
	buttonByte10LPadTouch  = 0x08
	buttonByte10RPadTouch  = 0x10
	buttonByte10L3         = 0x40
	buttonByte10LPadAndJoy = 0x80
)

const (
	CommandTypeOff   = 0
	CommandTypeTick  = 1
	CommandTypeClick = 2
)

const (
	PadSideLeft  = 0
	PadSideRight = 1
	PadSideBoth  = 2
)

const (
	IntensityDefault = 0
	IntensityShort   = 1
	IntensityMedium  = 2
	IntensityLong    = 3
	IntensityInsane  = 4
)

const (
	DefaultBatteryMilliVolts = 3000
	MaxAnalogTriggerRaw      = 26000
)
