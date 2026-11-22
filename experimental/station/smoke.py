"""Real subprocess integration, mock business effects only.

python3 experimental/station/smoke.py --binary /tmp/station-lab \
  --voice-python /path/to/pinned/validation/venv/bin/python
"""
import argparse
import json
import os
from pathlib import Path
import select
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True)
    p.add_argument("--voice-python", required=True)
    args = p.parse_args()
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    env = {**os.environ, "STATION_HUB":base}
    checks = []

    def http(path, body=None):
        req = urllib.request.Request(base+path, data=None if body is None else json.dumps(body).encode(), headers={"Content-Type":"application/json"})
        with urllib.request.urlopen(req, timeout=10) as r:
            return json.load(r)

    def stamp(snapshot, action, **kwargs):
        state = snapshot["state"]
        return {**{k:state[k] for k in ("session_id","revision","media_epoch")},"action":action,**kwargs}

    def ask(proc, value):
        proc.stdin.write(json.dumps(value, ensure_ascii=False)+"\n")
        proc.stdin.flush()
        if not select.select([proc.stdout], [], [], 30)[0]:
            raise TimeoutError("subprocess response")
        return json.loads(proc.stdout.readline())

    def result(response):
        assert not response["result"]["isError"], response
        return json.loads(response["result"]["content"][0]["text"])

    with tempfile.TemporaryDirectory(prefix="station-smoke-") as directory:
        processes = []
        with open(Path(directory)/"stderr.log","w+") as log:
            def server():
                proc = subprocess.Popen([args.binary,"--data",str(Path(directory)/"state"),"--port",str(port)], stdout=log,stderr=log)
                processes.append(proc)
                for _ in range(100):
                    try:
                        http("/api/patterns")
                        return proc
                    except OSError:
                        if proc.poll() is not None:
                            raise RuntimeError("Go server exited")
                        time.sleep(.05)
                raise TimeoutError("Go server readiness")

            try:
                go = server()
                for name, command in [("mcp",[sys.executable,str(ROOT/"mcp_bridge.py")]),("voice",[args.voice_python,str(ROOT/"pipecat_bridge.py")])]:
                    proc = subprocess.Popen(command,env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=log,text=True,bufsize=1)
                    processes.append(proc)
                    if name == "mcp": mcp = proc
                    else: voice = proc
                initialized = ask(mcp,{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}})
                assert initialized["result"]["capabilities"]["tools"] == {}
                checks.append("mcp_initialize")
                def call(name, arguments):
                    return ask(mcp,{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":name,"arguments":arguments}})
                state = result(call("station_create",{}))
                session = state["state"]["session_id"]
                invalid = call("station_propose",stamp(state,"patch",fields={"copies":"-1"}))
                assert invalid["result"]["isError"]
                checks.append("mcp_go_validation")
                state = result(call("station_propose",stamp(state,"patch",fields={"product":"鹿肉","cut":"ロース","lot":"00123","copies":"3"})))
                old_confirm = stamp(state,"confirm",confirmation_id=state["state"]["confirmation"]["id"])
                state = ask(voice,stamp(state,"interrupt"))
                assert not state["speech"]["speak"] and state["speech"]["reply"] == ""
                checks.append("interrupt_does_not_respeak_old_reply")
                state = ask(voice,stamp(state,"utterance",text="2枚に訂正"))
                assert state["state"]["fields"]["copies"] == "2" and not state["speech"]["audio_generated"]
                checks.append("pipecat_to_go_correction")
                try:
                    http("/api/commands",old_confirm)
                    raise AssertionError("stale confirmation accepted")
                except urllib.error.HTTPError as error:
                    assert error.code == 409
                checks.append("stale_confirmation_rejected")
                rejected = call("station_propose",stamp(state,"confirm",confirmation_id=state["state"]["confirmation"]["id"]))
                assert rejected["result"]["isError"]
                checks.append("mcp_cannot_approve")
                accepted = stamp(state,"confirm",confirmation_id=state["state"]["confirmation"]["id"])
                state = ask(voice,stamp(state,"utterance",text="はい",confirmation_id=state["state"]["confirmation"]["id"]))
                assert state["state"]["status"] == "done" and state["operation"]["mock"]
                assert state["operation"]["fields"]["copies"] == "2"
                checks.append("latest_voice_confirmation_mock_only")
                operation = state["operation"]["id"]
                go.terminate();go.wait(10)
                go = server()
                again = http("/api/commands",accepted)
                assert again["operation"]["id"] == operation
                database = json.loads((Path(directory)/"state/state.json").read_text())
                assert len(database["operations"]) == 1
                checks.append("restart_idempotency")
                read = result(call("station_get",{"session_id":session}))
                assert read["state"] == again["state"]
                checks.append("mcp_and_api_same_state")
                job = result(call("station_analyze",{"session_id":session,"facility":"fixture","start":"2026-11-01","end":"2026-11-30","compare_start":"2026-10-01","compare_end":"2026-10-31","metric":"sales_yen"}))
                for _ in range(100):
                    snapshot = http("/api/sessions/"+session)
                    completed = next((j for j in snapshot["jobs"] if j["id"]==job["id"] and j["status"]=="done"),None)
                    if completed:break
                    time.sleep(.01)
                assert completed and completed["current"]==30000 and completed["comparison"]==25000 and completed["difference"]==5000
                assert snapshot["state"]==again["state"]
                checks.append("mcp_analysis_keeps_work_state")
                for path in ("/","/app.js","/app.css"):
                    with urllib.request.urlopen(base+path) as response:
                        assert response.status == 200 and response.read()
                checks.append("ui_assets_http_200")
                print(json.dumps({"passed":len(checks),"checks":checks,"physical_audio_measured":False,"real_devices_used":False}, ensure_ascii=False))
            finally:
                for proc in reversed(processes):
                    if proc.poll() is None:
                        proc.terminate()
                        try:proc.wait(5)
                        except subprocess.TimeoutExpired:proc.kill();proc.wait()


if __name__ == "__main__":
    main()
