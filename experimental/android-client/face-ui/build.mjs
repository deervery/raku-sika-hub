import {build} from 'esbuild';
import {mkdir,writeFile,readFile} from 'node:fs/promises';
const out='../client/src/main/assets/face';
await mkdir(out,{recursive:true});
await build({entryPoints:['src/main.tsx'],bundle:true,minify:true,outdir:out,entryNames:'app',jsx:'automatic',loader:{'.module.css':'local-css'},define:{'process.env.NODE_ENV':'"production"'},metafile:true}).then(async result=>writeFile('bundle-inputs.json',JSON.stringify(Object.keys(result.metafile.inputs).filter(x=>!x.startsWith('node_modules/')),null,2)));
await writeFile(out+'/index.html','<!doctype html><html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; script-src \'self\'; style-src \'self\' \'unsafe-inline\'; img-src \'self\' data:; connect-src \'none\'; base-uri \'none\'; form-action \'none\'"><link rel="stylesheet" href="app.css"><title>ラクシカ アシスタント</title></head><body style="margin:0"><div id="root"></div><script src="app.js"></script></body></html>');

await writeFile(out+'/THIRD_PARTY_LICENSES.txt',(await Promise.all(['react','react-dom'].map(name=>readFile('node_modules/'+name+'/LICENSE','utf8')))).join('\n\n'));
