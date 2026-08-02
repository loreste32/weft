"use strict";

const assert = require("assert");
const { WeftRuntime, load } = require("./weft.js");

async function main() {
  globalThis.runWeft = (code, timeoutMs) => ({
    output: code + ":" + timeoutMs,
    error: null,
  });
  globalThis.runWeftAsync = (code, timeoutMs) => Promise.resolve({
    output: code + ":async:" + timeoutMs,
    error: null,
  });

  const runtime = new WeftRuntime();
  assert.deepStrictEqual(runtime.run("x", { timeoutMs: 12 }), {
    output: "x:12",
    error: null,
  });
  assert.deepStrictEqual(await runtime.runAsync("x", { timeoutMs: 13 }), {
    output: "x:async:13",
    error: null,
  });
  assert.strictEqual(runtime.run("x", { timeoutMs: 0 }).error.includes("timeoutMs"), true);
  assert.strictEqual(runtime.run(42).error, "code must be a string");

  const originalGo = globalThis.Go;
  const originalWebAssembly = globalThis.WebAssembly;
  const originalFetch = globalThis.fetch;
  const originalRunWeft = globalThis.runWeft;
  const originalRunWeftAsync = globalThis.runWeftAsync;
  let streamCalls = 0;
  let arrayBufferCalls = 0;
  let fetchCalls = 0;
  let goRuns = 0;

  function response() {
    return {
      ok: true,
      status: 200,
      clone: response,
      arrayBuffer: async () => new ArrayBuffer(0),
    };
  }

  try {
    delete globalThis.runWeft;
    delete globalThis.runWeftAsync;
    globalThis.Go = class FakeGo {
      constructor() { this.importObject = {}; }
      run() {
        goRuns += 1;
        globalThis.runWeft = (code) => ({ output: "loaded:" + code, error: null });
        globalThis.runWeftAsync = (code) => Promise.resolve({ output: "async:" + code, error: null });
        return Promise.resolve();
      }
    };
    globalThis.fetch = async () => { fetchCalls += 1; return response(); };
    globalThis.WebAssembly = {
      instantiateStreaming: async () => {
        streamCalls += 1;
        throw new Error("wrong MIME type");
      },
      instantiate: async () => {
        arrayBufferCalls += 1;
        return { instance: {} };
      },
    };

    const loaded = await load("fixture.wasm");
    assert.strictEqual(loaded.run("hello").output, "loaded:hello");
    assert.strictEqual((await loaded.runAsync("hello")).output, "async:hello");
    assert.strictEqual((await load("fixture.wasm")), loaded);
    assert.strictEqual(streamCalls, 1);
    assert.strictEqual(arrayBufferCalls, 1);
    assert.strictEqual(fetchCalls, 1);
    assert.strictEqual(goRuns, 1);
  } finally {
    if (originalGo === undefined) delete globalThis.Go; else globalThis.Go = originalGo;
    if (originalWebAssembly === undefined) delete globalThis.WebAssembly; else globalThis.WebAssembly = originalWebAssembly;
    if (originalFetch === undefined) delete globalThis.fetch; else globalThis.fetch = originalFetch;
    if (originalRunWeft === undefined) delete globalThis.runWeft; else globalThis.runWeft = originalRunWeft;
    if (originalRunWeftAsync === undefined) delete globalThis.runWeftAsync; else globalThis.runWeftAsync = originalRunWeftAsync;
  }
}

main().then(() => console.log("WASM loader tests passed"), (error) => {
  console.error(error);
  process.exitCode = 1;
});
