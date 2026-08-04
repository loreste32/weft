"use strict";

// Adversarial browser tests for the Weft WASM HTTP stack and virtual fs.
// Unlike the mocked-fetch checks in integration_test.js, every limit test
// here talks to real local HTTP endpoints (node http/net servers on ephemeral
// ports), so the browser network stack — not a mock — produces the bytes.

const assert = require("assert");
const fs = require("fs");
const http = require("http");
const net = require("net");
const path = require("path");
const zlib = require("zlib");
const { chromium, firefox } = require("@playwright/test");

const root = path.resolve(__dirname, "..");
const maxBodyBytes = 32 << 20;
const mib = 1 << 20;

function contentType(filePath) {
  if (filePath.endsWith(".html")) return "text/html; charset=utf-8";
  if (filePath.endsWith(".js")) return "text/javascript; charset=utf-8";
  if (filePath.endsWith(".wasm")) return "application/wasm";
  return "application/octet-stream";
}

// Regular HTTP server: static files plus well-behaved-protocol endpoints.
function startHTTPServer() {
  const state = {
    postRequests: 0,
    timeoutClosed: 0,
    oversizedBytesSent: 0,
  };
  const intervals = new Set();
  const largeBody = Buffer.alloc(maxBodyBytes + 1, "x");
  const gzipLarge = zlib.gzipSync(largeBody);
  const slowChunks = ["first-", "second-", "third-", "fourth-", "fifth-", "done"];
  const server = http.createServer((request, response) => {
    try {
      const requestPath = decodeURIComponent(new URL(request.url, "http://127.0.0.1").pathname);
      if (requestPath === "/adv/get" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain" });
        response.end("adversarial-get");
        return;
      }
      if (requestPath === "/adv/count-post" && request.method === "POST") {
        state.postRequests++;
        const chunks = [];
        request.on("data", (chunk) => chunks.push(chunk));
        request.on("end", () => {
          response.writeHead(200, { "Content-Type": "text/plain" });
          response.end("received");
        });
        return;
      }
      // Declares an over-limit Content-Length, then drips the body slowly.
      // A correct client must reject from the header long before the body
      // would finish transferring.
      if (requestPath === "/adv/oversized-declared" && request.method === "GET") {
        response.writeHead(200, {
          "Content-Type": "application/octet-stream",
          "Content-Length": String(maxBodyBytes + 1),
        });
        const timer = setInterval(() => {
          if (state.oversizedBytesSent > maxBodyBytes + 4 * mib) {
            clearInterval(timer);
            intervals.delete(timer);
            return;
          }
          const chunk = Buffer.alloc(8192, "z");
          state.oversizedBytesSent += chunk.length;
          response.write(chunk);
        }, 250);
        intervals.add(timer);
        response.once("close", () => {
          clearInterval(timer);
          intervals.delete(timer);
        });
        return;
      }
      // Over-limit chunked body (no Content-Length at all).
      if (requestPath === "/adv/chunked-large" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain", "Transfer-Encoding": "chunked" });
        response.end(largeBody);
        return;
      }
      // gzip whose decompressed size exceeds the limit, sent chunked with no
      // Content-Length: only the streaming limiter can catch this one.
      if (requestPath === "/adv/gzip-chunked" && request.method === "GET") {
        response.writeHead(200, {
          "Content-Type": "text/plain",
          "Content-Encoding": "gzip",
          "Transfer-Encoding": "chunked",
        });
        response.end(gzipLarge);
        return;
      }
      // Multi-chunk response well under the limit, dribbled out over time.
      if (requestPath === "/adv/slow-stream" && request.method === "GET") {
        response.writeHead(200, { "Content-Type": "text/plain", "Transfer-Encoding": "chunked" });
        let index = 0;
        const sendNext = () => {
          if (index >= slowChunks.length) {
            response.end();
            return;
          }
          response.write(slowChunks[index]);
          index++;
          setTimeout(sendNext, 30);
        };
        sendNext();
        return;
      }
      if (requestPath === "/adv/redirect-ok" && request.method === "GET") {
        response.writeHead(302, { Location: "/adv/get" });
        response.end();
        return;
      }
      if (requestPath === "/adv/redirect-large" && request.method === "GET") {
        response.writeHead(302, { Location: "/adv/chunked-large" });
        response.end();
        return;
      }
      // Never responds; the client must time out and abort.
      if (requestPath === "/adv/timeout" && request.method === "GET") {
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
  const close = () => {
    for (const timer of intervals) clearInterval(timer);
    intervals.clear();
    return new Promise((resolve) => server.close(resolve));
  };
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      resolve({ server, url: `http://127.0.0.1:${address.port}`, state, close });
    });
  });
}

