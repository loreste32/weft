#!/usr/bin/env python3
"""Glue workload: parse + stringify N JSON objects. Pair: json_roundtrip.weft"""
import json

def main() -> None:
    n = 5000
    last = ""
    for i in range(n):
        s = json.dumps({"n": i, "tag": "item", "ok": True}, separators=(",", ":"))
        o = json.loads(s)
        last = json.dumps(o, separators=(",", ":"))
    if "4999" in last:
        print("ok")
    else:
        print("bad")

if __name__ == "__main__":
    main()
