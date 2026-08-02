"use strict";

const assert = require("assert");
const fs = require("fs");

require("./wasm_exec.js");

async function start() {
  const wasmPath = process.argv[2] || "wasm/weft.wasm";
  const go = new Go();
  const bytes = fs.readFileSync(wasmPath);
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  go.run(instance);
  const deadline = Date.now() + 15000;
  while (typeof globalThis.runWeftAsync !== "function") {
    if (Date.now() >= deadline) throw new Error("runWeftAsync was not registered");
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

function run(source, timeoutMs = 5000) {
  return globalThis.runWeftAsync(source, timeoutMs);
}

async function main() {
  await start();
  let result = await run("fn main { say(1 + 2) }");
  assert.deepStrictEqual(result, { output: "3\n", error: null });

  const concurrent = await Promise.all([
    run("fn main { say(\"first\") }"),
    run("fn main { say(\"second\") }"),
  ]);
  assert.deepStrictEqual(concurrent.map((value) => value.output), ["first\n", "second\n"]);

  result = await run([
    "fn main {",
    '    fs.write("notes.txt", "hello")?',
    '    say(fs.list()?)',
    '    say(fs.walk(".")?)',
    '    say(fs.read("notes.txt")?)',
    "}",
  ].join("\n"));
  assert.strictEqual(result.error, null);
  assert.match(result.output, /notes\.txt/);
  assert.match(result.output, /hello\n$/);

  result = await run([
    "fn main {",
    '    r := http.get("data:text/plain,hello")?',
    '    say(r["body"])',
    "}",
  ].join("\n"));
  assert.deepStrictEqual(result, { output: "hello\n", error: null });

  const nativeFetch = globalThis.fetch;
  globalThis.fetch = (_url, options) => new Promise((_resolve, reject) => {
    options.signal.addEventListener("abort", () => reject(new Error("AbortError")));
  });
  result = await run([
    "fn main {",
    '    r := http.get("https://slow.invalid", {"timeout_ms": 20})',
    "    say(r)",
    "}",
  ].join("\n"));
  globalThis.fetch = nativeFetch;
  assert.match(result.output, /context deadline exceeded/);

  result = await run("fn main { while true { } }", 50);
  assert.strictEqual(result.error, "execution timed out");

  result = globalThis.runWeft([
    "fn main {",
    '    r := http.get("data:text/plain,hello")',
    "    say(r)",
    "}",
  ].join("\n"), 1000);
  assert.match(result.output, /requires runAsync/);

  result = globalThis.runWeft("fn main { say(1) }", NaN);
  assert.match(result.error, /timeout must be an integer/);
  result = globalThis.runWeft("fn main { say(1) }", 0);
  assert.match(result.error, /timeout must be an integer/);

  console.log("WASM integration tests passed");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
