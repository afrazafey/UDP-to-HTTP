#!/usr/bin/env python3
"""Linux runtime acceptance test. Synthetic transport packets contain no video."""
import argparse
import concurrent.futures
import json
import os
import platform
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', required=True)
parser.add_argument('--report', default='build/linux-smoke-results.json')
args = parser.parse_args()
if platform.system() != 'Linux':
    parser.error('an actual Linux runtime is required; cross-compilation is not validation')
import resource
binary = Path(args.binary).resolve()

def free_port(kind):
    with socket.socket(socket.AF_INET, kind) as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]

tcp_ports = set()
while len(tcp_ports) < 3:
    tcp_ports.add(free_port(socket.SOCK_STREAM))
management, primary, secondary = sorted(tcp_ports)
udp_port = free_port(socket.SOCK_DGRAM)
base = f'http://127.0.0.1:{management}'

def api(path, method='GET', body=None, token=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['X-CSRF-Token'] = token
    request = urllib.request.Request(base + path, method=method, headers=headers,
                                    data=json.dumps(body).encode() if body is not None else None)
    with urllib.request.urlopen(request, timeout=5) as response:
        data = response.read()
        return json.loads(data) if data else None

def eventually(check, seconds=8):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            result = check()
            if result:
                return result
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.03)
    raise AssertionError('condition not met before deadline')

with tempfile.TemporaryDirectory(prefix='udphttp-linux-') as directory:
    directory = Path(directory)
    configuration = {
        'management': {'host': '127.0.0.1', 'port': management, 'shutdown_timeout': '3s'},
        'network': {'http_interface': '127.0.0.1', 'http_port': primary, 'secondary_http_port': secondary},
        'streaming': {'allowed_client_networks': ['127.0.0.0/8'], 'allow_loopback_unicast': True,
                      'allowed_unicast_sources': ['127.0.0.1'], 'connect_timeout': '2s', 'inactivity_timeout': '1s'},
        'storage': {'path': str(directory / 'state.db')},
        'channels': [{'id': 'smoke', 'name': 'Smoke stream', 'source_type': 'unicast', 'source_address': '127.0.0.1',
                      'source_port': udp_port, 'interface': '127.0.0.1', 'enabled': True, 'startup': 'always'}],
    }
    config_path = directory / 'config.yaml'
    config_path.write_text(json.dumps(configuration), encoding='utf-8')
    stop = threading.Event()
    sent = [0]
    def generate():
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.bind(('127.0.0.1', 0))
            cc = 0
            while not stop.is_set():
                data = bytearray()
                for _ in range(7):
                    packet = bytearray([0xff] * 188)
                    packet[:4] = bytes([0x47, 1, 0, 0x10 | (cc & 15)])
                    cc += 1
                    data.extend(packet)
                sock.sendto(data, ('127.0.0.1', udp_port))
                sent[0] += len(data)
                stop.wait(0.01)

    with (directory / 'service.log').open('w+') as logs:
        process = subprocess.Popen([str(binary), 'serve', '--config', str(config_path)], stdout=subprocess.DEVNULL, stderr=logs)
        generator = None
        try:
            eventually(lambda: api('/health')['storage_ready'])
            assert api('/api/v1/status')['phase'] == 2
            token = api('/api/v1/csrf')['token']
            generator = threading.Thread(target=generate)
            generator.start()
            eventually(lambda: api('/api/v1/streams/smoke')['stream']['state'] == 'running')
            def watch(port):
                with urllib.request.urlopen(f'http://127.0.0.1:{port}/live/smoke.ts', timeout=5) as response:
                    assert response.headers['Content-Type'] == 'video/mp2t'
                    data = response.read(188 * 14)
                    assert len(data) == 188 * 14 and all(data[i] == 0x47 for i in range(0, len(data), 188))
                    return len(data)
            with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
                results = list(executor.map(watch, [primary, secondary, primary, secondary]))
            eventually(lambda: not api('/api/v1/clients')['clients'])
            snapshot = api('/api/v1/streams/smoke')['stream']
            assert snapshot['counters']['input_bytes'] > 0 and snapshot['counters']['output_bytes'] >= sum(results)
            assert snapshot['counters']['input_bytes'] <= sent[0]
            for action in ['stop', 'start', 'restart']:
                api(f'/api/v1/streams/smoke/{action}', 'POST', token=token)
                if action == 'stop':
                    assert api('/api/v1/streams/smoke')['stream']['state'] == 'stopped'
                else:
                    eventually(lambda: api('/api/v1/streams/smoke')['stream']['state'] == 'running')
            extra = dict(configuration['channels'][0], id='persisted', name='Persistence check', source_port=free_port(socket.SOCK_DGRAM), enabled=False)
            api('/api/v1/streams', 'POST', extra, token)
            extra['name'] = 'Updated persistence check'
            api('/api/v1/streams/persisted', 'PUT', extra, token)
            process.send_signal(signal.SIGTERM)
            assert process.wait(timeout=5) == 0
            # A restart proves schema initialization and channel persistence rather than just an API echo.
            process = subprocess.Popen([str(binary), 'serve', '--config', str(config_path)], stdout=subprocess.DEVNULL, stderr=logs)
            eventually(lambda: api('/health')['storage_ready'])
            token = api('/api/v1/csrf')['token']
            assert api('/api/v1/streams/persisted')['stream']['channel']['name'] == extra['name']
            api('/api/v1/streams/persisted', 'DELETE', token=token)
            process.send_signal(signal.SIGTERM)
            assert process.wait(timeout=5) == 0
            usage = resource.getrusage(resource.RUSAGE_CHILDREN)
            report = {'result': 'passed', 'platform': platform.platform(), 'machine': platform.machine(),
                      'viewers': len(results), 'playback_listeners': 2, 'payload_bytes_read': sum(results),
                      'stream_counters': snapshot['counters'], 'max_rss_kib': usage.ru_maxrss,
                      'child_user_cpu_seconds': usage.ru_utime, 'child_system_cpu_seconds': usage.ru_stime}
            target = Path(args.report)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
            print(json.dumps(report, indent=2))
        finally:
            stop.set()
            if generator:
                generator.join(timeout=2)
            if process.poll() is None:
                process.send_signal(signal.SIGTERM)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            logs.seek(0)
            print(logs.read())
