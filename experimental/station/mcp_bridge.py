"""Local stdio MCP tools forwarding to the SAME Go lab service as Web/voice.

Protocol subset: initialize, ping, tools/list, tools/call (2025-03-26).
Model tools may propose fields and query results, never impersonate user approval.
No production backend or credentials are used.
"""
import json
import sys
from voice_contract import hub

COMMAND_SCHEMA = {
    "type": "object", "additionalProperties": False,
    "properties": {
        "session_id": {"type": "string"}, "revision": {"type": "integer", "minimum": 1},
        "media_epoch": {"type": "integer", "minimum": 1},
        "action": {"type": "string", "enum": ["begin", "patch", "reconcile"]},
        "kind": {"type": "string", "enum": ["print", "inventory", "sale", "stock"]},
        "fields": {"type": "object", "additionalProperties": {"type": "string"}},
    }, "required": ["session_id", "revision", "media_epoch", "action"],
}
TOOLS = [
    {"name": "station_create", "description": "検証用セッションを作成する。実機には接続しない。", "inputSchema": {"type":"object", "properties":{}, "additionalProperties":False}},
    {"name": "station_get", "description": "Goの現在の業務状態と操作結果を照会する。", "inputSchema": {"type":"object", "properties":{"session_id":{"type":"string"}}, "required":["session_id"], "additionalProperties":False}},
    {"name": "station_propose", "description": "項目の提案・訂正または結果照会。確定・承認はできない。", "inputSchema": COMMAND_SCHEMA},
    {"name":"station_analyze", "description":"検証データを整数で集計する。作業画面は保持する。実施設の分析ではない。", "inputSchema":{"type":"object","additionalProperties":False,"properties":{k:{"type":"string"} for k in ("session_id","facility","start","end","compare_start","compare_end","metric")},"required":["session_id","facility","start","end","compare_start","compare_end","metric"]}},
]


def tool(name, args):
    if not isinstance(args, dict):
        raise ValueError("arguments must be an object")
    if name == "station_create" and not args:
        return hub("/api/sessions", {})
    if name == "station_get" and set(args) == {"session_id"}:
        session = args["session_id"]
        if not isinstance(session, str) or len(session) != 32 or any(c not in "0123456789abcdef" for c in session):
            raise ValueError("invalid session id")
        return hub("/api/sessions/" + session)
    if name == "station_propose" and set(args) <= set(COMMAND_SCHEMA["properties"]):
        if args.get("action") not in {"begin", "patch", "reconcile"}:
            raise ValueError("model tool cannot approve or impersonate speech")
        return hub("/api/commands", args)
    if name == "station_analyze":
        return hub("/api/analysis", args)
    raise ValueError("unknown tool or invalid arguments")


def handle(request):
    if not isinstance(request, dict):
        return {"jsonrpc":"2.0", "id":None, "error":{"code":-32600,"message":"Invalid request"}}
    if "id" not in request:
        return None
    response = {"jsonrpc":"2.0", "id":request["id"]}
    method = request.get("method")
    if request.get("jsonrpc") != "2.0":
        response["error"] = {"code":-32600,"message":"Invalid request"}
    elif method == "initialize":
        response["result"] = {"protocolVersion":"2025-03-26", "capabilities":{"tools":{}}, "serverInfo":{"name":"rakusika-station-mock", "version":"0.1.0"}}
    elif method == "ping":
        response["result"] = {}
    elif method == "tools/list":
        response["result"] = {"tools":TOOLS}
    elif method == "tools/call":
        try:
            params = request.get("params", {})
            value = tool(params.get("name"), params.get("arguments", {}))
            response["result"] = {"content":[{"type":"text","text":json.dumps(value, ensure_ascii=False)}],"isError":False}
        except (ValueError, TypeError, AttributeError, OSError) as error:
            response["result"] = {"content":[{"type":"text","text":str(error)}],"isError":True}
    else:
        response["error"] = {"code":-32601,"message":"Method not found"}
    return response


if __name__ == "__main__":
    for line in sys.stdin:
        try:
            request = json.loads(line)
            result = [r for v in request if (r := handle(v)) is not None] if isinstance(request, list) else handle(request)
        except (ValueError, TypeError):
            result = {"jsonrpc":"2.0", "id":None, "error":{"code":-32700,"message":"Parse error"}}
        if result is not None and result != []:
            print(json.dumps(result, ensure_ascii=False), flush=True)