// Raw socket server for responses node http refuses to produce: garbage,
// negative, duplicate, and deceptive Content-Length headers.
function startRawServer() {
  const chunked = (() => {
    const parts = [];
    let remaining = maxBodyBytes + 1;
    while (remaining > 0) {
      const size = Math.min(remaining, mib);
      parts.push(Buffer.from(`${size.toString(16)}\r\n`, "latin1"));
      parts.push(Buffer.alloc(size, "d"));
      remaining -= size;
    }
    parts.push(Buffer.from("0\r\n\r\n", "latin1"));
    return Buffer.concat(parts);
  })();
  const responses = {
    "/raw/garbage-cl": Buffer.from(
      "HTTP/1.1 200 OK\r\n" +
      "Content-Type: text/plain\r\n" +
      "Content-Length: garbage\r\n" +
      "Access-Control-Allow-Origin: *\r\n" +
      "Connection: close\r\n\r\n" +
      "hello-raw",
      "latin1",
    ),
    "/raw/negative-cl": Buffer.from(
      "HTTP/1.1 200 OK\r\n" +
      "Content-Type: text/plain\r\n" +
      "Content-Length: -1\r\n" +
      "Access-Control-Allow-Origin: *\r\n" +
      "Connection: close\r\n\r\n" +
      "hello-raw",
      "latin1",
    ),
    "/raw/duplicate-cl": Buffer.from(
      "HTTP/1.1 200 OK\r\n" +
      "Content-Type: text/plain\r\n" +
      "Content-Length: 5\r\n" +
      "Content-Length: 6\r\n" +
      "Access-Control-Allow-Origin: *\r\n" +
      "Connection: close\r\n\r\n" +
      "hello-raw",
      "latin1",
    ),
    // Deceptive: declares a 5-byte body but streams > 32 MiB chunked.
    "/raw/underreported-cl": Buffer.concat([
      Buffer.from(
        "HTTP/1.1 200 OK\r\n" +
        "Content-Type: text/plain\r\n" +
        "Content-Length: 5\r\n" +
        "Transfer-Encoding: chunked\r\n" +
        "Access-Control-Allow-Origin: *\r\n" +
        "Connection: close\r\n\r\n",
        "latin1",
      ),
      chunked,
    ]),
  };
  const sockets = new Set();
  const server = net.createServer((socket) => {
    sockets.add(socket);
    socket.once("close", () => sockets.delete(socket));
    socket.once("error", () => socket.destroy());
    let buffer = "";
    socket.on("data", (chunk) => {
      buffer += chunk.toString("latin1");
      if (!buffer.includes("\r\n\r\n")) return;
      const target = (buffer.split(" ")[1] || "").split("?")[0];
      const payload = responses[target];
      if (payload) {
        socket.end(payload);
      } else {
        socket.end(
          "HTTP/1.1 404 Not Found\r\nContent-Length: 9\r\nConnection: close\r\n\r\nnot found",
        );
      }
    });
  });
  const close = () => {
    for (const socket of sockets) socket.destroy();
    return new Promise((resolve) => server.close(resolve));
  };
  return new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      resolve({ server, url: `http://127.0.0.1:${address.port}`, close });
    });
  });
}

