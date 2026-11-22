"""Score human/field JSONL trials, keeping missing measurements null.

Each row supplies model/device/noise, expected/observed field dictionaries,
and optional measured timings. This does not generate ground truth or audio.
"""
import argparse
from collections import defaultdict
import json
import math
import statistics

FIELDS = ("product", "cut", "lot", "weight", "unit", "copies", "amount")
MEASURES = ("acoustic_start_ms", "playback_stop_ms", "asr_ms", "first_pcm_ms", "cpu_percent", "rss_mib")


def summarize(rows):
    groups = defaultdict(list)
    for row in rows:
        groups[(row["model"], row["device"], row["noise"])].append(row)
    output = []
    for (model,device,noise), trials in groups.items():
        counts = {}
        for field in FIELDS:
            subset = [r for r in trials if field in r["expected"]]
            if subset:
                matches = sum(r.get("observed",{}).get(field) == r["expected"][field] for r in subset)
                counts[field] = {"n":len(subset),"exact":matches,"rate":matches/len(subset)}
        timing = {}
        for key in MEASURES:
            values = sorted(r[key] for r in trials if isinstance(r.get(key),(int,float)) and not isinstance(r.get(key),bool) and math.isfinite(r[key]) and r[key]>=0)
            timing[key] = {"n":len(values),"median":statistics.median(values) if values else None,"p95_nearest_rank":values[math.ceil(.95*len(values))-1] if values else None}
        output.append({"model":model,"device":device,"noise":noise,"trials":len(trials),"field_exact":counts,"measurements":timing})
    return output


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("trials")
    args = parser.parse_args()
    with open(args.trials,encoding="utf8") as f:
        print(json.dumps(summarize([json.loads(line) for line in f if line.strip()]),ensure_ascii=False,indent=2))
