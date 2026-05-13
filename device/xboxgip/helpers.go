package xboxgip

// triggerU8ToU10 scales trigger range 0..255 to 0..1023 (GIP 10-bit per MS-GIPUSB spec).
func triggerU8ToU10(v uint8) uint16 {
	return uint16((uint32(v)*1023 + 127) / 255)
}

// xinputToGipButtons remaps an XInput button bitmask (uint16) to GIP layout.
func xinputToGipButtons(xinput uint32) uint16 {
	var gip uint16
	if xinput&xinputDPadUp != 0 {
		gip |= GIPBtnDPadUp
	}
	if xinput&xinputDPadDown != 0 {
		gip |= GIPBtnDPadDown
	}
	if xinput&xinputDPadLeft != 0 {
		gip |= GIPBtnDPadLeft
	}
	if xinput&xinputDPadRight != 0 {
		gip |= GIPBtnDPadRight
	}
	if xinput&xinputStart != 0 {
		gip |= GIPBtnMenu
	}
	if xinput&xinputBack != 0 {
		gip |= GIPBtnView
	}
	if xinput&xinputLThumb != 0 {
		gip |= GIPBtnLStick
	}
	if xinput&xinputRThumb != 0 {
		gip |= GIPBtnRStick
	}
	if xinput&xinputLB != 0 {
		gip |= GIPBtnLB
	}
	if xinput&xinputRB != 0 {
		gip |= GIPBtnRB
	}
	if xinput&xinputA != 0 {
		gip |= GIPBtnA
	}
	if xinput&xinputB != 0 {
		gip |= GIPBtnB
	}
	if xinput&xinputX != 0 {
		gip |= GIPBtnX
	}
	if xinput&xinputY != 0 {
		gip |= GIPBtnY
	}
	// Guide button (xinputGuide) is NOT in the main GIP input report.
	// It uses a separate Virtual Keycode message (0x07).
	return gip
}
