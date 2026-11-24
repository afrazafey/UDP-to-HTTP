import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createSocket } from "node:dgram";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer as createHTTPServer } from "node:http";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { createRequire } from "node:module";
import { setTimeout as delay } from "node:timers/promises";

const [binary, output = "build/dashboard-screenshots", playwrightPath = "playwright-core"] = process.argv.slice(2);
if (!binary) throw new Error("Supply the built daemon path.");
const require = createRequire(import.meta.url);
const { chromium } = require(playwrightPath.startsWith(".") ? resolve(playwrightPath) : playwrightPath);
const useFixture = binary === "--fixture";
const captureRoutes = process.env.UDPHTTP_CAPTURE_ROUTES === "1";
const directory = await mkdtemp(join(tmpdir(), "udphttp-dashboard-"));
const ports = new Set();
while (ports.size < 3) {
  const server = createServer();
  await new Promise((resolve, reject) => server.once("error",reject).listen(0,"127.0.0.1",resolve));
  ports.add(server.address().port);
  await new Promise(resolve=>server.close(resolve));
}
const [management, playback, secondary] = [...ports];
const base = `http://127.0.0.1:${management}`;
const udpSocket = createSocket("udp4");
await new Promise(resolve=>udpSocket.bind(0,"127.0.0.1",resolve));
const udp = udpSocket.address().port;
udpSocket.close();
const config = join(directory,"config.yaml");
const channels = [
  {id:"studio-feed",name:"Studio Feed",group:"Test feeds",source_type:"unicast",source_address:"127.0.0.1",source_port:udp,interface:"127.0.0.1",enabled:true,startup:"always"},
];
await writeFile(config, JSON.stringify({
  management:{port:management},network:{http_port:playback,secondary_http_port:secondary},
  storage:{path:join(directory,"state.db")},playlist:{advertised_host:"127.0.0.1"},
  streaming:{allow_loopback_unicast:true,allowed_unicast_sources:["127.0.0.1"],allowed_client_networks:["127.0.0.0/8"]},
  dashboard:{theme:"light",refresh_seconds:3},channels,
}));
const sender=createSocket("udp4");
let source, browser, daemon, mock, startupError, log="";
try {
  if (useFixture) {
    const { fixture, statusFixture } = await import("../frontend/src/test/operations-fixture.ts");
    const data = fixture();
    data.streams[0].channel.id = "studio-feed";
    data.streams[0].channel.name = "Studio Feed";
    data.clients = [{id:1,channel_id:"studio-feed",remote_ip:"192.0.2.24",connected_at:data.dashboard.sample.time,queued_bytes:0,closing:false,duration_seconds:83,bytes_delivered:12400000,output_bitrate:1490000}];
    data.settings.active.dashboard.theme = "light";
    data.settings.settings.dashboard.theme = "light";
    data.dashboard.version = "0.4.2-rc.1";
    data.system.version = "0.4.2-rc.1";
    data.system.environment = "Production";
    data.system.release_maturity = "Release candidate";
    data.dashboard.activity = [{id:1,time:data.dashboard.sample.time,level:"info",component:"receiver",stream_id:"studio-feed",message:"Receiver started (test fixture)",context:{}}];
    data.dashboard.history = Array.from({length:31},(_,i)=>({...data.dashboard.sample,time:new Date(Date.parse(data.dashboard.sample.time)+i*3000).toISOString(),input_bitrate:1470000+Math.sin(i/4)*120000,output_bitrate:2920000+Math.sin(i/3)*240000}));
    data.system.http_listeners.push({kind:"playback",address:"127.0.0.1:7781",state:"listening"});
    const routes = {
      "/health":{storage_ready:true},
      "/api/v1/status":{...statusFixture, version:"0.4.2-rc.1",environment:"Production",phase:4},
      "/api/v1/dashboard":data.dashboard,
      "/api/v1/streams":{streams:data.streams},
      "/api/v1/streams/studio-feed/metrics":{stream:data.streams[0],history:data.dashboard.history},
      "/api/v1/clients":{clients:data.clients},
      "/api/v1/interfaces":{interfaces:data.interfaces},
      "/api/v1/settings":data.settings,
      "/api/v1/system":data.system,
      "/api/v1/csrf":{token:"fixture"},
      "/api/v1/diagnostics":{channels:[{channel_id:"studio-feed",configuration_valid:true,interface_error:null,socket_bound:true,multicast_membership_joined:true,last_datagram_at:data.dashboard.sample.time,mpeg_ts_packets:5012,http_payload_bytes_delivered:12400000,active_viewers:1}]},
      "/api/v1/logs":{logs:data.dashboard.activity,total:data.dashboard.activity.length},
      "/api/v1/playlist/config":{playlist:{advertised_host:"127.0.0.1",remote_enabled:false},primary_port:7781,secondary_port:7782},
    };
    mock=createHTTPServer(async (req,res)=>{
      const path = new URL(req.url, base).pathname;
      try {
        if (path === "/api/v1/settings" && req.method === "PUT") {
          const chunks=[];
          for await (const chunk of req) chunks.push(chunk);
          const {settings}=JSON.parse(Buffer.concat(chunks));
          data.settings.settings=settings;
          data.settings.active=structuredClone(settings);
          data.settings.revision++;
        }
        if (Object.hasOwn(routes,path)) {
          res.writeHead(200,{"Content-Type":"application/json"});
          res.end(JSON.stringify(routes[path]));
        } else if (path==="/" || path==="/favicon.svg" || /^\/assets\/[\w.-]+\.(css|js)$/.test(path)) {
          const file=path==="/" ? "index.html" : path.slice(1);
          const type=path==="/favicon.svg" ? "image/svg+xml" : file.endsWith(".css") ? "text/css" : file.endsWith(".js") ? "text/javascript" : "text/html";
          const body=await readFile(join("internal/web/dist",file));
          res.writeHead(200,{"Content-Type":type,"Content-Security-Policy":"default-src 'self'; object-src 'none'; frame-ancestors 'none'"});
          res.end(body);
        } else {
          res.writeHead(404,{"Content-Type":"application/json"});
          res.end(JSON.stringify({error:"Fixture route unavailable"}));
        }
      } catch (error) { res.writeHead(500); res.end(String(error)); }
    });
    await new Promise((resolve,reject)=>mock.once("error",reject).listen(management,"127.0.0.1",resolve));
  } else {
    daemon=spawn(resolve(binary),["serve","--config",config],{stdio:["ignore","ignore","pipe"],windowsHide:true});
    daemon.on("error",error=>startupError=error);
    daemon.stderr.on("data",chunk=>{log=(log+chunk).slice(-8192);});
  }
  let ready=false;
  for (let attempt=0; attempt<80; attempt++) {
    if (startupError) throw startupError;
    if (daemon && daemon.exitCode !== null) throw new Error(log || `Daemon exited ${daemon.exitCode}`);
    try { ready=(await fetch(base+"/health")).ok; if(ready) break } catch {}
    await delay(150);
  }
  assert(ready,"Daemon did not reach /health");
  let cc=0;
  if (!useFixture) source=setInterval(()=>{
    const packets=Buffer.alloc(188*7,0xff);
    for(let i=0;i<7;i++) {const p=i*188;packets[p]=0x47;packets[p+1]=1;packets[p+2]=0;packets[p+3]=0x10|(cc++&15);}
    sender.send(packets,udp,"127.0.0.1");
  },20);
  let reader, readerDone;
  if (!useFixture) {
    const viewer=await fetch(`http://127.0.0.1:${playback}/live/studio-feed.ts`,{signal:AbortSignal.timeout(120000)});
    assert.equal(viewer.headers.get("content-type"),"video/mp2t");
    reader=viewer.body.getReader();
    const read=async()=>{while(!(await reader.read()).done){}};
    readerDone=read().catch(error=>{if(error.name!=="AbortError") throw error;});
  }
  try {
    browser=await chromium.launch({executablePath:process.env.UDPHTTP_BROWSER_PATH || undefined});
    const context=await browser.newContext({viewport:{width:1440,height:1080},reducedMotion:"reduce"});
    const page=await context.newPage();
    const errors=[];
    page.on("pageerror",error=>errors.push(error.message));
    await page.goto(base+"/#/overview");
    await page.getByRole("img",{name:"Network throughput"}).waitFor({timeout:18000});
    await page.getByRole("button",{name:"30 seconds",exact:true}).waitFor({timeout:45000});
    if (useFixture) {
      const range=page.getByRole("group",{name:"Throughput time range"});
      await range.getByRole("button",{name:"30 seconds"}).click();
      assert.equal(await range.getByRole("button",{name:"30 seconds"}).getAttribute("aria-pressed"),"true");
      assert.equal(await range.getByRole("button",{name:"15 minutes"}).count(),0);
      const inspector=page.getByText("Inspect samples",{exact:true});
      await inspector.click();
      const slider=page.getByRole("slider",{name:"Inspect historical sample"});
      assert.equal(await slider.getAttribute("max"),"10");
      const reading=await slider.getAttribute("aria-valuetext");
      await slider.press("ArrowLeft");
      assert.notEqual(await slider.getAttribute("aria-valuetext"),reading);
      await range.getByRole("button",{name:"Available history"}).click();
      assert.equal(await slider.getAttribute("max"),"30");
      await inspector.click();
    }
    await mkdir(output,{recursive:true});
    const widths=[320,375,390,430,768,1024,1280,1360,1440,1920,2560];
    const noOverflow=async(label)=>{
      const result=await page.evaluate(()=>{
        if(document.documentElement.scrollWidth<=window.innerWidth) {
          const overflowingCard=[...document.querySelectorAll(".op-adaptive-table")]
            .find(element=>element.clientWidth<=740 && element.scrollWidth>element.clientWidth+1);
          return overflowingCard ? ["record cards"] : null;
        }
        return [...document.querySelectorAll("body *")]
          .filter(element=>element.getBoundingClientRect().right>window.innerWidth+1)
          .slice(0,6).map(element=>element.getAttribute("class") || element.tagName);
      });
      assert.equal(result,null,`Horizontal page overflow: ${label} (${result?.join(", ")})`);
    };
    const routes=["overview","streams","streams/studio-feed","playlists","import-export","import-export?mode=export","network","clients","monitoring","logs","settings","system"];
    for (const width of widths) {
      await page.setViewportSize({width,height:width===1360?765:width<768?840:1024});
      for (const route of routes) {
        await page.goto(base+"/#/"+route);
        await page.waitForFunction(route => location.hash === "#/"+route, route);
        await page.locator(".op-content > .op-footer").waitFor();
        await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
        await noOverflow(`${width}px ${route}`);
        if(captureRoutes && (width===390 || width===1360) && !route.includes("/") && route!=="overview") {
          if(route==="logs") await page.locator(".op-log-message").first().waitFor();
          const name=route.replace("?mode=","-");
          await page.screenshot({path:join(output,`${name}-${width}.png`),fullPage:width===390});
        }
      }
      await page.goto(base+"/#/overview");
      await page.getByRole("heading",{name:"Overview",exact:true}).waitFor();
      if(width===1360 || width===390) {
        const [throughput,health]=await Promise.all([
          page.locator(".op-overview-throughput").boundingBox(),
          page.locator(".op-overview-health").boundingBox(),
        ]);
        assert(throughput && health,"Overview panels are visible");
        if(width===1360) {
          assert(health.x>=throughput.x+throughput.width,"Desktop health panel sits beside throughput");
          assert.equal(health.y,throughput.y);
        } else {
          assert(health.y>=throughput.y+throughput.height,"Mobile health panel stacks below throughput");
        }
      }
      if(width===390 || width===768 || width===1360 || width===1440) {
        await page.screenshot({path:join(output,`overview-${width}.png`),fullPage:width!==1360});
      }
    }
    await page.setViewportSize({width:390,height:840});
    await page.getByRole("button",{name:"Open navigation"}).click();
    await page.getByRole("dialog",{name:"Navigation"}).waitFor();
    const aboutLink=page.getByRole("dialog",{name:"Navigation"}).getByRole("link",{name:"afraaz.xyz"});
    assert.equal(await aboutLink.getAttribute("href"),"https://afraaz.xyz");
    assert.match(await aboutLink.getAttribute("rel"),/\bnoopener\b/);
    await noOverflow("mobile navigation");
    await page.keyboard.press("Escape");
    assert(await page.getByRole("button",{name:"Open navigation"}).evaluate(el=>document.activeElement===el));
    await page.getByRole("button",{name:"Add Channel",exact:true}).click();
    await page.getByRole("dialog",{name:"Add stream"}).waitFor();
    await noOverflow("mobile channel form");
    await page.screenshot({path:join(output,"channel-form-390.png")});
    await page.keyboard.press("Escape");
    await page.goto(base+"/#/streams");
    const firstRow=page.locator(".op-adaptive-table tbody tr").first();
    await firstRow.waitFor();
    assert.equal(await firstRow.evaluate(el=>getComputedStyle(el).display),"grid");
    const actions=page.getByLabel("Actions for Studio Feed");
    await actions.click();
    await page.getByRole("button",{name:"Edit Studio Feed"}).click();
    await page.getByRole("dialog",{name:"Edit Studio Feed"}).waitFor();
    await page.keyboard.press("Escape");
    const channelSearch=page.getByLabel("Search streams");
    await channelSearch.fill("no matching channel");
    await page.getByRole("heading",{name:"No matching streams"}).waitFor();
    await channelSearch.clear();
    await firstRow.waitFor();
    await page.getByText("More filters",{exact:true}).click();
    await page.getByLabel("Enabled state").selectOption("disabled");
    await page.getByRole("heading",{name:"No matching streams"}).waitFor();
    await page.getByLabel("Enabled state").selectOption("");
    await firstRow.waitFor();
    await noOverflow("expanded channel filters");
    await page.setViewportSize({width:1360,height:765});
    assert.equal(await firstRow.evaluate(el=>getComputedStyle(el).display),"table-row");
    await page.goto(base+"/#/playlists");
    await page.getByRole("heading",{name:"Playlist preview"}).waitFor();
    const previewRow=page.getByRole("table",{name:"Playlist order for the selected group"}).locator("tbody tr").first();
    const previewBounds=await previewRow.boundingBox();
    assert(previewBounds && previewBounds.height<140 && previewBounds.y<500,"Playlist preview shows channel rows near the panel heading");
    await page.goto(base+"/#/settings");
    const general=page.getByRole("tab",{name:"General"});
    await general.press("ArrowRight");
    const streaming=page.getByRole("tab",{name:"Streaming",selected:true});
    assert(await streaming.evaluate(el=>document.activeElement===el));
    await page.getByRole("tabpanel",{name:"Streaming"}).waitFor();
    await page.goto(base+"/#/overview");
    await page.setViewportSize({width:1440,height:1080});
    await page.getByLabel("Dashboard theme").selectOption("dark");
    await page.waitForFunction(()=>document.documentElement.dataset.theme==="dark");
    await page.screenshot({path:join(output,"overview-dark-1440.png"),fullPage:true});
    if(captureRoutes) {
      await page.goto(base+"/#/monitoring");
      await page.getByRole("heading",{name:"Measured throughput history"}).waitFor();
      await noOverflow("dark monitoring");
      await page.screenshot({path:join(output,"monitoring-dark-1440.png")});
    }
    assert.deepEqual(errors,[]);
    console.log(`Verified ${routes.length} routes at ${widths.length} viewport widths, table-to-card layout, channel search and actions, settings keyboard navigation, theme saving, and mobile drawer focus (${useFixture ? "fixture API with chart range selection" : "live daemon"}).`);
  } finally {
    await reader?.cancel();
    await readerDone;
  }
} finally {
  clearInterval(source);
  sender.close();
  await browser?.close();
  if (daemon?.pid && daemon.exitCode === null) {
    const closed = new Promise(resolve=>daemon.once("close",resolve));
    daemon.kill();
    await closed;
  }
  if (mock) await new Promise(resolve=>mock.close(resolve));
  await rm(directory,{recursive:true,force:true});
}
