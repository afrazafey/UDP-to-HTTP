#!/usr/bin/env python3
"""Build a standard ar-format Debian package without requiring a Linux build host.
dpkg-deb inspection and install/upgrade/remove tests still run on Linux CI.
"""
import gzip
import hashlib
import io
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile

root=Path(__file__).resolve().parents[1]
def tar_bytes(files,epoch,include_directories=False):
    result=io.BytesIO()
    with gzip.GzipFile(fileobj=result,mode='wb',filename='',mtime=epoch) as compressed:
        with tarfile.open(fileobj=compressed,mode='w') as archive:
            if include_directories:
                parents = {Path(name).parent for name in files}
                directories = {ancestor.as_posix() for parent in parents for ancestor in (parent, *parent.parents)}
                for name in sorted(directories, key=lambda path: (path.count('/'), path)):
                    entry = tarfile.TarInfo('.' if name == '.' else './' + name)
                    entry.type = tarfile.DIRTYPE
                    entry.mode = 0o755
                    entry.uname = entry.gname = 'root'
                    entry.mtime = epoch
                    archive.addfile(entry)
            for name,(data,mode) in sorted(files.items()):
                entry=tarfile.TarInfo('./'+name);entry.size=len(data);entry.mode=mode;entry.uid=entry.gid=0;entry.uname=entry.gname='root';entry.mtime=epoch
                archive.addfile(entry,io.BytesIO(data))
    return result.getvalue()
def build(arch,version,binary):
    if arch not in ('amd64','arm64') or not re.fullmatch(r'[0-9][0-9A-Za-z.+:~_-]*',version):raise ValueError('invalid architecture or version')
    epoch=int(os.environ.get('SOURCE_DATE_EPOCH','0'))
    data={'usr/bin/udphttp-proxy':(Path(binary).read_bytes(),0o755),'etc/udphttp-proxy/config.yaml':((root/'deploy/config.yaml').read_bytes(),0o644),'usr/lib/systemd/system/udphttp-proxy.service':((root/'deploy/udphttp-proxy.service').read_bytes(),0o644)}
    for path in [*[root/name for name in ('README.md','CHANGELOG.md','ARCHITECTURE.md','ROADMAP.md','LINUX_TESTING.md','PHASE1_REPORT.md','PHASE2_REPORT.md','PHASE3_AUDIT.md','PHASE3_REPORT.md')],*sorted((root/'docs').glob('*.md')),*sorted((root/'docs/screenshots').glob('*.png')),*sorted((root/'deploy').glob('*.md')),root/'deploy/nginx-management.conf']:
        if path.exists():
            relative=path.relative_to(root).as_posix();data['usr/share/doc/udphttp-proxy/'+relative]=(path.read_bytes(),0o644)
    for name in ('backup.sh','restore.sh'):
        data['usr/share/doc/udphttp-proxy/scripts/'+name]=((root/'scripts'/name).read_bytes(),0o755)
    control={'control':(f'Package: udphttp-proxy\nVersion: {version}\nArchitecture: {arch}\nSection: net\nPriority: optional\nMaintainer: UDPHTTP Proxy contributors <maintainers@example.invalid>\nDepends: adduser, init-system-helpers\nDescription: Shared UDP MPEG-TS to HTTP streaming proxy\n Phase 4 validation candidate; inspect docs/PHASE4_REPORT.md before rollout.\n'.encode(),0o644),'conffiles':(b'/etc/udphttp-proxy/config.yaml\n',0o644)}
    control['md5sums']=(''.join(f'{hashlib.md5(content).hexdigest()}  {name}\n' for name,(content,_) in sorted(data.items())).encode(),0o644)
    for name in ('postinst','prerm','postrm'):control[name]=((root/'packaging/debian'/name).read_bytes().replace(b'\r\n',b'\n'),0o755)
    output=root/'build'/f'udphttp-proxy_{version}_{arch}.deb';output.parent.mkdir(exist_ok=True)
    if dpkg := shutil.which('dpkg-deb'):
        with tempfile.TemporaryDirectory(prefix='udphttp-deb-') as directory:
            stage = Path(directory) / 'package'
            for files, prefix in ((data, stage), (control, stage / 'DEBIAN')):
                for name, (content, mode) in files.items():
                    path = prefix / name
                    path.parent.mkdir(mode=0o755, parents=True, exist_ok=True)
                    path.write_bytes(content)
                    path.chmod(mode)
            for path in [stage, *stage.rglob('*')]:
                if path.is_dir():
                    path.chmod(0o755)
            subprocess.run([dpkg, '--root-owner-group', '-Zgzip', '--build', str(stage), str(output)],
                           env=dict(os.environ, SOURCE_DATE_EPOCH=str(epoch)), check=True)
        return output
    with output.open('wb') as archive:
        archive.write(b'!<arch>\n')
        for name,content in [('debian-binary',b'2.0\n'),('control.tar.gz',tar_bytes(control,epoch)),('data.tar.gz',tar_bytes(data,epoch,include_directories=True))]:
            archive.write(f'{name+"/":<16}{epoch:<12}{0:<6}{0:<6}{"100644":<8}{len(content):<10}`\n'.encode());archive.write(content)
            if len(content)%2:archive.write(b'\n')
    return output
if __name__=='__main__':print(build(*sys.argv[1:]))
