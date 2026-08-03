"use strict";

const assert = require("assert");
const fs = require("fs");
const http = require("http");
const path = require("path");
const zlib = require("zlib");
const { chromium, firefox } = require("@playwright/test");

const root = path.resolve(__dirname, "..");
const maxBodyBytes = 32 << 20;

function contentType(filePath) {
  if (filePath.endsWith(".html")) return "text/html; charset=utf-8";
  if (filePath.endsWith(".js")) return "text/javascript; charset=utf-8";
  if (filePath.endsWith(".wasm")) return "application/wasm";
  return "application/octet-stream";
}

function startServer() {
  const state = { postBody: "", timeoutClosed: 0 };
  const largeBody = Buffer.alloc(maxBodyBytes + 1, "x");
  const deceptiveBody = zlib.gzipSync(largeBody);
  const server = http.createServer((request, response) => {
    try {
      const requestPath = decodeURIComponent(new URL(request.url, "http://127.0.0.1").pathname);
      if (requestPath === "/http/get" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain", "Content-Length": "11" });
        response.end("browser-get");
        return;
      }
      if (requestPath === "/http/post" && request.method === "POST") {
        const chunks = [];
        request.on("data", (chunk) => chunks.push(chunk));
        request.on("end", () => {
          state.postBody = Buffer.concat(chunks).toString();
          response.writeHead(200, { "Content-Type": "text/plain" });
          response.end(state.postBody);
        });
        return;
      }
      if (requestPath === "/http/stream" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain", "Transfer-Encoding": "chunked" });
        response.write("stream-");
        setTimeout(() => response.end("response"), 10);
        return;
      }
      if (requestPath === "/http/redirect" && request.method === "GET") {
        response.writeHead(302, { Location: "/http/get" });
        response.end();
        return;
      }
      if (requestPath === "/http/large" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain", "Transfer-Encoding": "chunked" });
        response.end(largeBody);
        return;
      }
      if (requestPath === "/http/deceptive" && request.method === "GET") {
        response.writeHead(200, {
          "Content-Type": "text/plain",
          "Content-Encoding": "gzip",
          "Content-Length": String(deceptiveBody.length),
        });
        response.end(deceptiveBody);
        return;
      }
      if (requestPath === "/http/timeout" && request.method === "GET") {
        request.once("close", () => {
          state.timeoutClosed++;
        });
        return;
      }
      const filePath = path.resolve(root, `.${requestPath}`);
      if (!filePath.startsWith(`${root}${path.sep}`)) {
        response.writeHead(403);
        response.end("forbidden");
        return;
      }
      fs.stat(filePath, (statError, stats) => {
        if (statError || !stats.isFile()) {
          response.writeHead(404);
          response.end("not found");
          return;
        }
        response.writeHead(200, { "Content-Type": contentType(filePath) });
        const file = fs.createReadStream(filePath);
        file.on("error", () => response.destroy());
        file.pipe(response);
      });
    } catch (error) {
      response.writeHead(400);
      response.end(String(error));
    }
  });
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      resolve({ server, url: `http://127.0.0.1:${address.port}`, state });
    });
  });
}

