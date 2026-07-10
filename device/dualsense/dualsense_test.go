package dualsense_test

import (
	"context"
	"time"
	"encoding/binary"
	"testing"

	"github.com/Alia5/VIIPER/device/dualsense"
	"github.com/Alia5/VIIPER/usbip"
	"github.com/stretchr/testify/assert"
)

func TestInputReports(t *testing.T) {
	tests := []struct {
		name     string
		state    dualsense.InputState
		validate func(t *testing.T, got []byte)
	}{
		{
			name:  "neutral defaults",
			state: dualsense.InputState{},
			validate: func(t *testing.T, got []byte) {
				assert.Len(t, got, dualsense.InputReportSize)
				assert.Equal(t, byte(dualsense.ReportIDInput), got[0])
				assert.Equal(t, byte(0x80), got[1])
				assert.Equal(t, byte(0x80), got[2])
				assert.Equal(t, byte(0x80), got[3])
				assert.Equal(t, byte(0x80), got[4])
				assert.Equal(t, byte(dualsense.DPadUSBNeutral), got[8]&0x0F)
				assert.Equal(t, byte(dualsense.TouchInactiveMask), got[33])
				assert.Equal(t, byte(dualsense.TouchInactiveMask), got[37])
				assert.Equal(t, byte(dualsense.StatusDefault), got[53])
			},
		},
		{
			name: "buttons axes touch sensors",
			state: dualsense.InputState{
				Buttons:     dualsense.ButtonCross | dualsense.ButtonL1 | dualsense.ButtonPS | dualsense.ButtonTouchpadClick,
				DPad:        dualsense.DPadUp | dualsense.DPadRight,
				L2:          12,
				R2:          34,
				TouchX:      1234,
				TouchY:      987,
				TouchActive: true,
				GyroX:       111,
				GyroY:       -222,
				GyroZ:       333,
				AccelX:      -444,
				AccelY:      555,
				AccelZ:      -666,
			},
			validate: func(t *testing.T, got []byte) {
				assert.Equal(t, byte(12), got[5])
				assert.Equal(t, byte(34), got[6])
				assert.Equal(t, byte(dualsense.DPadUSBUpRight)|0x20, got[8])
				assert.Equal(t, byte(0x0D), got[9])
				assert.Equal(t, byte(0x03), got[10])
				assert.Equal(t, uint16(111), binary.LittleEndian.Uint16(got[16:18]))
				assert.Equal(t, uint16(0xff22), binary.LittleEndian.Uint16(got[18:20]))
				assert.Equal(t, uint16(333), binary.LittleEndian.Uint16(got[20:22]))
				assert.Equal(t, uint16(0xfe44), binary.LittleEndian.Uint16(got[22:24]))
				assert.Equal(t, uint16(555), binary.LittleEndian.Uint16(got[24:26]))
				assert.Equal(t, uint16(0xfd66), binary.LittleEndian.Uint16(got[26:28]))
				assert.Equal(t, byte(0x00), got[33])
				assert.Equal(t, byte(1234&0xFF), got[34])
				assert.Equal(t, byte((1234>>8)&0x0F)|byte((987&0x0F)<<4), got[35])
				assert.Equal(t, byte(987>>4), got[36])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev, err := dualsense.New(nil)
			if !assert.NoError(t, err) {
				return
			}
			dev.UpdateInputState(&tt.state)
			got := dev.HandleTransfer(gateTestCtx(), 4, usbip.DirIn, nil)
			tt.validate(t, got)
		})
	}
}

func TestFeedback(t *testing.T) {
	dev, err := dualsense.New(nil)
	if !assert.NoError(t, err) {
		return
	}

	var got dualsense.OutputState
	dev.SetOutputCallback(func(out dualsense.OutputState) {
		got = out
	})

	cmd := make([]byte, dualsense.OutputReportSize)
	cmd[0] = dualsense.ReportIDOutput
	cmd[dualsense.OutOffsetRumbleSmall] = 0x12
	cmd[dualsense.OutOffsetRumbleLarge] = 0xFE
	cmd[dualsense.OutOffsetLedRed] = 0x01
	cmd[dualsense.OutOffsetLedGreen] = 0x02
	cmd[dualsense.OutOffsetLedBlue] = 0x03

	_, handled := dev.HandleControl(0x21, 0x09, uint16(0x0200|dualsense.ReportIDOutput), 0, uint16(len(cmd)), cmd)
	if !assert.True(t, handled) {
		return
	}
	assert.Equal(t, dualsense.OutputState{
		RumbleSmall: 0x12,
		RumbleLarge: 0xFE,
		LedRed:      0x01,
		LedGreen:    0x02,
		LedBlue:     0x03,
	}, got)
}

// gateTestCtx returns a context with a short deadline so gated interrupt-IN
// HandleTransfer calls in tests never block: a signalled input gate completes
// immediately (GateFresh), an unsignalled one completes at the deadline
// (GateDeadline) — both build a report from current state. (Port of upstream
// data-driven completion, 7e33d2d3.)
func gateTestCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = cancel
	return ctx
}
