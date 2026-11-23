#!/usr/bin/env python3
"""Measured synthetic UDP -> actual HTTP fan-out; no capacity/video-decoding claims."""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import queue
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.request

p=argparse.ArgumentParser(description=__doc__);p.add_argument('--binary',required=True);p.add_argument('--duration',type=float,default=6);p.add_argument('--viewers',default='1,10,25,50,100');p.add_argument('--report',default='build/phase4-load-results.json');args=p.parse_args()
if args.duration<3:p.error('duration must cover at least one 3-second resource sample')
counts=[int(n) for n in args.viewers.split(',')]
if not counts or min(counts)<1 or max(counts)>100:p.error('viewer counts must be 1..100')
report={'platform':os.name,'synthetic_mpeg_ts':True,'viewer_results':[],'cpu_sample_interval_seconds':3,'resource_scope':'whole daemon including monitoring and log persistence','limitations':['local UDP sender and local HTTP readers; no real codec payload','CPU/RSS are measured process observations, not capacity guarantees']}
def port(kind):
    with socket.socket(socket.AF_INET,kind) as s:s.bind(('127.0.0.1',0));return s.getsockname()[1]
ports=set()
while len(ports)<3:ports.add(port(socket.SOCK_STREAM))
management,primary,secondary=sorted(ports);udp=port(socket.SOCK_DGRAM);base=f'http://127.0.0.1:{management}'
def api(path,method='GET',body=None,token=None):
    headers={'Content-Type':'application/json'}
    if token:headers['X-CSRF-Token']=token
    request=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method=method)
    with urllib.request.urlopen(request,timeout=8) as r:data=r.read();return json.loads(data) if data else None
def wait(check,seconds=12):
    until=time.monotonic()+seconds
    while time.monotonic()<until:
        try:
            result=check()
            if result:return result
        except (OSError,ValueError):pass
        time.sleep(.03)
    raise AssertionError('condition did not become true before deadline')
proc=None
try:
 with tempfile.TemporaryDirectory(prefix='udphttp-load-') as directory:
    root=Path(directory);config={'management':{'port':management,'shutdown_timeout':'5s'},'network':{'http_interface':'127.0.0.1','http_port':primary,'secondary_http_port':secondary},'streaming':{'allow_loopback_unicast':True,'allowed_unicast_sources':['127.0.0.1'],'allowed_client_networks':['127.0.0.0/8'],'connect_timeout':'5s','max_clients':100},'storage':{'path':str(root/'state.db')},'channels':[{'id':'load','name':'Synthetic load','source_type':'unicast','source_address':'127.0.0.1','source_port':udp,'interface':'127.0.0.1','enabled':True,'idle_timeout_seconds':1,'startup':'on_demand'}]}
    (root/'config.yaml').write_text(json.dumps(config),encoding='utf-8');stop_source=threading.Event()
    with (root/'daemon.jsonl').open('wb') as logs:
      options={'creationflags':subprocess.CREATE_NEW_PROCESS_GROUP} if os.name=='nt' else {}
      proc=subprocess.Popen([str(Path(args.binary).resolve()),'serve','--config',str(root/'config.yaml')],stdout=subprocess.DEVNULL,stderr=logs,**options)
      def generate():
        with socket.socket(socket.AF_INET,socket.SOCK_DGRAM) as sender:
          sender.bind(('127.0.0.1',0));cc=0
          while not stop_source.wait(.01):
            data=bytearray()
            for _ in range(7):data.extend(bytes([0x47,1,0,0x10|(cc&15)])+bytes([255])*184);cc+=1
            sender.sendto(data,('127.0.0.1',udp))
      source=threading.Thread(target=generate,daemon=True);source.start()
      try:
        wait(lambda:api('/health')['storage_ready']);token=api('/api/v1/csrf')['token']
        for count in counts:
          ready=queue.Queue();start=threading.Event();stop=threading.Event()
          def reader():
            delivered=0;errors=0
            with urllib.request.urlopen(f'http://127.0.0.1:{primary}/live/load.ts',timeout=10) as response:
              if response.headers.get_content_type()!='video/mp2t':raise AssertionError('invalid playback MIME')
              ready.put(True);start.wait(12)
              while not stop.is_set():
                data=response.read(1316)
                if not data:break
                delivered+=len(data)
                if len(data)%188 or any(data[i]!=0x47 for i in range(0,len(data),188)):errors+=1
            return delivered,errors
          with concurrent.futures.ThreadPoolExecutor(max_workers=count) as pool:
            futures=[pool.submit(reader) for _ in range(count)]
            for _ in range(count):ready.get(timeout=12)
            wait(lambda:api('/api/v1/streams')['streams'][0]['clients']==count)
            before=api('/api/v1/streams')['streams'][0]['counters']['output_bytes'];started=time.monotonic();start.set();samples=[]
            while time.monotonic()-started<args.duration:samples.append(api('/api/v1/metrics')['sample']);time.sleep(.5)
            stop.set();delivered=[f.result(timeout=12) for f in futures];elapsed=time.monotonic()-started;current=api('/api/v1/streams')['streams'][0];metrics=api('/api/v1/metrics')
          wait(lambda:len(api('/api/v1/clients')['clients'])==0)
          if any(error for _,error in delivered):raise AssertionError('TS packet integrity failed')
          if metrics['sample']['receivers']!=1:raise AssertionError('fan-out did not use one receiver')
          report['viewer_results'].append({'viewers':count,'duration_seconds':elapsed,'http_payload_bytes_read':sum(n for n,_ in delivered),'output_bits_per_second':(current['counters']['output_bytes']-before)*8/elapsed,'cpu_percent_latest':samples[-1]['cpu_percent'],'rss_bytes_max_observed':max((s['rss_bytes'] for s in samples if s['rss_bytes'] is not None),default=None),'go_heap_bytes_max_observed':max(s['heap_bytes'] for s in samples),'goroutines_max_observed':max(s['goroutines'] for s in samples),'active_receivers':metrics['sample']['receivers'],'packet_integrity_errors':0,'all_clients_disconnected':True})
        wait(lambda:api('/api/v1/streams')['streams'][0]['state']=='stopped')
        for _ in range(10):
          api('/api/v1/streams/load/start','POST',token=token);api('/api/v1/streams/load/stop','POST',token=token)
        report['start_stop_cycles']=10
        api('/api/v1/streams/load/start','POST',token=token)
        if os.name=='nt':
          # CI runners may have no Win32 console to deliver CTRL_BREAK to.
          # Native SCM graceful-stop validation is a separate required test.
          report['shutdown_validation']='not exercised by this Windows load harness; see windows-service-results.json'
          proc.terminate();proc.wait(timeout=12)
        else:
          proc.send_signal(signal.SIGTERM);proc.wait(timeout=12);report['graceful_shutdown_exit_code']=proc.returncode
          if proc.returncode!=0:raise AssertionError('graceful shutdown failed')
        report['status']='passed'
      finally:
        stop_source.set();source.join(timeout=2)
        if proc.poll() is None:proc.terminate();proc.wait(timeout=10)
except Exception as error:
 report['status']='failed_or_blocked';report['error']=str(error)
finally:
 output=Path(args.report);output.parent.mkdir(parents=True,exist_ok=True);output.write_text(json.dumps(report,indent=2),encoding='utf-8');print(output)
if report.get('status')!='passed':raise SystemExit(1)