async function waitForTimeoutCloses(state, target) {
  const deadline = Date.now() + 2000;
  while (state.timeoutClosed < target && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
}

async function checkBrowser(name, browserType, baseURL, state) {
  const timeoutClosedBefore = state.timeoutClosed;
  const executablePath = name === "Chromium"
    ? process.env.WEFT_PLAYWRIGHT_CHROMIUM_PATH
    : process.env.WEFT_PLAYWRIGHT_FIREFOX_PATH;
  const browser = await browserType.launch({
    headless: true,
    ...(executablePath ? { executablePath } : {}),
  });
  const page = await browser.newPage();
  const pageErrors = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  try {
    await page.goto(`${baseURL}/wasm/playground.html`, { waitUntil: "load" });
    const result = await page.evaluate(async ({ baseURL, maxBodyBytes }) => {
      const runtime = await globalThis.Weft.load("/wasm/weft.wasm");
      const core = await runtime.runAsync('fn main { say("browser") }');
      const filesystem = await runtime.runAsync([
        "fn main {",
        '    fs.write("browser.txt", "ok")?',
        '    say(fs.read("browser.txt")?)',
        "}",
      ].join("\n"));
      const run = (source, timeoutMs = 10000) => runtime.runAsync(source, { timeoutMs });
      const get = await run([
        "fn main {",
        `    r := http.get("${baseURL}/http/get")?`,
        '    say(r.body)',
        "}",
      ].join("\n"));
      const post = await run([
        "fn main {",
        `    r := http.post("${baseURL}/http/post", "browser-post")?`,
        '    say(r.body)',
        "}",
      ].join("\n"));
      const stream = await run([
        "fn main {",
        `    r := http.get("${baseURL}/http/stream")?`,
        '    say(r.body)',
        "}",
      ].join("\n"));
      const redirect = await run([
        "fn main {",
        `    r := http.get("${baseURL}/http/redirect")?`,
        '    say(r.body)',
        "}",
      ].join("\n"));
      const responseLimit = await run([
        "fn main {",
        `    r := http.get("${baseURL}/http/large")`,
        "    say(r)",
        "}",
      ].join("\n"));
      const deceptiveLength = await run([
        "fn main {",
        `    r := http.get("${baseURL}/http/deceptive")`,
        "    say(r)",
        "}",
      ].join("\n"));
      let fallbackBodyRead = false;
      const nativeFetch = globalThis.fetch;
      globalThis.fetch = async () => ({
        status: 200,
        headers: { get: () => "not-a-number" },
        text: async () => {
          fallbackBodyRead = true;
          return "not read";
        },
      });
      const malformedLength = await run([
        "fn main {",
        '    r := http.get("https://malformed.invalid")',
        "    say(r)",
        "}",
      ].join("\n"));
      globalThis.fetch = nativeFetch;
      const requestLimit = await run([
        "fn main {",
        `    r := http.post("${baseURL}/http/post", str.repeat("x", ${maxBodyBytes + 1}))`,
        "    say(r)",
        "}",
      ].join("\n"));
      const timeouts = [];
      for (let index = 0; index < 4; index++) {
        timeouts.push(await run([
          "fn main {",
          `    r := http.get("${baseURL}/http/timeout", {"timeout_ms": 25})`,
          "    say(r)",
          "}",
        ].join("\n"), 2000));
      }
      return {
        core,
        filesystem,
        capabilities: runtime.capabilities(),
        get,
        post,
        stream,
        redirect,
        responseLimit,
        deceptiveLength,
        malformedLength,
        fallbackBodyRead,
        requestLimit,
        timeouts,
      };
    }, { baseURL, maxBodyBytes });
    assert.deepStrictEqual(result.core, { output: "browser\n", error: null });
    assert.deepStrictEqual(result.filesystem, { output: "ok\n", error: null });
    assert.strictEqual(result.capabilities.browser, true);
    assert.deepStrictEqual(result.get, { output: "browser-get\n", error: null });
    assert.deepStrictEqual(result.post, { output: "browser-post\n", error: null });
    assert.deepStrictEqual(result.stream, { output: "stream-response\n", error: null });
    assert.deepStrictEqual(result.redirect, { output: "browser-get\n", error: null });
    assert.match(result.responseLimit.output, /http response body exceeds 32 MiB limit/);
    assert.match(result.deceptiveLength.output, /http response body exceeds 32 MiB limit/);
    assert.match(result.malformedLength.output, /http response body size cannot be bounded/);
    assert.strictEqual(result.fallbackBodyRead, false);
    assert.match(result.requestLimit.output, /http request body exceeds 32 MiB limit/);
    assert.strictEqual(result.timeouts.length, 4);
    for (const timeout of result.timeouts) {
      assert.match(timeout.output, /context deadline exceeded/);
      assert.strictEqual(timeout.error, null);
    }
    await waitForTimeoutCloses(state, timeoutClosedBefore + 4);
    assert.strictEqual(state.postBody, "browser-post");
    assert.strictEqual(state.timeoutClosed - timeoutClosedBefore, 4);
    assert.deepStrictEqual(pageErrors, []);
    console.log(`${name}: browser WASM checks passed`);
  } finally {
    await browser.close();
  }
}

async function main() {
  const { server, url, state } = await startServer();
  try {
    await checkBrowser("Chromium", chromium, url, state);
    await checkBrowser("Firefox", firefox, url, state);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
