package xboxgip

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alia5/VIIPER/device"
	"github.com/Alia5/VIIPER/usb"
	"github.com/Alia5/VIIPER/usbip"
)

// XboxGIP implements a GIP (Game Input Protocol) Xbox Series X|S controller.
type XboxGIP struct {
	inputState *InputState
	stateMu    sync.Mutex
	outputFunc func(OutputState)
	descriptor usb.Descriptor

	// GIP protocol state: only Arrival (0) and Active (2).
	// No Idle state — device stays in Arrival until host sends SetState START.
	// Metadata is served from the response queue with priority over Hello.
	gipState  int32  // atomic: stateArrival / stateActive
	seqDevice uint32 // atomic: device-side sequence counter (1-255)

	// Response queue for EP IN (metadata fragments, acks, status).
	// These are returned with priority over Hello/input messages.
	respQueue chan []byte

	// Hello is sent once; cached message is replayed until host advances the handshake.
	helloSent int32  // atomic: 1 after first Hello sent
	helloMsg  []byte // cached Hello message (built on first send)

	// Debug log file
	debugLog    *os.File
	loggedFirst int32 // atomic: set to 1 after first input report is logged
}

type xboxGIPCreateOptions struct{}

// msOSVendorCode is used in the MS OS String Descriptor and as bRequest
// for Extended Compatible ID queries.
const msOSVendorCode = 0x90

// New returns a new XboxGIP device.
func New(o *device.CreateOptions) (*XboxGIP, error) {
	d := &XboxGIP{
		descriptor: makeDescriptor(),
		respQueue:  make(chan []byte, 32),
	}
	atomic.StoreUint32(&d.seqDevice, 1)
	// Start in Arrival state — xboxgip.sys expects Hello (0x02) first,
	// then drives the Metadata/SetState handshake before accepting input reports.
	atomic.StoreInt32(&d.gipState, stateArrival)

	// Open debug log file next to the executable.
	if exePath, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(exePath), "xboxgip_debug.log")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
			d.debugLog = f
			d.logf("GIP device created")
		}
	}

	if o != nil {
		if o.IdVendor != nil {
			d.descriptor.Device.IDVendor = *o.IdVendor
		}
		if o.IdProduct != nil {
			d.descriptor.Device.IDProduct = *o.IdProduct
		}
		if o.DeviceSpecific != nil {
			data, err := json.Marshal(o.DeviceSpecific)
			if err != nil {
				return nil, fmt.Errorf("invalid JSON payload: %w", err)
			}
			var args xboxGIPCreateOptions
			if err = json.Unmarshal(data, &args); err != nil {
				return nil, fmt.Errorf("invalid JSON payload: %w", err)
			}
		}
	}

	d.logf("VID=0x%04X PID=0x%04X", d.descriptor.Device.IDVendor, d.descriptor.Device.IDProduct)
	return d, nil
}

// logf writes a timestamped line to the debug log file.
func (d *XboxGIP) logf(format string, args ...any) {
	if d.debugLog == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(d.debugLog, "%s  %s\n", ts, msg)
	d.debugLog.Sync()
}

// nextSeq returns the next GIP sequence number (1-255 wrapping).
func (d *XboxGIP) nextSeq() uint8 {
	for {
		old := atomic.LoadUint32(&d.seqDevice)
		next := old + 1
		if next > 255 {
			next = 1
		}
		if atomic.CompareAndSwapUint32(&d.seqDevice, old, next) {
			return uint8(next)
		}
	}
}

// SetOutputCallback sets the callback for force-feedback data (rumble).
func (d *XboxGIP) SetOutputCallback(f func(OutputState)) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.outputFunc = f
}

// UpdateInputState updates the current controller state (thread-safe).
func (d *XboxGIP) UpdateInputState(state *InputState) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.inputState = state
}

