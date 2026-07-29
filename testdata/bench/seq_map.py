#!/usr/bin/env python3
"""Glue workload: map over a range. Pair: seq_map.weft"""

def main() -> None:
    n = 20000
    ys = [x * 3 + 1 for x in range(n)]
    print(ys[0])
    print(ys[n - 1])

if __name__ == "__main__":
    main()
