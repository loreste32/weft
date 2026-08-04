"use strict";

// Browser execution tests for the pure-Weft numerical packages (warp +
// dataframe). The browser harness compiles a single self-contained source, so
// the test fetches the package sources from the local server and inlines them
// into one program — the same sources that run on the host, executed by the
// browser Wasm VM with host tensor storage unavailable (CPU list fallback).

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
  if (filePath.endsWith(".weft")) return "text/plain; charset=utf-8";
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

// Runs inside the page: fetch package sources, inline them into single-file
// programs, and execute warp / dataframe / scale / capability checks.
async function runSuiteInPage() {
  const runtime = await globalThis.Weft.load("/wasm/weft.wasm");

  const warpResponse = await fetch("/packages/warp/lib.weft");
  const warpSource = await warpResponse.text();
  const dfResponse = await fetch("/packages/dataframe/lib.weft");
  const dfSourceRaw = await dfResponse.text();
  if (!warpResponse.ok || !dfResponse.ok || warpSource.length < 1000 || dfSourceRaw.length < 1000) {
    throw new Error("failed to fetch package sources");
  }

  // warp and dataframe both define these top-level names; inlined into one
  // file they would collide, so rename the dataframe copies (definitions and
  // call sites, never `.field` accesses or longer identifiers).
  const renames = [
    ["_merge_values", "_df_merge_values"],
    ["col", "df_col"],
    ["describe", "df_describe"],
    ["diff", "df_diff"],
    ["index", "df_index"],
    ["print_", "df_print_"],
    ["shape", "df_shape"],
  ];
  let dfSource = dfSourceRaw;
  for (const [from, to] of renames) {
    dfSource = dfSource.replace(new RegExp(`(?<![\\w$.])${from}(?=\\s*\\()`, "g"), to);
  }
  // dataframe imports warp as a module; inlined, its four warp calls resolve
  // to the warp globals directly. Runs after the renames so `warp.shape(`
  // survives while the dataframe's own `shape(` is already `df_shape(`.
  dfSource = dfSource.replace(/(?<![\w$.])warp\.(array_typed|ravel|shape|to_list)\(/g, "$1(");
  dfSource = dfSource.replace(/^use warp[ \t]*$/m, "");
  if (/warp\./.test(dfSource)) throw new Error("unresolved warp references remain");

  const warpProgram = `${warpSource}
fn main {
    a := array([1, 2, 3, 4, 5, 6], [2, 3])
    b := array([10, 20, 30], [3])
    say(json.stringify(to_list(add(a, b))))
    say(json.stringify(to_list(mul(a, 2))))
    say(json.stringify(to_list(sum_axis(a, 0))))
    say(json.stringify(to_list(mean_axis(a, 1))))
    m1 := array([1, 2, 3, 4], [2, 2])
    m2 := array([5, 6, 7, 8], [2, 2])
    say(json.stringify(to_list(matmul(m1, m2))))
    say(sum(a))
    say(mean(a))
    say(storage_kind(a))
    say(dtype(array_typed([1, 2, 3], [3], "int64")))
    say(json.stringify(to_list(arange(0, 5, null))))
    wide := broadcast_to(array([1, 2, 3], [3]), [2, 3])
    say(json.stringify(shape(wide)))
    say(json.stringify(to_list(wide)))
    say(tensor.supported())
    say(is_err(tensor.from_list("float64", [2], [1.0, 2.0])))
}
`;

  const dataframeProgram = `${warpSource}
${dfSource}
fn main {
    t := from_rows([
        {"name": "Alice", "age": 30, "dept": "eng", "salary": 90000},
        {"name": "Bob", "age": 25, "dept": "eng", "salary": 75000},
        {"name": "Carol", "age": 35, "dept": "sales", "salary": 85000},
        {"name": "Dan", "age": 28, "dept": "sales", "salary": 70000},
        {"name": "Eve", "age": 32, "dept": "eng", "salary": 95000},
    ])
    eng := filter_(t, fn(r) { r.dept == "eng" })
    say(nrows(eng))
    say(json.stringify(df_col(sort_by(t, "salary", true), "name")))
    g := group_by(t, "dept", {
        "total_salary": {"col": "salary", "op": "sum"},
        "avg_age": {"col": "age", "op": "mean"},
        "headcount": {"col": "name", "op": "count"},
    })
    say(json.stringify(to_records(g)))
    left := from_rows([{"id": 1, "name": "a"}, {"id": 2, "name": "b"}, {"id": 3, "name": "c"}])
    right := from_rows([{"id": 1, "score": 90}, {"id": 3, "score": 80}])
    say(json.stringify(to_records(join(left, right, "id", "inner"))))
    say(json.stringify(to_records(join(left, right, "id", "left"))))
    packed := to_warp(t, ["age", "salary"], "float64")
    say(packed.ok)
    say(json.stringify(shape(packed.value)))
    say(json.stringify(to_list(ravel(packed.value))))
    say(storage_kind(packed.value))
    restored := from_warp(packed.value, ["age", "salary"])
    say(restored.ok)
    say(json.stringify(df_col(restored.value, "age")))
    d := df_describe(t, "salary")
    say(d.count)
    say(d.sum)
    say(d.min)
    say(d.max)
}
`;

  const scaleProgram = `${warpSource}
${dfSource}
fn main {
    let mut rows = []
    let mut i = 0
    while i < 10000 {
        rows = push(rows, {"g": i % 10, "v": i})
        i = i + 1
    }
    t := from_rows(rows)
    g := group_by(t, "g", {"total": {"col": "v", "op": "sum"}, "n": {"col": "v", "op": "count"}})
    s := sort_by(g, "total", true)
    say(nrows(t))
    say(json.stringify(to_records(s)))
}
`;

  const capabilityProgram = `
fn main {
    say(accelerator.supported())
    say(accelerator.load("/native/plugin.so"))
    say(tensor.supported())
    say(tensor.matmul(1, 2))
}
`;

  const run = async (source) => {
    const started = Date.now();
    const result = await runtime.runAsync(source, { timeoutMs: 30000 });
    return { output: result.output, error: result.error, ms: Date.now() - started };
  };

  return {
    warp: await run(warpProgram),
    dataframe: await run(dataframeProgram),
    scale: await run(scaleProgram),
    capability: await run(capabilityProgram),
  };
}

const EXPECTED_WARP_OUTPUT = [
  "[11,22,33,14,25,36]", // broadcast add
  "[2,4,6,8,10,12]", // scalar multiply
  "[5,7,9]", // sum over axis 0
  "[2,5]", // mean over axis 1
  "[19,22,43,50]", // matmul
  "21",
  "3.5",
  "list", // host tensor storage is unavailable: CPU list fallback
  "int64",
  "[0,1,2,3,4]",
  "[2,3]",
  "[1,2,3,1,2,3]",
  "false", // tensor.supported()
  "true", // tensor.from_list returns Err
  "",
].join("\n");

const EXPECTED_DATAFRAME_OUTPUT = [
  "3", // eng rows after filter
  '["Eve","Alice","Carol","Bob","Dan"]', // sort by salary desc
  '[{"avg_age":29,"dept":"eng","headcount":3,"total_salary":260000},{"avg_age":31.5,"dept":"sales","headcount":2,"total_salary":155000}]',
  '[{"id":1,"name":"a","score":90},{"id":3,"name":"c","score":80}]', // inner join
  '[{"id":1,"name":"a","score":90},{"id":2,"name":"b","score":null},{"id":3,"name":"c","score":80}]', // left join
  "true", // to_warp ok
  "[5,2]",
  "[30,90000,25,75000,35,85000,28,70000,32,95000]",
  "list", // interchange array also uses list storage in the browser
  "true", // from_warp ok
  "[30,25,35,28,32]",
  "5",
  "415000",
  "70000",
  "95000",
  "",
].join("\n");

const EXPECTED_SCALE_OUTPUT = [
  "10000",
  '[{"g":9,"n":1000,"total":5004000},{"g":8,"n":1000,"total":5003000},{"g":7,"n":1000,"total":5002000},{"g":6,"n":1000,"total":5001000},{"g":5,"n":1000,"total":5000000},{"g":4,"n":1000,"total":4999000},{"g":3,"n":1000,"total":4998000},{"g":2,"n":1000,"total":4997000},{"g":1,"n":1000,"total":4996000},{"g":0,"n":1000,"total":4995000}]',
  "",
].join("\n");

async function checkBrowser(name, browserType, baseURL) {
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
    const result = await page.evaluate(runSuiteInPage);

    assert.deepStrictEqual(
      { output: result.warp.output, error: result.warp.error },
      { output: EXPECTED_WARP_OUTPUT, error: null },
    );
    assert.deepStrictEqual(
      { output: result.dataframe.output, error: result.dataframe.error },
      { output: EXPECTED_DATAFRAME_OUTPUT, error: null },
    );

    // Scale smoke: 10k-row group_by + sort. The 30s runAsync ceiling is the
    // only hard budget; anything well under it is healthy, so flag drift with
    // a generous 30s wall-clock check too (never flaky on a passing run).
    assert.strictEqual(result.scale.error, null);
    assert.strictEqual(result.scale.output, EXPECTED_SCALE_OUTPUT);
    assert.ok(result.scale.ms < 30000, `scale smoke took ${result.scale.ms}ms`);

    const capabilityLines = result.capability.output.trim().split("\n");
    assert.strictEqual(result.capability.error, null);
    assert.strictEqual(capabilityLines[0], "false"); // accelerator.supported()
    assert.match(capabilityLines[1], /native accelerator plugins are unavailable in browser WASM/);
    assert.strictEqual(capabilityLines[2], "false"); // tensor.supported()
    assert.match(capabilityLines[3], /host tensor package is unavailable in browser WASM/);

    assert.deepStrictEqual(pageErrors, []);
    console.log(
      `${name}: browser warp/dataframe checks passed ` +
      `(warp ${result.warp.ms}ms, dataframe ${result.dataframe.ms}ms, scale ${result.scale.ms}ms)`,
    );
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
