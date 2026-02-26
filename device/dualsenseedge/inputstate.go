package dualsenseedge

import (
	"encoding/binary"
	"io"
)

// viiper:wire dualsenseedge c2s stickLX:i8 stickLY:i8 stickRX:i8 stickRY:i8 buttons:u32 dpad:u8 triggerL2:u8 triggerR2:u8 touch1X:u16 touch1Y:u16 touch1Active:bool touch2X:u16 touch2Y:u16 touch2Active:bool gyroX:i16 gyroY:i16 gyroZ:i16 accelX:i16 accelY:i16 accelZ:i16
type InputState struct {
	LX, LY  int8
	RX, RY  int8
	Buttons uint32
	DPad    uint8
	L2, R2  uint8

	Touch1X, Touch1Y uint16
	Touch1Active     bool
	Touch2X, Touch2Y uint16
	Touch2Active     bool

	GyroX, GyroY, GyroZ   int16
	AccelX, AccelY, AccelZ int16
}

const InputStateSize = 33

func (s *InputState) MarshalBinary() ([]byte, error) {
	b := make([]byte, InputStateSize)
	b[0] = uint8(s.LX)
	b[1] = uint8(s.LY)
	b[2] = uint8(s.RX)
	b[3] = uint8(s.RY)
	binary.LittleEndian.PutUint32(b[4:8], s.Buttons)
	b[8] = s.DPad
	b[9] = s.L2
	b[10] = s.R2
	binary.LittleEndian.PutUint16(b[11:13], s.Touch1X)
	binary.LittleEndian.PutUint16(b[13:15], s.Touch1Y)
	if s.Touch1Active {
		b[15] = 1
	}
	binary.LittleEndian.PutUint16(b[16:18], s.Touch2X)
	binary.LittleEndian.PutUint16(b[18:20], s.Touch2Y)
	if s.Touch2Active {
		b[20] = 1
	}
	binary.LittleEndian.PutUint16(b[21:23], uint16(s.GyroX))
	binary.LittleEndian.PutUint16(b[23:25], uint16(s.GyroY))
	binary.LittleEndian.PutUint16(b[25:27], uint16(s.GyroZ))
	binary.LittleEndian.PutUint16(b[27:29], uint16(s.AccelX))
	binary.LittleEndian.PutUint16(b[29:31], uint16(s.AccelY))
	binary.LittleEndian.PutUint16(b[31:33], uint16(s.AccelZ))
	return b, nil
}

func (s *InputState) UnmarshalBinary(data []byte) error {
	if len(data) < InputStateSize {
		return io.ErrUnexpectedEOF
	}
	s.LX = int8(data[0])
	s.LY = int8(data[1])
	s.RX = int8(data[2])
	s.RY = int8(data[3])
	s.Buttons = binary.LittleEndian.Uint32(data[4:8])
	s.DPad = data[8]
	s.L2 = data[9]
	s.R2 = data[10]
	s.Touch1X = binary.LittleEndian.Uint16(data[11:13])
	s.Touch1Y = binary.LittleEndian.Uint16(data[13:15])
	s.Touch1Active = data[15] != 0
	s.Touch2X = binary.LittleEndian.Uint16(data[16:18])
	s.Touch2Y = binary.LittleEndian.Uint16(data[18:20])
	s.Touch2Active = data[20] != 0
	s.GyroX = int16(binary.LittleEndian.Uint16(data[21:23]))
	s.GyroY = int16(binary.LittleEndian.Uint16(data[23:25]))
	s.GyroZ = int16(binary.LittleEndian.Uint16(data[25:27]))
	s.AccelX = int16(binary.LittleEndian.Uint16(data[27:29]))
	s.AccelY = int16(binary.LittleEndian.Uint16(data[29:31]))
	s.AccelZ = int16(binary.LittleEndian.Uint16(data[31:33]))
	return nil
}

// viiper:wire dualsenseedge s2c rumbleSmall:u8 rumbleLarge:u8 ledRed:u8 ledGreen:u8 ledBlue:u8 playerLeds:u8
type OutputState struct {
	RumbleSmall uint8
	RumbleLarge uint8
	LedRed      uint8
	LedGreen    uint8
	LedBlue     uint8
	PlayerLeds  uint8
}

const OutputStateSize = 6

func (f *OutputState) MarshalBinary() ([]byte, error) {
	return []byte{
		f.RumbleSmall,
		f.RumbleLarge,
		f.LedRed,
		f.LedGreen,
		f.LedBlue,
		f.PlayerLeds,
	}, nil
}

func (f *OutputState) UnmarshalBinary(data []byte) error {
	if len(data) < OutputStateSize {
		return io.ErrUnexpectedEOF
	}
	f.RumbleSmall = data[0]
	f.RumbleLarge = data[1]
	f.LedRed = data[2]
	f.LedGreen = data[3]
	f.LedBlue = data[4]
	f.PlayerLeds = data[5]
	return nil
}
