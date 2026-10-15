const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawnSync}=require('node:child_process');
const assert=require('node:assert/strict');
const catalog=JSON.parse(fs.readFileSync(path.join(__dirname,'v1.json'),'utf8'));
const expected=new Map([['cf-ddns','komari-ddns-stable'],['komari-passkey','komari-passkey-stable']]);
assert.equal(catalog.schema,1);assert.equal(catalog.plugins.length,2);
const seen=new Set();
async function verify(entry){
 assert(expected.has(entry.short),'Unexpected plugin');assert(!seen.has(entry.short),'Duplicate plugin');seen.add(entry.short);
 const repository='https://github.com/xinian5216/'+expected.get(entry.short);
 assert.equal(entry.url,repository);assert.match(entry.version,/^\d+\.\d+\.\d+-stable\.\d+$/);assert.match(entry.sha256,/^[a-f0-9]{64}$/);
 assert.equal(entry.download,`${repository}/releases/download/v${entry.version}/${entry.short}-${entry.version}.zip`);
 assert.equal(entry.komari,entry.short==='cf-ddns'?'>=1.4.0':'>=1.4.3');
 assert(entry.name&&entry.description&&entry.author,'Required metadata missing');
 if(!process.argv.includes('--remote'))return;
 const response=await fetch(entry.download,{signal:AbortSignal.timeout(30000)});assert(response.ok,'Asset download failed');
 const chunks=[];let size=0;
 for await(const chunk of response.body){size+=chunk.length;assert(size<=8*1024*1024,'Plugin exceeds 8 MiB');chunks.push(chunk);}
 const zip=Buffer.concat(chunks);assert.equal(crypto.createHash('sha256').update(zip).digest('hex'),entry.sha256,'Asset checksum mismatch');
 const os=require('node:os');const dir=fs.mkdtempSync(path.join(os.tmpdir(),'komari-catalog-'));
 const file=path.join(dir,'plugin.zip');
 try{
  fs.writeFileSync(file,zip);
  const result=spawnSync('unzip',['-p',file,'komari-plugin.json'],{encoding:'utf8',maxBuffer:1024*1024});
  if(result.error)throw result.error;assert.equal(result.status,0,'Cannot read root manifest');
  checkManifest(JSON.parse(result.stdout),entry);
 }finally{if(fs.existsSync(file))fs.unlinkSync(file);fs.rmdirSync(dir);}
 console.log(`Verified ${entry.short} ${entry.version}: ${size} bytes`);
}
function checkManifest(manifest,entry){
 for(const key of ['short','version','komari'])assert.equal(manifest[key],entry[key],`Manifest ${key} mismatch`);
 assert.equal(manifest.entry,'script.js');assert.equal(manifest.permissions.allowSystemRPC,true);
 for(const key of ['allowExec','allowListen','allowAllFileAccess'])assert.notEqual(manifest.permissions[key],true,'Unexpected privileged permission');
}
(async()=>{for(const entry of catalog.plugins)await verify(entry);console.log('Stable plugin catalog is valid.');})().catch(error=>{console.error(error.message);process.exitCode=1;});
