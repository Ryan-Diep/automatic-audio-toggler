package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

// Protocol: https://github.com/bbesli/Synapse-for-Linux/blob/main/docs/PROTOCOL.md
// Other Razer models use different protocols; enable only the verified receiver.
func supportsLinkQuery(vendorID, productID uint16) bool {
	return vendorID == 0x1532 && productID == 0x0565
}

const linkPollInterval = 250 * time.Millisecond
const linkReplyTimeout = 150 * time.Millisecond

func linkRequest(sequence byte) []byte {
	report := make([]byte, 64)
	report[0] = 0x02
	report[2] = 0x60 | (sequence & 0x1f)
	report[6] = 4
	// Domain 0 addresses the receiver, which replies even with the headset off.
	report[10] = 0x20
	for _, value := range report[:62] {
		report[62] ^= value
	}
	return report
}

func parseLinkReply(data []byte, sequence byte) (bool, bool) {
	if len(data) != 64 {
		return false, false
	}
	if data[0] != 2 || data[1] != 2 || data[2]&0x1f != sequence&0x1f ||
		data[6] != 5 || data[8] != 0x80 || data[9] != 0 ||
		data[10] != 0x20 || data[11] != 1 || data[12] != 1 || data[13] > 1 {
		return false, false
	}
	var checksum byte
	for _, value := range data[1:62] {
		checksum ^= value
	}
	if checksum != data[62] {
		return false, false
	}
	return data[13] == 1, true
}

type statusDevice interface {
	Write([]byte) (int, error)
	ReadWithTimeout([]byte, time.Duration) (int, error)
}

func queryLink(device statusDevice, sequence byte) (bool, error) {
	request := linkRequest(sequence)
	n, err := device.Write(request)
	if err != nil {
		return false, err
	}
	if n != len(request) {
		return false, fmt.Errorf("incomplete headset status request")
	}
	deadline := time.Now().Add(linkReplyTimeout)
	buf := make([]byte, 64)
	for remaining := time.Until(deadline); remaining > 0; remaining = time.Until(deadline) {
		// HIDAPI truncates to milliseconds; round up to avoid a zero-timeout spin.
		timeout := (remaining + time.Millisecond - 1) / time.Millisecond * time.Millisecond
		n, err := device.ReadWithTimeout(buf, timeout)
		if errors.Is(err, hid.ErrTimeout) || (err == nil && n == 0) {
			break
		}
		if err != nil {
			return false, err
		}
		if state, valid := parseLinkReply(buf[:n], sequence); valid {
			return state, nil
		}
	}
	// A missing/invalid response is unknown, never evidence of a disconnected link.
	return false, fmt.Errorf("receiver did not answer the headset link query within %s: %w", linkReplyTimeout, hid.ErrTimeout)
}