// HandleTransfer implements the GIP protocol over interrupt IN/OUT endpoints.
func (d *XboxGIP) HandleTransfer(ep uint32, dir uint32, out []byte) []byte {
	d.logf("HandleTransfer ep=%d dir=%d outLen=%d", ep, dir, len(out))
	if dir == usbip.DirIn && ep == 1 {
		return d.handleEPIn()
	}
	if dir == usbip.DirOut && ep == 1 {
		d.handleEPOut(out)
	}
	// EP2 (interface 1 — GIP secondary): return idle NAK-equivalent empty packet.
	if dir == usbip.DirIn && ep == 2 {
		return []byte{0x00}
	}
	return nil
}

// handleEPIn returns data for the host.
// Priority: queued responses (metadata, acks) > Hello (Arrival) / input (Active).
// NEVER returns nil — always sends data to avoid 0-byte USBIP responses.
func (d *XboxGIP) handleEPIn() []byte {
	// Check response queue first (metadata fragments, acks, status).
	select {
	case resp := <-d.respQueue:
		d.logf("EP IN: queued response len=%d type=0x%02X", len(resp), resp[0])
		return resp
	default:
	}

	state := atomic.LoadInt32(&d.gipState)
	switch state {
	case stateActive:
		// Send current input report.
		d.stateMu.Lock()
		var st InputState
		if d.inputState != nil {
			st = *d.inputState
		}
		d.stateMu.Unlock()
		rpt := buildInputReport(&st, d.nextSeq())
		// Log the first input report for debugging (raw hex).
		if atomic.CompareAndSwapInt32(&d.loggedFirst, 0, 1) {
			d.logf("EP IN: first input report hex=%X buttons=0x%04X LT=%d RT=%d LX=%d LY=%d RX=%d RY=%d",
				rpt, st.Buttons, st.LT, st.RT, st.LX, st.LY, st.RX, st.RY)
		}
		return rpt

	default:
		// Arrival: send Hello, then proactively push metadata + status.
		// xboxgip.sys never sends EP1 OUT over USBIP, so we can't wait for
		// a Descriptor request. Push the full handshake sequence on EP IN.
		if atomic.CompareAndSwapInt32(&d.helloSent, 0, 1) {
			vid := d.descriptor.Device.IDVendor
			pid := d.descriptor.Device.IDProduct
			d.helloMsg = buildHelloMessage(1, vid, pid)
			d.logf("EP IN: sending Hello → metadata → status → Active")

			// Queue metadata fragments.
			seq := d.nextSeq()
			fragments := fragmentMetadata(seq)
			for _, frag := range fragments {
				select {
				case d.respQueue <- frag:
				default:
				}
			}
			// Queue status message.
			status := buildStatusMessage(d.nextSeq())
			select {
			case d.respQueue <- status:
			default:
			}

			// Go Active immediately — xboxgip.sys never sends EP1 OUT
			// over USBIP, so we can't wait for SetState START.
			atomic.StoreInt32(&d.gipState, stateActive)
			d.logf("EP IN: forced transition to Active")
		}
		return d.helloMsg
	}
}

// handleEPOut processes GIP commands from the host on EP OUT.
func (d *XboxGIP) handleEPOut(data []byte) {
	if len(data) < 4 {
		return
	}

	msgType := data[0]
	flags := data[1]
	seq := data[2]

	d.logf("EP OUT: type=0x%02X flags=0x%02X seq=%d len=%d data=%X",
		msgType, flags, seq, len(data), data)

	switch msgType {
	case GIPDescriptor: // 0x04 — Metadata Request
		d.handleMetadataRequest(seq)

	case GIPSetState: // 0x05 — Set Device State
		if len(data) >= 5 {
			d.handleSetState(data[4], seq)
		}

	case GIPAuthenticate: // 0x06 — Auth
		d.logf("GIP: ignoring auth message seq=%d", seq)

	case GIPRumble: // 0x09 — Force Feedback
		d.handleRumble(data)

	case GIPLedControl: // 0x0A — LED
		d.logf("GIP: ignoring LED control seq=%d", seq)

	case GIPAcknowledge: // 0x01 — ACK from host
		d.logf("GIP: received ACK seq=%d", seq)

	default:
		d.logf("GIP: unhandled type=0x%02X flags=0x%02X seq=%d", msgType, flags, seq)
	}
}

