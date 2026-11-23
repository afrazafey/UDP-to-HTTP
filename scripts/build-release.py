#!/usr/bin/env python3
"""Build reviewable local release candidates; never publishes or deploys."""
import argparse
import datetime
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import zipfile

root=Path(__file__).resolve().parents[1]
def run(args,**kwargs):return subprocess.run(args,cwd=root,check=True,**kwargs)
parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--version',default='0.4.0-dev');parser.add_argument('--go',default='go');parser.add_argument('--targets',default='linux/amd64,linux/arm64,windows/amd64');args=parser.parse_args()
if not re.fullmatch(r'[0-9][0-9A-Za-z.+-]*',args.version):parser.error('invalid version')
sha=run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip()
dirty=bool(run(['git','status','--porcelain'],capture_output=True,text=True).stdout.strip())
epoch=int(os.environ.get('SOURCE_DATE_EPOCH',str(int(datetime.datetime.now(datetime.timezone.utc).timestamp()))));date=datetime.datetime.fromtimestamp(epoch,datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
commit=sha+('+dirty' if dirty else '')
ldflags=f'-s -w -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Version={args.version} -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Commit={commit} -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Date={date}'
build=root/'build';build.mkdir(exist_ok=True);outputs=[]
# Carry actual dependency notices without declaring a license for this project.
notices={'PROJECT-LICENSE-STATUS.txt':b'No repository license is declared. Resolve distribution rights with the owner before publication. Dependencies retain their own licenses.\n'}
modules=run([args.go,'list','-m','-json','all'],capture_output=True,text=True).stdout
decoder=json.JSONDecoder();remaining=modules
while remaining.strip():
    module,end=decoder.raw_decode(remaining.lstrip());remaining=remaining.lstrip()[end:]
    directory=module.get('Dir')
    if directory and not module.get('Main'):
        for filename in ('LICENSE','LICENSE.txt','LICENSE.md','COPYING','NOTICE'):
            path=Path(directory)/filename
            if path.is_file():notices[f'third_party/go/{module["Path"]}/{filename}']=path.read_bytes()
for package in ('react','react-dom','scheduler','lucide-react'):
    directory=root/'frontend/node_modules'/package
    for filename in ('LICENSE','LICENSE.txt','LICENSE.md'):
        path=directory/filename
        if path.is_file():notices[f'third_party/npm/{package}/{filename}']=path.read_bytes()
docs=[*[root/name for name in ('README.md','CHANGELOG.md','ARCHITECTURE.md','ROADMAP.md','LINUX_TESTING.md','PHASE1_REPORT.md','PHASE2_REPORT.md','PHASE3_AUDIT.md','PHASE3_REPORT.md')],*sorted((root/'docs').glob('*.md')),*sorted((root/'configs').glob('*.yaml')),*sorted((root/'deploy').glob('*')),*sorted((root/'scripts').glob('*.ps1')),*sorted((root/'scripts').glob('*.sh'))]
for target in args.targets.split(','):
    if target not in ('linux/amd64','linux/arm64','windows/amd64'):parser.error('unsupported target')
    system,arch=target.split('/');name=f'udphttp-proxy-{system}-{arch}'+('.exe' if system=='windows' else '')
    binary=build/name;env=dict(os.environ,GOOS=system,GOARCH=arch,CGO_ENABLED='0');run([args.go,'build','-trimpath','-ldflags',ldflags,'-o',str(binary),'./cmd/udphttp-proxy'],env=env)
    entries={'udphttp-proxy'+('.exe' if system=='windows' else ''):binary.read_bytes()}
    entries.update(notices)
    for path in docs:
        if path.is_file():entries[path.relative_to(root).as_posix()]=path.read_bytes()
    entries['BUILD.json']=json.dumps({'version':args.version,'source_commit':sha,'dirty':dirty,'built':date,'target':target,'production_acceptance':'pending'},indent=2).encode()
    if system=='windows':
        archive=build/f'udphttp-proxy_{args.version}_{system}_{arch}.zip'
        with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
            for path,content in sorted(entries.items()):info=zipfile.ZipInfo(path,date_time=datetime.datetime.fromtimestamp(max(epoch,315532800),datetime.timezone.utc).timetuple()[:6]);info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=0o644<<16;z.writestr(info,content)
    else:
        archive=build/f'udphttp-proxy_{args.version}_{system}_{arch}.tar.gz'
        with archive.open('wb') as raw,gzip.GzipFile(fileobj=raw,filename='',mode='wb',mtime=epoch) as compressed,tarfile.open(fileobj=compressed,mode='w') as tar:
            for path,content in sorted(entries.items()):info=tarfile.TarInfo(path);info.size=len(content);info.mtime=epoch;info.mode=0o755 if path=='udphttp-proxy' or path.endswith('.sh') else 0o644;tar.addfile(info,io.BytesIO(content))
        debversion=args.version.replace('-dev','~dev').replace('-rc','~rc')
        env=dict(os.environ,SOURCE_DATE_EPOCH=str(epoch));run([sys.executable,str(root/'scripts/build-deb.py'),arch,debversion,str(binary)],env=env);outputs.append(build/f'udphttp-proxy_{debversion}_{arch}.deb')
    outputs.extend([binary,archive])
source=build/f'udphttp-proxy_{args.version}_source.zip'
files=run(['git','ls-files','--cached','--others','--exclude-standard','-z'],capture_output=True,text=True).stdout.split('\0')
with zipfile.ZipFile(source,'w',zipfile.ZIP_DEFLATED) as archive:
    for name in sorted(set(files)):
        if name and (root/name).is_file():
            info=zipfile.ZipInfo(name,date_time=datetime.datetime.fromtimestamp(max(epoch,315532800),datetime.timezone.utc).timetuple()[:6]);info.compress_type=zipfile.ZIP_DEFLATED;archive.writestr(info,(root/name).read_bytes())
outputs.append(source)
manifest=build/'SHA256SUMS-phase4';manifest.write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in outputs),encoding='utf-8');print(manifest)