async function waitForTimeoutCloses(state, target) {
  const deadline = Date.now() + 5000;
  while (state.timeoutClosed < target && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
}

// An outcome is "explicit" when the runtime either returned a well-formed
// error Result or succeeded with exactly one of the expected bodies inside
// the Ok response map. Anything else (hang, crash, silent truncation) fails
// the assertion.
function assertExplicitOutcome(result, okBodies, label) {
  assert.strictEqual(result.error, null, `${label}: runtime must not crash or time out`);
  const isErr = result.output.startsWith("Err(");
  const isExactOk = okBodies.some((body) =>
    result.output.startsWith(`Ok({"status": 200, "body": ${body}, "headers": {`) &&
    result.output.endsWith("}})\n"));
  assert.ok(isErr || isExactOk, `${label}: unexpected output ${JSON.stringify(result.output.slice(0, 200))}`);
  return isErr ? "err" : "ok";
}

async function checkBrowser(name, browserType, baseURL, rawURL, state) {
  const timeoutClosedBefore = state.timeoutClosed;
  const oversizedBytesBefore = state.oversizedBytesSent;
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
    const result = await page.evaluate(async ({ baseURL, rawURL, maxBodyBytes }) => {
      const runtime = await globalThis.Weft.load("/wasm/weft.wasm");
      const run = (source, timeoutMs = 15000) => runtime.runAsync(source, { timeoutMs });
      const get = (url, opts, timeoutMs) => run([
        "fn main {",
        `    r := http.get("${url}"${opts ? `, ${opts}` : ""})`,
        "    say(r)",
        "}",
      ].join("\n"), timeoutMs);
      const getBody = (url, timeoutMs) => run([
        "fn main {",
        `    r := http.get("${url}")?`,
        "    say(r.body)",
        "}",
      ].join("\n"), timeoutMs);
      const outcomes = {};

      // --- Declared over-limit Content-Length: must reject from the header,
      // long before the slow drip body finishes. ---
      const declaredStart = Date.now();
      outcomes.oversizedDeclared = await get(`${baseURL}/adv/oversized-declared`, null, 15000);
      outcomes.oversizedDeclaredMs = Date.now() - declaredStart;

      // --- Deceptive Content-Length: declares 5 bytes, streams > 32 MiB
      // chunked. Must end in an error (stream limiter or network-layer
      // rejection), never in truncated data reported as success. ---
      outcomes.underreported = await get(`${rawURL}/raw/underreported-cl`, null, 20000);

      // --- Malformed Content-Length headers over raw sockets. ---
      outcomes.garbageCL = await get(`${rawURL}/raw/garbage-cl`);
      outcomes.negativeCL = await get(`${rawURL}/raw/negative-cl`);
      outcomes.duplicateCL = await get(`${rawURL}/raw/duplicate-cl`);

      // --- Compressed expansion with no Content-Length at all. ---
      outcomes.gzipChunked = await get(`${baseURL}/adv/gzip-chunked`);

      // --- Slow multi-chunk streaming within the limit: byte-exact. ---
      outcomes.slowStream = await getBody(`${baseURL}/adv/slow-stream`);

      // --- Redirects: to a good endpoint and to an over-limit one. ---
      outcomes.redirectOk = await getBody(`${baseURL}/adv/redirect-ok`);
      outcomes.redirectLarge = await get(`${baseURL}/adv/redirect-large`);

      // --- Request body limit across body-bearing methods: nothing may
      // reach the server. ---
      const oversizedBody = `str.repeat("x", ${maxBodyBytes + 1})`;
      outcomes.requestPost = await run([
        "fn main {",
        `    r := http.post("${baseURL}/adv/count-post", ${oversizedBody})`,
        "    say(r)",
        "}",
      ].join("\n"));
      outcomes.requestPut = await run([
        "fn main {",
        `    r := http.put("${baseURL}/adv/count-post", ${oversizedBody})`,
        "    say(r)",
        "}",
      ].join("\n"));
      outcomes.requestPatch = await run([
        "fn main {",
        `    r := http.patch("${baseURL}/adv/count-post", ${oversizedBody})`,
        "    say(r)",
        "}",
      ].join("\n"));
      outcomes.requestGeneric = await run([
        "fn main {",
        `    r := http.request("POST", "${baseURL}/adv/count-post", {"body": ${oversizedBody}})`,
        "    say(r)",
        "}",
      ].join("\n"));

      // --- Repeated timed-out executions, then proof the page still works. ---
      outcomes.timeouts = [];
      for (let index = 0; index < 10; index++) {
        outcomes.timeouts.push(await get(`${baseURL}/adv/timeout`, '{"timeout_ms": 25}', 5000));
      }
      outcomes.afterTimeoutGet = await getBody(`${baseURL}/adv/get`);
      outcomes.afterTimeoutCore = await run('fn main { say("alive") }');

      // --- Repeated full-program executions (js.Func / memory churn). ---
      outcomes.repeated = [];
      for (let index = 0; index < 12; index++) {
        outcomes.repeated.push(await run([
          "fn main {",
          '    fs.write("adv-repeat.txt", "repeat")?',
          '    say(fs.read("adv-repeat.txt")?)',
          '    fs.remove("adv-repeat.txt")?',
          `    r := http.get("${baseURL}/adv/get")?`,
          "    say(r.body)",
          "    say(6 * 7)",
          "}",
        ].join("\n")));
      }

      // --- Virtual filesystem: within-quota lifecycle, per-file limit,
      // total-store limit, and byte reclamation after delete. ---
      outcomes.fsLifecycle = await run([
        "fn main {",
        '    fs.write("adv/cleanup.txt", "scratch")?',
        '    say(fs.read("adv/cleanup.txt")?)',
        '    fs.remove("adv/cleanup.txt")?',
        '    say(fs.exists("adv/cleanup.txt"))',
        '    say(fs.write("adv/too-big.txt", str.repeat("y", 16777217)))',
        '    fs.write("adv/f1", str.repeat("a", 13631488))?',
        '    fs.write("adv/f2", str.repeat("a", 13631488))?',
        '    fs.write("adv/f3", str.repeat("a", 13631488))?',
        '    fs.write("adv/f4", str.repeat("a", 13631488))?',
        '    say(fs.write("adv/f5", str.repeat("a", 13631488)))',
        '    fs.remove("adv/f1")?',
        '    fs.remove("adv/f2")?',
        '    fs.remove("adv/f3")?',
        '    fs.remove("adv/f4")?',
        '    fs.write("adv/f6", str.repeat("a", 13631488))?',
        '    say(fs.size("adv/f6")?)',
        '    fs.remove("adv/f6")?',
        '    fs.remove("adv")?',
        '    say(fs.exists("adv"))',
        "}",
      ].join("\n"), 30000);

      return outcomes;
    }, { baseURL, rawURL, maxBodyBytes });

    // Declared over-limit Content-Length: early reject with the limit error,
    // not a hang, and the slow-drip body cannot have transferred much.
    assert.strictEqual(result.oversizedDeclared.error, null);
    assert.match(result.oversizedDeclared.output, /http response body exceeds 32 MiB limit/);
    assert.ok(result.oversizedDeclaredMs < 10000,
      `oversized declared CL rejected after ${result.oversizedDeclaredMs}ms (drip body needs far longer)`);
    assert.ok(state.oversizedBytesSent - oversizedBytesBefore < mib,
      `oversized declared CL: server sent ${state.oversizedBytesSent - oversizedBytesBefore} bytes before rejection`);

    // Deceptive under-reported CL: bounded error, never truncated success.
    // Chromium rejects Content-Length + Transfer-Encoding at the network
    // layer; a browser that honors the chunked stream instead must trip the
    // stream limiter. Either way the error must name a real cause.
    assert.strictEqual(result.underreported.error, null, "deceptive CL must not wedge the runtime");
    assert.ok(result.underreported.output.startsWith("Err("),
      `deceptive CL must not return success, got ${JSON.stringify(result.underreported.output.slice(0, 120))}`);
    assert.match(result.underreported.output,
      /exceeds 32 MiB limit|network error|Failed to fetch|NetworkError/i,
      "deceptive CL error must be explicit");
    console.log(`${name}: deceptive CL outcome: ${result.underreported.output.split("\n")[0].slice(0, 120)}`);

    // Malformed CL headers: explicit, bounded outcome in every browser.
    // Browsers legitimately differ here — some reject at the network layer
    // (Err), others ignore the header and stream the real body (exact ok) —
    // so the assertion pins the set of acceptable outcomes per case and the
    // console log records which one each browser produced.
    const garbage = assertExplicitOutcome(result.garbageCL, ["hello-raw"], "garbage CL");
    const negative = assertExplicitOutcome(result.negativeCL, ["hello-raw"], "negative CL");
    const duplicate = assertExplicitOutcome(result.duplicateCL, ["hello", "hello-raw"], "duplicate CL");
    console.log(`${name}: malformed CL outcomes: garbage=${garbage} negative=${negative} duplicate=${duplicate}`);

    // Compressed expansion without Content-Length: stream limiter trips.
    assert.strictEqual(result.gzipChunked.error, null);
    assert.match(result.gzipChunked.output, /http response body exceeds 32 MiB limit/);

    // Slow streaming within the limit arrives byte-exact.
    assert.deepStrictEqual(result.slowStream,
      { output: "first-second-third-fourth-fifth-done\n", error: null });

    // Redirects.
    assert.deepStrictEqual(result.redirectOk, { output: "adversarial-get\n", error: null });
    assert.strictEqual(result.redirectLarge.error, null);
    assert.match(result.redirectLarge.output, /http response body exceeds 32 MiB limit/);

    // Request body limit on every body-bearing path; server saw nothing.
    for (const key of ["requestPost", "requestPut", "requestPatch", "requestGeneric"]) {
      assert.strictEqual(result[key].error, null, `${key} runtime error`);
      assert.match(result[key].output, /http request body exceeds 32 MiB limit/, key);
    }
    assert.strictEqual(state.postRequests, 0, "oversized request bodies must never reach the server");

    // Repeated timeouts: every one reports the deadline, none wedge the page.
    assert.strictEqual(result.timeouts.length, 10);
    for (const timeout of result.timeouts) {
      assert.strictEqual(timeout.error, null);
      assert.match(timeout.output, /context deadline exceeded/);
    }
    assert.deepStrictEqual(result.afterTimeoutGet, { output: "adversarial-get\n", error: null });
    assert.deepStrictEqual(result.afterTimeoutCore, { output: "alive\n", error: null });
    await waitForTimeoutCloses(state, timeoutClosedBefore + 10);
    assert.strictEqual(state.timeoutClosed - timeoutClosedBefore, 10,
      "every aborted request must close its server-side connection");

    // Repeated full executions: identical, clean results every time.
    assert.strictEqual(result.repeated.length, 12);
    for (const run of result.repeated) {
      assert.deepStrictEqual(run, { output: "repeat\nadversarial-get\n42\n", error: null });
    }

    // Virtual filesystem lifecycle and quotas.
    assert.strictEqual(result.fsLifecycle.error, null);
    const fsLines = result.fsLifecycle.output.trimEnd().split("\n");
    assert.strictEqual(fsLines[0], "scratch");
    assert.strictEqual(fsLines[1], "false");
    assert.match(fsLines[2], /^Err\(Error\{message: fs\.write\("adv\/too-big\.txt"\): file exceeds 16 MiB limit,/);
    assert.match(fsLines[3], /^Err\(Error\{message: fs\.write\("adv\/f5"\): virtual filesystem exceeds 64 MiB limit,/);
    assert.strictEqual(fsLines[4], "13631488");
    assert.strictEqual(fsLines[5], "false");

    assert.deepStrictEqual(pageErrors, []);
    console.log(`${name}: adversarial browser WASM checks passed`);
  } finally {
    await browser.close();
  }
}

async function main() {
  const only = (process.env.WEFT_BROWSER_ONLY || "").toLowerCase();
  const httpServer = await startHTTPServer();
  const rawServer = await startRawServer();
  try {
    if (!only || only === "chromium") {
      await checkBrowser("Chromium", chromium, httpServer.url, rawServer.url, httpServer.state);
    }
    if (!only || only === "firefox") {
      await checkBrowser("Firefox", firefox, httpServer.url, rawServer.url, httpServer.state);
    }
  } finally {
    await httpServer.close();
    await rawServer.close();
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