// handleMetadataRequest responds to a metadata request by queueing
// the fragmented metadata blob and a device status message.
func (d *XboxGIP) handleMetadataRequest(hostSeq uint8) {
	d.logf("GIP: metadata request received (hostSeq=%d), queueing metadata", hostSeq)
	slog.Info("GIP: metadata request received, sending metadata")

	// Fragment the 182-byte metadata blob.
	seq := d.nextSeq()
	fragments := fragmentMetadata(seq)
	for i, frag := range fragments {
		select {
		case d.respQueue <- frag:
			d.logf("GIP: queued metadata fragment %d/%d len=%d", i+1, len(fragments), len(frag))
		default:
			d.logf("GIP: WARNING response queue full, dropping fragment %d", i+1)
			slog.Warn("GIP: response queue full, dropping metadata fragment")
		}
	}

	// Also queue a device status message (wired, full power).
	status := buildStatusMessage(d.nextSeq())
	select {
	case d.respQueue <- status:
		d.logf("GIP: queued status message")
	default:
	}

	// Stay in Arrival state. Don't transition to Idle.
	// The device keeps sending Hello when the queue is empty,
	// until the host sends SetState START → Active.
}

// handleSetState processes power state changes from the host.
func (d *XboxGIP) handleSetState(stateCode byte, seq uint8) {
	d.logf("GIP: SetState code=0x%02X seq=%d", stateCode, seq)

	switch stateCode {
	case GIPStateStart: // 0x00 → Active
		d.logf("GIP: → Active (input reports enabled)")
		slog.Info("GIP: SetState START → Active")
		atomic.StoreInt32(&d.gipState, stateActive)

	case GIPStateStop: // 0x01 → Arrival
		d.logf("GIP: → Arrival (input reports paused)")
		slog.Info("GIP: SetState STOP → Arrival")
		atomic.StoreInt32(&d.gipState, stateArrival)

	case GIPStateOff: // 0x04 → Off
		d.logf("GIP: → Off (power down)")
		slog.Info("GIP: SetState OFF")
		status := buildStatusMessage(d.nextSeq())
		select {
		case d.respQueue <- status:
		default:
		}
		atomic.StoreInt32(&d.gipState, stateArrival)

	case GIPStateQuiesce: // 0x05 — Clear motors
		d.logf("GIP: Quiesce (clearing motors)")
		d.stateMu.Lock()
		fn := d.outputFunc
		d.stateMu.Unlock()
		if fn != nil {
			fn(OutputState{})
		}

	case GIPStateReset: // 0x07 — Full reset
		d.logf("GIP: → Reset (restarting)")
		slog.Info("GIP: SetState RESET → Arrival")
		atomic.StoreInt32(&d.gipState, stateArrival)
		atomic.StoreUint32(&d.seqDevice, 1)
		atomic.StoreInt32(&d.helloSent, 0)

	default:
		d.logf("GIP: unknown state code 0x%02X", stateCode)
	}
}

// handleRumble parses a GIP force feedback message (command 0x09).
func (d *XboxGIP) handleRumble(data []byte) {
	if len(data) < 13 {
		return
	}
	leftTrigger := data[6]
	rightTrigger := data[7]
	leftMotor := data[8]
	rightMotor := data[9]

	d.stateMu.Lock()
	fn := d.outputFunc
	d.stateMu.Unlock()

	if fn != nil {
		fn(OutputState{
			LeftMotor:    leftMotor,
			RightMotor:   rightMotor,
			LeftTrigger:  leftTrigger,
			RightTrigger: rightTrigger,
		})
	}
}

// HandleControl handles EP0 control transfers for MS OS descriptors.
func (d *XboxGIP) HandleControl(bmRequestType, bRequest uint8, wValue, wIndex, wLength uint16, data []byte) ([]byte, bool) {
	d.logf("EP0: bmReqType=0x%02X bReq=0x%02X wVal=0x%04X wIdx=0x%04X wLen=%d",
		bmRequestType, bRequest, wValue, wIndex, wLength)

	// Standard GET_STATUS(Device) — return 2 bytes: not self-powered, no remote wakeup.
	if bmRequestType == 0x80 && bRequest == 0x00 && wIndex == 0x0000 {
		d.logf("EP0: returning GET_STATUS (device)")
		return []byte{0x00, 0x00}, true
	}

	// Vendor request: Extended Compatible ID OS Descriptor.
	// Accept ANY vendor code — Windows caches the code per VID:PID, so the real
	// Xbox controller's cached code (0xFD) may differ from our string descriptor (0x90).
	if bmRequestType == 0xC0 && wIndex == 0x0004 {
		d.logf("EP0: returning Extended Compatible ID (XGIP10)")
		resp := extCompatIDDescriptor[:]
		if int(wLength) < len(resp) {
			resp = resp[:wLength]
		}
		return resp, true
	}

	return nil, false
}

