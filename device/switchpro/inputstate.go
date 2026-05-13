package switchpro

import (
	"encoding/binary"
	"fmt"
)

// InputState is the wire format sent from the C# client to the Go device.
// 24 bytes total.
type InputState struct {
	Buttons uint32 // All buttons packed (byte0 | byte1<<8 | byte2<<16)
	LX, LY  int16  // Left stick (±32767, center=0)
	RX, RY  int16  // Right stick
	GyroX   int16
	GyroY   int16
	GyroZ   int16
	AccelX  int16
	AccelY  int16
	AccelZ  int16
}

const InputStateSize = 24

func (s *InputState) UnmarshalBinary(data []byte) error {
	if len(data) < InputStateSize {
		return fmt.Errorf("switchpro: input state too short: %d < %d", len(data), InputStateSize)
	}
	s.Buttons = binary.LittleEndian.Uint32(data[0:4])
	s.LX = int16(binary.LittleEndian.Uint16(data[4:6]))
	s.LY = int16(binary.LittleEndian.Uint16(data[6:8]))
	s.RX = int16(binary.LittleEndian.Uint16(data[8:10]))
	s.RY = int16(binary.LittleEndian.Uint16(data[10:12]))
	s.GyroX = int16(binary.LittleEndian.Uint16(data[12:14]))
	s.GyroY = int16(binary.LittleEndian.Uint16(data[14:16]))
	s.GyroZ = int16(binary.LittleEndian.Uint16(data[16:18]))
	s.AccelX = int16(binary.LittleEndian.Uint16(data[18:20]))
	s.AccelY = int16(binary.LittleEndian.Uint16(data[20:22]))
	s.AccelZ = int16(binary.LittleEndian.Uint16(data[22:24]))
	return nil
}

// OutputState is the wire format sent from the Go device back to the C# client.
// 2 bytes total (simplified rumble amplitude).
type OutputState struct {
	RumbleLeft  uint8
	RumbleRight uint8
}

const OutputStateSize = 2

func (o *OutputState) MarshalBinary() ([]byte, error) {
	return []byte{o.RumbleLeft, o.RumbleRight}, nil
}

func (o *OutputState) UnmarshalBinary(data []byte) error {
	if len(data) < OutputStateSize {
		return fmt.Errorf("switchpro: output state too short: %d < %d", len(data), OutputStateSize)
	}
	o.RumbleLeft = data[0]
	o.RumbleRight = data[1]
	return nil
}
