#!/usr/bin/env python3
"""Build reviewable local release candidates; never publishes or deploys."""
import argparse
import datetime
import gzip
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
import zipfile

root=Path(__file__).resolve().parents[1]
def run(args,**kwargs):return subprocess.run(args,cwd=root,check=True,**kwargs)
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--version',default='0.4.2-rc.1')
parser.add_argument('--go',default='go')
parser.add_argument('--targets',default='linux/amd64,linux/arm64,windows/amd64')
parser.add_argument('--output',type=Path,default=root/'build')
parser.add_argument('--frontend-dir',type=Path,default=root/'frontend')
parser.add_argument('--evidence',type=Path)
parser.add_argument('--extra-source',action='append',default=[])
parser.add_argument('--without-windows-resources',action='store_true',help='Local candidate only when the PE resource tool cannot run')
args=parser.parse_args()
if not re.fullmatch(r'[0-9][0-9A-Za-z.+-]*',args.version):parser.error('invalid version')
if args.without_windows_resources and '-rc.' not in args.version and '-dev' not in args.version:
    parser.error('Windows resources are required for stable builds')
if not (root/'internal/web/dist/index.html').is_file():
    parser.error('build the production dashboard first')
sha=run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip()
dirty=bool(run(['git','status','--porcelain'],capture_output=True,text=True).stdout.strip())
now=datetime.datetime.now(datetime.timezone.utc)
epoch=int(os.environ.get('SOURCE_DATE_EPOCH',str(int(now.timestamp()))))
date=now.strftime('%Y-%m-%dT%H:%M:%SZ')
commit=sha+('+dirty' if dirty else '')
ldflags=f'-s -w -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Version={args.version} -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Commit={commit} -X github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo.Date={date}'
build=args.output.resolve();build.mkdir(parents=True,exist_ok=True);outputs=[]
def source_file(name):
    relative=PurePosixPath(name)
    if relative.is_absolute() or '..' in relative.parts or '\\' in name or ':' in name or name.startswith(('.git/','.tools/','data/','build/','frontend/node_modules/')):
        raise ValueError(f'unsafe source archive path: {name}')
    path=root/name
    if not stat.S_ISREG(path.lstat().st_mode) or path.resolve()!=path:
        raise ValueError(f'source archive requires a regular checkout file: {name}')
    return path

def windows_resources(binary, version):
    numbers=re.match(r'^(\d+)\.(\d+)\.(\d+)', version)
    if not numbers:
        raise ValueError('Windows resources require a three-part version')
    numeric='.'.join([*numbers.groups(),'0'])
    descriptor={'RT_VERSION':{'#1':{'0000':{
        'fixed': {'file_version':numeric,'product_version':numeric},
        'info': {'0409': {'CompanyName':'Afraaz','FileDescription':'UDPHTTP Proxy','ProductName':'UDPHTTP Proxy',
                           'FileVersion':version,'ProductVersion':version,'OriginalFilename':'udphttp-proxy.exe',
                           'Comments':'Developed by Afraaz — https://afraaz.xyz'}}
    }}}}
    with tempfile.TemporaryDirectory(prefix='udphttp-winres-') as directory:
        resource=Path(directory)/'winres.json'
        resource.write_text(json.dumps(descriptor),encoding='utf-8')
        run([args.go,'run','github.com/tc-hib/go-winres@v0.3.3','patch','--no-backup','--in',str(resource),str(binary)])
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
    directory=args.frontend_dir.resolve()/'node_modules'/package
    for filename in ('LICENSE','LICENSE.txt','LICENSE.md'):
        path=directory/filename
        if path.is_file():notices[f'third_party/npm/{package}/{filename}']=path.read_bytes()