func (d *XboxGIP) GetDescriptor() *usb.Descriptor {
	return &d.descriptor
}

func (d *XboxGIP) GetDeviceSpecificArgs() map[string]any {
	return nil
}

// makeDescriptor builds the USB descriptor for a GIP Xbox Series controller.
func makeDescriptor() usb.Descriptor {
	return usb.Descriptor{
		Device: usb.DeviceDescriptor{
			BcdUSB:             0x0200,
			BDeviceClass:       0xFF,
			BDeviceSubClass:    0x47,
			BDeviceProtocol:    0xD0,
			BMaxPacketSize0:    0x40,
			IDVendor:           0x045E,
			IDProduct:          0x0B12,
			BcdDevice:          0x0517,
			IManufacturer:      0x01,
			IProduct:           0x02,
			ISerialNumber:      0x03,
			BNumConfigurations: 0x01,
			Speed:              2, // Full-speed (12 Mbps) — MS-GIPUSB spec §2.2.1 requires FS; MUST STALL device_qualifier
		},
		Interfaces: []usb.InterfaceConfig{
			// Interface 0: GIP Primary — gamepad input/output
			{
				Descriptor: usb.InterfaceDescriptor{
					BInterfaceNumber:   0x00,
					BAlternateSetting:  0x00,
					BNumEndpoints:      0x02,
					BInterfaceClass:    0xFF,
					BInterfaceSubClass: 0x47,
					BInterfaceProtocol: 0xD0,
					IInterface:         0x00,
				},
				Endpoints: []usb.EndpointDescriptor{
					{BEndpointAddress: 0x81, BMAttributes: 0x03, WMaxPacketSize: 0x0040, BInterval: 0x04},
					{BEndpointAddress: 0x01, BMAttributes: 0x03, WMaxPacketSize: 0x0040, BInterval: 0x04},
				},
			},
			// Interface 1: GIP Secondary — auth/security channel
			{
				Descriptor: usb.InterfaceDescriptor{
					BInterfaceNumber:   0x01,
					BAlternateSetting:  0x00,
					BNumEndpoints:      0x02,
					BInterfaceClass:    0xFF,
					BInterfaceSubClass: 0x47,
					BInterfaceProtocol: 0xD0,
					IInterface:         0x00,
				},
				Endpoints: []usb.EndpointDescriptor{
					{BEndpointAddress: 0x82, BMAttributes: 0x03, WMaxPacketSize: 0x0040, BInterval: 0x04},
					{BEndpointAddress: 0x02, BMAttributes: 0x03, WMaxPacketSize: 0x0040, BInterval: 0x08},
				},
			},
			// Interface 2: GIP Audio — no endpoints in alt setting 0
			{
				Descriptor: usb.InterfaceDescriptor{
					BInterfaceNumber:   0x02,
					BAlternateSetting:  0x00,
					BNumEndpoints:      0x00,
					BInterfaceClass:    0xFF,
					BInterfaceSubClass: 0x47,
					BInterfaceProtocol: 0xD0,
					IInterface:         0x00,
				},
			},
		},
		Strings: map[uint8]string{
			0:    "\x04\x09",
			1:    "Microsoft",
			2:    "Xbox Wireless Controller",
			3:    "0000FFFB56495052",
			0xEE: "MSFT100\x90",
		},
	}
}

var (
	_ usb.Device        = (*XboxGIP)(nil)
	_ usb.ControlDevice = (*XboxGIP)(nil)
)

var _ = binary.LittleEndian
