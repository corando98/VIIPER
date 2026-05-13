package xboxgip

import "encoding/binary"

// gipMetadata is the 182-byte compiled metadata binary from the MS-GIPUSB spec.
// Describes a standard Xbox gamepad (Windows.Xbox.Input.Gamepad) with 3 interface
// GUIDs (IController, IGamepad, INavigationController) and message definitions
// for input report (0x20, 14 bytes up) and rumble (0x09, 9 bytes down).
var gipMetadata = [182]byte{
	0x10, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xB6, 0x00,
	0x77, 0x00, 0x16, 0x00, 0x1B, 0x00, 0x1C, 0x00, 0x23, 0x00, 0x29, 0x00, 0x46, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x06, 0x01, 0x02, 0x03,
	0x04, 0x06, 0x07, 0x05, 0x01, 0x04, 0x05, 0x06, 0x0A, 0x01, 0x1A, 0x00, 0x57, 0x69, 0x6E, 0x64,
	0x6F, 0x77, 0x73, 0x2E, 0x58, 0x62, 0x6F, 0x78, 0x2E, 0x49, 0x6E, 0x70, 0x75, 0x74, 0x2E, 0x47,
	0x61, 0x6D, 0x65, 0x70, 0x61, 0x64, 0x03, 0x56, 0xFF, 0x76, 0x97, 0xFD, 0x9B, 0x81, 0x45, 0xAD,
	0x45, 0xB6, 0x45, 0xBB, 0xA5, 0x26, 0xD6, 0x2C, 0x40, 0x2E, 0x08, 0xDF, 0x07, 0xE1, 0x45, 0xA5,
	0xAB, 0xA3, 0x12, 0x7A, 0xF1, 0x97, 0xB5, 0xE7, 0x1F, 0xF3, 0xB8, 0x86, 0x73, 0xE9, 0x40, 0xA9,
	0xF8, 0x2F, 0x21, 0x26, 0x3A, 0xCF, 0xB7, 0x02, 0x17, 0x00, 0x20, 0x0E, 0x00, 0x01, 0x00, 0x10,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x17,
	0x00, 0x09, 0x09, 0x00, 0x01, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// Extended Compatible ID OS Descriptor (40 bytes).
// Returns "XGIP10" as the compatible ID, triggering xboxgip.sys driver loading.
var extCompatIDDescriptor = [40]byte{
	// Header (16 bytes)
	0x28, 0x00, 0x00, 0x00, // dwLength = 40
	0x00, 0x01,             // bcdVersion = 1.00
	0x04, 0x00,             // wIndex = 4 (Extended Compatible ID)
	0x01,                   // bCount = 1 function
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // reserved
	// Function section (24 bytes)
	0x00,                                                 // bFirstInterfaceNumber = 0
	0x01,                                                 // bNumInterfaces = 1 (MS-GIPUSB §2.2.6: 0x01 for no audio, 0x02 for audio)
	0x58, 0x47, 0x49, 0x50, 0x31, 0x30, 0x00, 0x00,     // compatibleID = "XGIP10\0\0"
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,     // subCompatibleID = "\0\0\0\0\0\0\0\0"
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,                  // reserved
}

// Maximum GIP fragment payload (64-byte USB packet minus 6-byte fragment header).
const gipFragMaxPayload = 58

// buildHelloMessage builds the GIP Device Arrival announcement (command 0x02).
// Format: 4-byte GIP header + 28-byte payload containing device identity.
func buildHelloMessage(seq uint8, vid, pid uint16) []byte {
	msg := make([]byte, 32)
	// GIP header
	msg[0] = GIPAnnounce    // 0x02
	msg[1] = GIPFlagSystem  // 0x20 — system message
	msg[2] = seq
	msg[3] = 28 // payload length

	// Payload: 8-byte serial (Device ID: MSBs = 00 00 FF FB per MS-GIPUSB)
	msg[4] = 0x00
	msg[5] = 0x00
	msg[6] = 0xFF
	msg[7] = 0xFB
	msg[8] = 'V'
	msg[9] = 'I'
	msg[10] = 'P'
	msg[11] = 'R'

	// VID / PID
	binary.LittleEndian.PutUint16(msg[12:14], vid)
	binary.LittleEndian.PutUint16(msg[14:16], pid)

	// Firmware version: Major=1, Minor=0, Build=0, Revision=0
	// MUST match at least one entry in metadata's SupportsDeviceFirmwareVersions
	// (MS-GIPUSB §2.2.14: host stops responding if versions don't match)
	binary.LittleEndian.PutUint16(msg[16:18], 1)
	binary.LittleEndian.PutUint16(msg[18:20], 0)
	binary.LittleEndian.PutUint16(msg[20:22], 0)
	binary.LittleEndian.PutUint16(msg[22:24], 0)

	// Hardware version: Major=1, Minor=0, Build=0, Revision=0
	binary.LittleEndian.PutUint16(msg[24:26], 1)
	binary.LittleEndian.PutUint16(msg[26:28], 0)
	binary.LittleEndian.PutUint16(msg[28:30], 0)
	binary.LittleEndian.PutUint16(msg[30:32], 0)

	return msg
}

// buildStatusMessage builds a GIP Device Status message (command 0x03).
// Reports wired power (no battery) and connected state.
func buildStatusMessage(seq uint8) []byte {
	return []byte{
		GIPStatus,      // 0x03
		GIPFlagSystem,  // 0x20
		seq,
		0x04,           // payload length = 4
		0x00,           // battery: level=0 (wired), type=0 (none/wired)
		0x00, 0x00, 0x00, // reserved
	}
}

// fragmentMetadata splits the 182-byte metadata blob into GIP fragmented
// packets for transport over the 64-byte interrupt endpoint.
// Each fragment is a complete GIP packet ready for EP IN.
func fragmentMetadata(seq uint8) [][]byte {
	data := gipMetadata[:]
	total := len(data)
	var fragments [][]byte

	offset := 0
	first := true
	for offset < total {
		payloadSize := gipFragMaxPayload
		if remaining := total - offset; remaining < payloadSize {
			payloadSize = remaining
		}

		pkt := make([]byte, 6+payloadSize)
		pkt[0] = GIPDescriptor // 0x04

		if first {
			// First fragment: Fragment + InitFrag + System + ACME
			pkt[1] = GIPFlagFragment | GIPFlagInitFrag | GIPFlagSystem | GIPFlagACME // 0xF0
			pkt[2] = seq
			pkt[3] = byte(payloadSize) // this fragment's payload length
			// Total message length in LEB128 (182 = 0xB6)
			pkt[4] = byte(total&0x7F) | 0x80 // low 7 bits + continuation
			pkt[5] = byte(total >> 7)          // high bits
			first = false
		} else {
			// Subsequent fragments: Fragment + System (+ ACME on last)
			flags := byte(GIPFlagFragment | GIPFlagSystem) // 0xA0
			if offset+payloadSize >= total {
				flags |= GIPFlagACME // final fragment needs ACK
			}
			pkt[1] = flags
			pkt[2] = seq // same sequence as first fragment
			pkt[3] = byte(payloadSize)
			// Fragment offset in LEB128
			if offset < 128 {
				pkt[4] = byte(offset)
				pkt[5] = 0x00
			} else {
				pkt[4] = byte(offset&0x7F) | 0x80
				pkt[5] = byte(offset >> 7)
			}
		}

		copy(pkt[6:], data[offset:offset+payloadSize])
		fragments = append(fragments, pkt)
		offset += payloadSize
	}

	return fragments
}

// buildInputReport builds an 18-byte GIP gamepad input report from InputState.
// Format: [0x20, 0x00, seq, 0x0E, buttons_lo, buttons_hi, LT_lo, LT_hi,
//
//	RT_lo, RT_hi, LX_lo, LX_hi, LY_lo, LY_hi, RX_lo, RX_hi, RY_lo, RY_hi]
func buildInputReport(state *InputState, seq uint8) []byte {
	b := make([]byte, gipInputReportSize)
	// GIP header
	b[0] = GIPInput // 0x20
	b[1] = 0x00     // flags: non-system, no ACK
	b[2] = seq
	b[3] = gipInputPayloadSize // 0x0E = 14

	// Buttons: remap XInput → GIP
	buttons := xinputToGipButtons(state.Buttons)
	binary.LittleEndian.PutUint16(b[4:6], buttons)

	// Triggers: scale 0-255 → 0-1023 (10-bit per MS-GIPUSB spec)
	binary.LittleEndian.PutUint16(b[6:8], triggerU8ToU10(state.LT))
	binary.LittleEndian.PutUint16(b[8:10], triggerU8ToU10(state.RT))

	// Sticks: pass through as-is (int16 LE)
	binary.LittleEndian.PutUint16(b[10:12], uint16(state.LX))
	binary.LittleEndian.PutUint16(b[12:14], uint16(state.LY))
	binary.LittleEndian.PutUint16(b[14:16], uint16(state.RX))
	binary.LittleEndian.PutUint16(b[16:18], uint16(state.RY))

	return b
}

// buildAcknowledge builds a GIP ACK message for a received command.
func buildAcknowledge(origSeq uint8, origCmd uint8, bytesReceived uint16) []byte {
	b := make([]byte, 13)
	b[0] = GIPAcknowledge // 0x01
	b[1] = GIPFlagSystem  // 0x20
	b[2] = origSeq        // match the sequence being acknowledged
	b[3] = 0x09           // payload length = 9
	b[4] = 0x00           // unknown
	b[5] = origCmd        // inner command ID being acknowledged
	b[6] = 0x20           // inner client/flags
	binary.LittleEndian.PutUint16(b[7:9], bytesReceived)
	b[9] = 0x00  // unknown
	b[10] = 0x00 // unknown
	binary.LittleEndian.PutUint16(b[11:13], 0) // remaining buffer
	return b
}
