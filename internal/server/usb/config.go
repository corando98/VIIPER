package usb

import "time"

// ServerConfig represents the server subcommand configuration.
type ServerConfig struct {
	Addr                    string        `help:"USB-IP server listen address" default:":3241" env:"VIIPER_USB_ADDR"`
	ConnectionTimeout       time.Duration `kong:"-"`
	BusCleanupTimeout       time.Duration `help:"-"`
	WriteBatchFlushInterval time.Duration `help:"Interval to flush write batches to clients; 0 to disable" default:"0" env:"VIIPER_USB_WRITE_BATCH_FLUSH_INTERVAL"`
	// HardwarePacedCompletions completes interrupt-IN URBs at each endpoint's
	// bInterval (like real USB hardware polls) instead of once per input
	// update. Input state is conflated latest-wins between completions. At
	// input rates above the poll rate this proportionally cuts TCP round
	// trips and kernel URB work; below the poll rate behavior is unchanged.
	HardwarePacedCompletions bool `help:"Pace interrupt-IN completions to the endpoint bInterval instead of per input update" default:"true" env:"VIIPER_HW_PACED"`
	// NakWhenIdle leaves interrupt-IN URBs pending until fresh input arrives
	// instead of replaying the last report at every bInterval (keepalive).
	// Real HID/XUSB hardware NAKs the endpoint when state is unchanged, so
	// this is the hardware-faithful mode: completions = min(input rate, poll
	// rate) and an idle device generates zero traffic and zero timer churn.
	NakWhenIdle bool `help:"NAK idle interrupt-IN endpoints instead of replaying keepalive reports" default:"false" env:"VIIPER_NAK_IDLE"`
}
