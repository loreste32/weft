#!/usr/bin/env python3
"""String-heavy glue: split / join / upper. Pair: str_split_join.weft"""

def main() -> None:
    base = "alpha,beta,gamma,delta,epsilon"
    last = ""
    for _ in range(8000):
        parts = base.split(",")
        last = "|".join(p.upper() for p in parts)
    print(last)

if __name__ == "__main__":
    main()