docs=[*[root/name for name in ('README.md','CHANGELOG.md','SECURITY.md','ARCHITECTURE.md','ROADMAP.md','LINUX_TESTING.md','PHASE1_REPORT.md','PHASE2_REPORT.md','PHASE3_AUDIT.md','PHASE3_REPORT.md')],*sorted((root/'docs').glob('*.md')),*sorted((root/'configs').glob('*.yaml')),*sorted((root/'deploy').glob('*')),*sorted((root/'scripts').glob('*.ps1')),*sorted((root/'scripts').glob('*.sh'))]
tracked=run(['git','ls-files','--cached','-z'],capture_output=True,text=True).stdout.split('\0')
source_paths=sorted(set(filter(None,[*tracked,*args.extra_source])))
source_inputs={name: source_file(name) for name in source_paths}
evidence={}
if args.evidence:
    for name in ('go-sbom.json','frontend-sbom.json','govulncheck.txt','npm-audit.json','npm-audit-production.json'):
        path=args.evidence/name
        evidence[name]=path.read_bytes()
for target in args.targets.split(','):
    if target not in ('linux/amd64','linux/arm64','windows/amd64'):parser.error('unsupported target')
    system,arch=target.split('/');name=f'udphttp-proxy-{system}-{arch}'+('.exe' if system=='windows' else '')
    binary=build/name;env=dict(os.environ,GOOS=system,GOARCH=arch,CGO_ENABLED='0');run([args.go,'build','-trimpath','-ldflags',ldflags,'-o',str(binary),'./cmd/udphttp-proxy'],env=env)
    with_resources=system=='windows' and not args.without_windows_resources
    if with_resources:
        windows_resources(binary,args.version)
    entries={'udphttp-proxy'+('.exe' if system=='windows' else ''):binary.read_bytes()}
    entries.update(notices)
    for path in [*docs, *(root/'docs/screenshots').glob('*.png')]:
        relative=path.relative_to(root).as_posix()
        if relative in source_inputs:
            entries[relative]=source_inputs[relative].read_bytes()
    entries.update({f'security/{name}':content for name,content in evidence.items()})
    entries['BUILD.json']=json.dumps({'application':'UDPHTTP Proxy','developer':'Afraaz','website':'https://afraaz.xyz','version':args.version,'source_commit':sha,'dirty':dirty,'built':date,'target':target,'windows_version_resource':with_resources,'production_acceptance':'pending'},indent=2).encode()
    if system=='windows':
        archive=build/f'udphttp-proxy_{args.version}_{system}_{arch}.zip'
        with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED) as z:
            for path,content in sorted(entries.items()):info=zipfile.ZipInfo(path,date_time=datetime.datetime.fromtimestamp(max(epoch,315532800),datetime.timezone.utc).timetuple()[:6]);info.compress_type=zipfile.ZIP_DEFLATED;info.external_attr=0o644<<16;z.writestr(info,content)
    else:
        archive=build/f'udphttp-proxy_{args.version}_{system}_{arch}.tar.gz'
        with archive.open('wb') as raw,gzip.GzipFile(fileobj=raw,filename='',mode='wb',mtime=epoch) as compressed,tarfile.open(fileobj=compressed,mode='w') as tar:
            for path,content in sorted(entries.items()):info=tarfile.TarInfo(path);info.size=len(content);info.mtime=epoch;info.mode=0o755 if path=='udphttp-proxy' or path.endswith('.sh') else 0o644;tar.addfile(info,io.BytesIO(content))
        debversion=args.version.replace('-dev','~dev').replace('-rc','~rc')
        env=dict(os.environ,SOURCE_DATE_EPOCH=str(epoch));run([sys.executable,str(root/'scripts/build-deb.py'),arch,debversion,str(binary),str(build)],env=env);outputs.append(build/f'udphttp-proxy_{debversion}_{arch}.deb')
    outputs.extend([binary,archive])
source=build/f'udphttp-proxy_{args.version}_source.zip'
with zipfile.ZipFile(source,'w',zipfile.ZIP_DEFLATED) as archive:
    for name,path in source_inputs.items():
        info=zipfile.ZipInfo(name,date_time=datetime.datetime.fromtimestamp(max(epoch,315532800),datetime.timezone.utc).timetuple()[:6]);info.compress_type=zipfile.ZIP_DEFLATED;archive.writestr(info,path.read_bytes())
outputs.append(source)
for name,content in evidence.items():
    path=build/name
    path.write_bytes(content)
    outputs.append(path)
manifest=build/'SHA256SUMS';manifest.write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in outputs),encoding='utf-8',newline='\n');print(manifest)
