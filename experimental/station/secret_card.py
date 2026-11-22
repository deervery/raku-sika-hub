"""One-use Gemini key entry, behind tailnet-only HTTPS. Never logs key values."""
import argparse
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import secrets
import tempfile
import threading

PAGE = '''<!doctype html><html lang="ja"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>ラクシカ Gemini接続</title><link rel="stylesheet" href="/style.css"><script src="/app.js" defer></script><main><p>ラクシカステーション · 検証環境</p><h1>Geminiキーを設定</h1><p>キーはこのPCのサーバー用領域に保存します。チャット、Androidアプリ、Gitには含めません。前後の空白・改行は自動で取り除きます。</p><form id="form"><label for="key">Gemini API key</label><input id="key" type="password" autocomplete="off" spellcheck="false" autocapitalize="none" required maxlength="4096"><button id="save">安全に保存する</button></form><p id="result" role="status" aria-live="polite"></p></main></html>'''
CSS = '''body{background:#0b1d1b;color:#edfff7;font-family:system-ui;margin:0;padding:24px}main{max-width:520px;margin:7vh auto;padding:28px;background:#15332d;border-radius:28px}h1{font-size:32px}p{line-height:1.8}label{display:block;margin-top:24px}input,button{box-sizing:border-box;width:100%;font:inherit;padding:18px;border-radius:14px;margin-top:12px}input{background:#071713;color:white;border:1px solid #82b6a4}button{background:#b6ffe2;color:#09251c;border:0;font-weight:bold}button:disabled{opacity:.5}'''
JS = '''let token=location.hash.slice(1);history.replaceState(null,'',location.pathname);const form=document.getElementById('form'),status=document.getElementById('result'),save=document.getElementById('save'),key=document.getElementById('key');if(!token){save.disabled=true;status.textContent='チャットの専用リンクから開いてください。';}form.onsubmit=async e=>{e.preventDefault();save.disabled=true;status.textContent='保存しています…';try{const response=await fetch('/api/key',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer '+token},body:JSON.stringify({key:key.value})});const body=await response.json();if(!response.ok)throw Error(body.error);key.value='';token='';form.hidden=true;status.textContent='保存しました。キーは再表示しません。この画面を閉じて大丈夫です。';}catch(e){status.textContent=e.message;save.disabled=false;}};'''


def save_key(destination, value):
    # Accept only a single token, preventing env-file injection.
    if isinstance(value,str):
        value=value.strip().strip("\ufeff\u200b\u200c\u200d\u2060").strip()
    # Google key formats are not limited to the legacy AIza token length or
    # alphabet. In particular, do not reject dots or long auth-bound keys.
    if not isinstance(value,str) or not re.fullmatch(r"[A-Za-z0-9._~+/=-]{16,4096}",value):
        raise ValueError("キーの途中に空白・改行がないか確認してください。前後の空白は自動で除去しています。")
    destination.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(destination.parent,0o700)
    fd, name = tempfile.mkstemp(dir=destination.parent,prefix=".key-")
    try:
        with os.fdopen(fd,"w") as file:
            os.fchmod(file.fileno(),0o600)
            file.write("GEMINI_API_KEY="+value+"\n")
            file.flush();os.fsync(file.fileno())
        os.replace(name,destination)
        directory=os.open(destination.parent,os.O_RDONLY)
        try:os.fsync(directory)
        finally:os.close(directory)
    finally:
        if os.path.exists(name):os.unlink(name)


def make_server(port, origin, destination, token):
    lock=threading.Lock()
    saved=destination.exists()
    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def response(self,code,data,content_type="application/json"):
            body=data.encode() if isinstance(data,str) else json.dumps(data).encode()
            self.send_response(code)
            self.send_header("Content-Type",content_type+"; charset=utf-8")
            self.send_header("Cache-Control","no-store")
            self.send_header("Referrer-Policy","no-referrer")
            self.send_header("X-Content-Type-Options","nosniff")
            self.send_header("Content-Security-Policy","default-src 'self'; script-src 'self'; style-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
            self.send_header("Content-Length",str(len(body)))
            self.end_headers();self.wfile.write(body)
        def do_GET(self):
            pages={"/":(PAGE,"text/html"),"/app.js":(JS,"application/javascript"),"/style.css":(CSS,"text/css")}
            if self.path not in pages:self.response(404,{"error":"not found"});return
            body,mime=pages[self.path];self.response(200,body,mime)
        def do_POST(self):
            nonlocal saved
            if self.path!="/api/key":self.response(404,{"error":"not found"});return
            if self.headers.get("Origin")!=origin or not hmac.compare_digest(self.headers.get("Authorization",""),"Bearer "+token):
                self.response(403,{"error":"専用リンクから再度開いてください。"});return
            if self.headers.get("Content-Type")!="application/json":self.response(415,{"error":"JSON required"});return
            try:
                length=int(self.headers.get("Content-Length","0"))
                if length<=0 or length>8192:raise ValueError()
                data=json.loads(self.rfile.read(length))
                if not isinstance(data,dict) or set(data)!={"key"}:raise ValueError()
            except (ValueError,TypeError):self.response(400,{"error":"入力を確認してください。"});return
            with lock:
                if saved:self.response(409,{"error":"このリンクは使用済みです。"});return
                try:save_key(destination,data["key"])
                except ValueError as error:self.response(400,{"error":str(error)});return
                except OSError:self.response(503,{"error":"保存できませんでした。管理者へ連絡してください。"});return
                saved=True
                self.response(200,{"saved":True})
    return ThreadingHTTPServer(("127.0.0.1",port),Handler)


if __name__=="__main__":
    parser=argparse.ArgumentParser()
    parser.add_argument("--origin",required=True)
    parser.add_argument("--port",type=int,default=3111)
    parser.add_argument("--destination",default=str(Path.home()/".config/rakusikastation/secrets/gemini.env"))
    parser.add_argument("--token-file")
    args=parser.parse_args()
    if not args.origin.startswith("https://") or not args.origin.split("://",1)[1].split(":",1)[0].endswith(".ts.net"):
        raise SystemExit("tailnet HTTPS origin required")
    token=Path(args.token_file).read_text().strip() if args.token_file else secrets.token_urlsafe(32)
    server=make_server(args.port,args.origin.rstrip("/"),Path(args.destination),token)
    print("KEY_CARD_URL="+args.origin.rstrip("/")+"/#"+token,flush=True)
    server.serve_forever()
