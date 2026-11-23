#!/usr/bin/env python3
"""Read-only checksum/container/architecture inspection, never native acceptance."""
import argparse
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import tarfile
import zipfile

p=argparse.ArgumentParser(description=__doc__);p.add_argument('--directory',default='build');args=p.parse_args();root=Path(args.directory).resolve()
def safe(name):
    path=PurePosixPath(name)
    assert not path.is_absolute() and '..' not in path.parts and '\\' not in name and ':' not in name, f'unsafe archive path {name}'
def elf(data,arch):
    assert data[:4]==b'\x7fELF' and data[4]==2 and data[5]==1
    assert int.from_bytes(data[18:20],'little')=={'amd64':62,'arm64':183}[arch]
def windows(data):
    assert data[:2]==b'MZ';offset=int.from_bytes(data[60:64],'little');assert data[offset:offset+4]==b'PE\0\0';assert int.from_bytes(data[offset+4:offset+6],'little')==0x8664
def metadata(data,target,binary):
    obj=json.loads(data);assert obj['target']==target;assert re.fullmatch('[0-9a-f]{40}',obj['source_commit']);assert re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ',obj['built']);assert isinstance(obj['dirty'],bool)
    for value in (obj['version'],obj['built'],obj['source_commit']+('+dirty' if obj['dirty'] else '')):assert value.encode() in binary, f'missing linked metadata {value}'
report={'checksums_verified':0,'linux_bundles':0,'windows_bundles':0,'source_archives':0,'debian_packages':0,'native_dpkg_installation':'not performed by static inspection','native_runtime':'not established by static inspection'}
for line in (root/'SHA256SUMS-phase4').read_text().splitlines():
    expected,name=line.split('  ',1);assert Path(name).name==name;path=root/name;assert path.is_file();assert hashlib.sha256(path.read_bytes()).hexdigest()==expected,f'checksum mismatch {name}';report['checksums_verified']+=1
    if name.endswith('.tar.gz'):
        arch='arm64' if 'arm64' in name else 'amd64'
        with tarfile.open(path,'r:gz') as tar:
            for member in tar.getmembers():safe(member.name);assert member.isfile() or member.isdir()
            binary=tar.extractfile('udphttp-proxy').read();elf(binary,arch);metadata(tar.extractfile('BUILD.json').read(),'linux/'+arch,binary);assert 'docs/PHASE4_REPORT.md' in tar.getnames();assert tar.getmember('udphttp-proxy').mode&0o111
        report['linux_bundles']+=1
    elif name.endswith('_windows_amd64.zip'):
        with zipfile.ZipFile(path) as archive:
            for member in archive.namelist():safe(member)
            binary=archive.read('udphttp-proxy.exe');windows(binary);metadata(archive.read('BUILD.json'),'windows/amd64',binary);assert 'scripts/install-windows.ps1' in archive.namelist();assert 'docs/PHASE4_REPORT.md' in archive.namelist()
        report['windows_bundles']+=1
    elif name.endswith('_source.zip'):
        with zipfile.ZipFile(path) as archive:
            for member in archive.namelist():
                safe(member);assert not member.startswith(('.git/','.tools/','data/','build/','frontend/node_modules/'))
            assert 'go.mod' in archive.namelist() and 'docs/PHASE4_REPORT.md' in archive.namelist()
        report['source_archives']+=1
    elif name.endswith('.deb'):
        data=path.read_bytes();assert data.startswith(b'!<arch>\n');offset=8;members={}
        while offset<len(data):
            header=data[offset:offset+60];assert len(header)==60 and header[58:]==b'`\n';member=header[:16].decode().strip().rstrip('/');size=int(header[48:58]);offset+=60;members[member]=data[offset:offset+size];offset+=size+size%2
        assert members['debian-binary']==b'2.0\n';arch='arm64' if 'arm64' in name else 'amd64'
        with tarfile.open(fileobj=io.BytesIO(members['control.tar.gz']),mode='r:gz') as control:
            text=control.extractfile('control').read().decode();assert f'Architecture: {arch}\n' in text;assert 'Package: udphttp-proxy\n' in text
            assert control.extractfile('conffiles').read()==b'/etc/udphttp-proxy/config.yaml\n'
            for hook in ('postinst','prerm','postrm'):assert control.getmember(hook).mode&0o111 and b'\r\n' not in control.extractfile(hook).read()
        with tarfile.open(fileobj=io.BytesIO(members['data.tar.gz']),mode='r:gz') as payload:
            for member in payload.getmembers():safe(member.name)
            elf(payload.extractfile('usr/bin/udphttp-proxy').read(),arch)
            assert 'etc/udphttp-proxy/config.yaml' in payload.getnames();assert 'usr/share/doc/udphttp-proxy/docs/PHASE4_REPORT.md' in payload.getnames()
            unit=payload.extractfile('usr/lib/systemd/system/udphttp-proxy.service').read();assert b'User=udphttp-proxy' in unit
        report['debian_packages']+=1
    elif name.endswith('.exe'):windows(path.read_bytes())
    elif '-linux-' in name:elf(path.read_bytes(),'arm64' if name.endswith('arm64') else 'amd64')
report['status']='passed static inspection'
(root/'phase4-artifact-inspection.json').write_text(json.dumps(report,indent=2));print(json.dumps(report,indent=2))
