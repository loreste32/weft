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
    '    say(fs.dir("../escape"))',
    '    say(fs.base("../escape"))',
    '    say(fs.ext("../escape"))',
    '    say(fs.stem("../escape"))',
    '    say(fs.splitext("../escape"))',
    '    say(fs.expanduser("../escape"))',
    '    say(fs.join("../escape", "child"))',
    '    say(fs.norm("../escape"))',
    '    say(fs.abs("../escape"))',
    '    say(fs.parents("../escape"))',
    '    say(fs.rel("../escape", "safe"))',
    '    say(fs.with_suffix("../escape", ".txt"))',
    '    say(fs.read("../escape"))',
    '    say(fs.read_bytes("../escape"))',
    '    say(fs.write("../escape", "blocked"))',
    '    say(fs.write_bytes("../escape", "blocked"))',
    '    say(fs.append("../escape", "blocked"))',
    '    say(fs.exists("../escape"))',
    '    say(fs.is_file("../escape"))',
    '    say(fs.is_dir("../escape"))',
    '    say(fs.list("../escape"))',
    '    say(fs.remove("../escape"))',
    '    say(fs.remove_all("../escape"))',
    '    say(fs.replace("../escape", "dest"))',
    '    say(fs.rename("../escape", "dest"))',
    '    say(fs.write_atomic("../escape", "blocked"))',
    '    say(fs.lines("../escape"))',
    '    say(fs.glob("../escape/*"))',
    '    say(fs.stat("../escape"))',
    '    say(fs.size("../escape"))',
    '    say(fs.chmod("../escape", "0644"))',
    '    say(fs.walk("../escape"))',
    '    fs.copy("../escape", "copy-target")',
    '    fs.write("after-copy", "ok")?',
    '    say(fs.read("after-copy")?)',
    '    fs.write("parent", "file")?',
    '    fs.write("parent/child", "blocked")',
    '    say(fs.exists("parent/child"))',
    '    fs.write("same", "x")?',
    '    fs.rename("same", "same")?',
    '    fs.append("same", "y")?',
    '    say(fs.read("same")?)',
    "}",
  ].join("\n"));
  assert.strictEqual(result.error, null);
  assert.match(result.output, /path escapes virtual filesystem root/);
  assert.match(result.output, /ok\n/);
  assert.match(result.output, /false\nxy\n$/);

  result = await run([
    "fn main {",
    '    r := http.get("data:text/plain,hello")?',
    '    say(r["body"])',
    "}",
  ].join("\n"));
  assert.deepStrictEqual(result, { output: "hello\n", error: null });

  const fetchBeforeSizeLimit = globalThis.fetch;
  let oversizedBodyRead = false;
  globalThis.fetch = async () => ({
    status: 200,
    headers: {
      get: () => String((32 << 20) + 1),
    },
    text: async () => {
      oversizedBodyRead = true;
      return "not read";
    },
  });
  result = await run([
    "fn main {",
    '    r := http.get("https://large.invalid")',
    "    say(r)",
    "}",
  ].join("\n"));
  globalThis.fetch = fetchBeforeSizeLimit;
  assert.strictEqual(result.error, null);
  assert.match(result.output, /http response body exceeds 32 MiB limit/);
  assert.strictEqual(oversizedBodyRead, false);

  let oversizedStreamCancelled = false;
  globalThis.fetch = async () => ({
    status: 200,
    headers: {
      get: () => null,
    },
    body: {
      getReader: () => ({
        read: () => Promise.resolve({
          done: false,
          value: new Uint8Array((32 << 20) + 1),
        }),
        cancel: () => {
          oversizedStreamCancelled = true;
          return Promise.resolve();
        },
      }),
    },
  });
  result = await run([
    "fn main {",
    '    r := http.get("https://stream-large.invalid")',
    "    say(r)",
    "}",
  ].join("\n"));
  globalThis.fetch = fetchBeforeSizeLimit;
  assert.strictEqual(result.error, null);
  assert.match(result.output, /http response body exceeds 32 MiB limit/);
  assert.strictEqual(oversizedStreamCancelled, true);

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

  const capacitySetup = Array.from({ length: 4999 }, (_, index) =>
    `fs.mkdir("d${index}")?`,
  ).join("\n");
  assert.ok(capacitySetup.length < 100000);
  result = await run([
    "fn main {",
    capacitySetup,
    '    fs.write("new/dir/file", "x")',
    '    say(fs.exists("new"))',
    '    fs.write("source", "x")?',
    '    fs.rename("source", "new/dir/dest")',
    '    say(fs.exists("source"))',
    '    say(fs.exists("new"))',
    '    fs.mkdir("overflow")',
    '    say(fs.exists("overflow"))',
    '    fs.temp_dir("overflow-temp")',
    '    say(fs.exists("tmp/overflow-temp1"))',
    "}",
  ].join("\n"), 30000);
  assert.strictEqual(result.error, null);
  assert.deepStrictEqual(result.output.trim().split("\n").slice(-5), [
    "false",
    "true",
    "false",
    "false",
    "false",
  ]);

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
