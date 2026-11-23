#!/usr/bin/env python3
"""Send synthetic MPEG-TS packets locally; no decodable video or provider required."""
import argparse
import ipaddress
import socket
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--host', default='127.0.0.1')
parser.add_argument('--port', type=int, default=15000)
parser.add_argument('--bind', default='127.0.0.1')
parser.add_argument('--seconds', type=float, default=30)
parser.add_argument('--interval', type=float, default=0.01)
args = parser.parse_args()
ipaddress.IPv4Address(args.host)
ipaddress.IPv4Address(args.bind)
if not 1 <= args.port <= 65535 or args.seconds <= 0 or args.interval <= 0:
    parser.error('port, seconds and interval must be positive and in range')
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind((args.bind, 0))
if ipaddress.IPv4Address(args.host).is_multicast:
    sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_IF, socket.inet_aton(args.bind))
    sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 1)
    sock.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_LOOP, 1)
end = time.monotonic() + args.seconds
counter = 0
try:
    while time.monotonic() < end:
        payload = bytearray()
        for _ in range(7):
            packet = bytearray([0xff] * 188)
            packet[:4] = bytes([0x47, 0x01, 0x00, 0x10 | (counter & 15)])
            counter += 1
            payload.extend(packet)
        sock.sendto(payload, (args.host, args.port))
        time.sleep(args.interval)
except KeyboardInterrupt:
    pass
finally:
    sock.close()
print(f'Sent {counter} synthetic TS packets ({counter * 188} payload bytes)')
