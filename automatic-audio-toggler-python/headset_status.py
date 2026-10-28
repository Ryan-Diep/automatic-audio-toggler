"""Read the BlackShark V2 HyperSpeed receiver's RF link, not USB presence.

Protocol: https://github.com/bbesli/Synapse-for-Linux/blob/main/docs/PROTOCOL.md
Only VID/PID 1532:0565 is enabled; other Razer models use different protocols.
"""

import time

POLL_INTERVAL = 2.0
REPLY_TIMEOUT = 1.0


def supports_link_query(vendor_id, product_id):
    return (vendor_id, product_id) == (0x1532, 0x0565)


def link_request(sequence):
    report = bytearray(64)
    report[0] = 0x02
    report[2] = 0x60 | (sequence & 0x1F)
    report[6] = 4
    # Domain 0 addresses the receiver, which can reply when the headset is off.
    report[10] = 0x20
    for value in report[:62]:
        report[62] ^= value
    return bytes(report)


def parse_link_reply(data, sequence):
    """Return True/False for a valid matching reply, None for unrelated data."""
    if len(data) != 64:
        return None
    if (data[0] != 2 or data[1] != 2 or
            data[2] & 0x1F != sequence & 0x1F or
            data[6] != 5 or data[8] != 0x80 or data[9] != 0 or
            data[10] != 0x20 or data[11] != 1 or data[12] != 1 or
            data[13] not in (0, 1)):
        return None
    checksum = 0
    for value in data[1:62]:
        checksum ^= value
    if checksum != data[62]:
        return None
    return bool(data[13])


def query_link(device, sequence, clock=time.monotonic, sleep=time.sleep):
    """Query a nonblocking HID handle. A timeout is unknown, never 'off'."""
    request = link_request(sequence)
    if device.write(request) != len(request):
        raise OSError("Incomplete headset status request")
    deadline = clock() + REPLY_TIMEOUT
    while clock() < deadline:
        state = parse_link_reply(device.read(64), sequence)
        if state is not None:
            return state
        sleep(0.01)
    raise TimeoutError("Receiver did not answer the headset link query")
