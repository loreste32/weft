#!/usr/bin/env python3
"""Recursion microbench. Pair: fib.weft"""

def fib(n: int) -> int:
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)

def main() -> None:
    print(fib(28))

if __name__ == "__main__":
    main()
