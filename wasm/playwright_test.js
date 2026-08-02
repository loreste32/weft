"use strict";

const assert = require("assert");
const fs = require("fs");
const http = require("http");
const path = require("path");
const { chromium, firefox } = require("@playwright/test");

const root = path.resolve(__dirname, "..");

function contentType(filePath) {
  if (filePath.endsWith(".html")) return "text/html; charset=utf-8";
  if (filePath.endsWith(".js")) return "text/javascript; charset=utf-8";
  if (filePath.endsWith(".wasm")) return "application/wasm";
  return "application/octet-stream";
}

function startServer() {
  const server = http.createServer((request, response) => {
    try {
      const requestPath = decodeURIComponent(new URL(request.url, "http://127.0.0.1").pathname);
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
      resolve({ server, url: `http://127.0.0.1:${address.port}` });
    });
  });
}

async function checkBrowser(name, browserType, baseURL) {
  const browser = await browserType.launch({ headless: true });
  const page = await browser.newPage();
  const pageErrors = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  try {
    await page.goto(`${baseURL}/wasm/playground.html`, { waitUntil: "load" });
    const result = await page.evaluate(async () => {
      const runtime = await globalThis.Weft.load("/wasm/weft.wasm");
      const core = await runtime.runAsync('fn main { say("browser") }');
      const filesystem = await runtime.runAsync([
        "fn main {",
        '    fs.write("browser.txt", "ok")?',
        '    say(fs.read("browser.txt")?)',
        "}",
      ].join("\n"));
      return { core, filesystem, capabilities: runtime.capabilities() };
    });
    assert.deepStrictEqual(result.core, { output: "browser\n", error: null });
    assert.deepStrictEqual(result.filesystem, { output: "ok\n", error: null });
    assert.strictEqual(result.capabilities.browser, true);
    assert.deepStrictEqual(pageErrors, []);
    console.log(`${name}: browser WASM checks passed`);
  } finally {
    await browser.close();
  }
}

async function main() {
  const { server, url } = await startServer();
  try {
    await checkBrowser("Chromium", chromium, url);
    await checkBrowser("Firefox", firefox, url);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
