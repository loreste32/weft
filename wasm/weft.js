/**
 * Weft browser runtime — loads the Go GOOS=js GOARCH=wasm interpreter.
 *
 * Usage:
 *   <script src="wasm_exec.js"></script>
 *   <script src="weft.js"></script>
 *   const weft = await Weft.load("/wasm/weft.wasm");
 *   const result = await weft.runAsync('fn main { say("hi") }');
 */
(function (root) {
  "use strict";

  const DEFAULT_TIMEOUT_MS = 5000;
  const MAX_TIMEOUT_MS = 30000;
  const LOAD_TIMEOUT_MS = 15000;
  let activeLoad = null;

  /** @typedef {{ output: string, error: string|null }} RunResult */

  function result(output, error) {
    return {
      output: output == null ? "" : String(output),
      error: error == null || error === "" || String(error) === "null"
        ? null
        : String(error),
    };
  }

  function normalizeResult(value) {
    if (!value) return result("", "runWeft returned empty");
    if (typeof value.then === "function") {
      return result("", "run() received an asynchronous result; use runAsync()");
    }
    return result(value.output, value.error);
  }

  function timeoutFromOptions(options) {
    if (options == null) return DEFAULT_TIMEOUT_MS;
    if (typeof options !== "object") {
      throw new Error("options must be an object");
    }
    const value = options.timeoutMs;
    if (value === undefined) return DEFAULT_TIMEOUT_MS;
    if (!Number.isFinite(value) || !Number.isInteger(value) ||
        value < 1 || value > MAX_TIMEOUT_MS) {
      throw new Error(
        "timeoutMs must be an integer from 1 to " + MAX_TIMEOUT_MS + " milliseconds"
      );
    }
    return value;
  }

  function validateCode(code) {
    if (typeof code !== "string") return "code must be a string";
    if (new TextEncoder().encode(code).length > 100000) {
      return "code too large (max 100000 bytes)";
    }
    return null;
  }

  class WeftRuntime {
    constructor(runError) {
      this.ready = true;
      this._runError = runError || null;
    }

    /**
     * Run source synchronously. Prefer runAsync() in UI code or for I/O.
     * @param {string} code
     * @param {{ timeoutMs?: number }} [options]
     * @returns {RunResult}
     */
    run(code, options) {
      const codeError = validateCode(code);
      if (codeError) return result("", codeError);
      let timeoutMs;
      try {
        timeoutMs = timeoutFromOptions(options);
      } catch (error) {
        return result("", error.message || String(error));
      }
      const loadError = typeof this._runError === "function" ? this._runError() : this._runError;
      if (loadError) return result("", loadError);
      if (typeof globalThis.runWeft !== "function") {
        return result("", "Weft Wasm not initialized (runWeft missing)");
      }
      try {
        return normalizeResult(globalThis.runWeft(code, timeoutMs));
      } catch (error) {
        return result("", error && error.message ? error.message : error);
      }
    }

    /**
     * Run source without blocking the browser's JavaScript call stack.
     * @param {string} code
     * @param {{ timeoutMs?: number }} [options]
     * @returns {Promise<RunResult>}
     */
    runAsync(code, options) {
      const codeError = validateCode(code);
      if (codeError) return Promise.resolve(result("", codeError));
      let timeoutMs;
      try {
        timeoutMs = timeoutFromOptions(options);
      } catch (error) {
        return Promise.resolve(result("", error.message || String(error)));
      }
      const loadError = typeof this._runError === "function" ? this._runError() : this._runError;
      if (loadError) return Promise.resolve(result("", loadError));
      const fn = globalThis.runWeftAsync || globalThis.runWeft;
      if (typeof fn !== "function") {
        return Promise.resolve(result("", "Weft Wasm not initialized (runWeft missing)"));
      }
      return Promise.resolve()
        .then(function () { return fn(code, timeoutMs); })
        .then(normalizeResult)
        .catch(function (error) {
          return result("", error && error.message ? error.message : error);
        });
    }

    /** @returns {string} */
    version() {
      if (typeof globalThis.weftVersion === "string") return globalThis.weftVersion;
      if (globalThis.weftVersion != null) return String(globalThis.weftVersion);
      return "unknown";
    }

    /** @returns {object} */
    capabilities() {
      const value = globalThis.weftCapabilities;
      if (!value || typeof value !== "object") {
        return { core: true, async: true, browser: true };
      }
      return value;
    }
  }

  async function fetchWasm(wasmURL) {
    if (typeof fetch !== "function") throw new Error("fetch is not available");
    const response = await fetch(wasmURL);
    if (!response || !response.ok) {
      throw new Error("failed to fetch " + wasmURL + ": " + (response && response.status));
    }
    return response;
  }

  async function instantiate(response, wasmURL, importObject) {
    if (typeof WebAssembly === "undefined") {
      throw new Error("WebAssembly is not available in this browser");
    }

    // Keep an untouched response for the fallback. instantiateStreaming fails
    // on servers that return application/octet-stream instead of application/wasm.
    let fallback = null;
    if (typeof response.clone === "function") {
      try { fallback = response.clone(); } catch (_) { fallback = null; }
    }
    if (typeof WebAssembly.instantiateStreaming === "function") {
      try {
        return await WebAssembly.instantiateStreaming(response, importObject);
      } catch (_) {
        // Fall through to the array-buffer path below.
      }
    }

    if (!fallback) fallback = await fetchWasm(wasmURL);
    return WebAssembly.instantiate(await fallback.arrayBuffer(), importObject);
  }

  async function waitForRuntime(previousRunWeft, runError) {
    const deadline = Date.now() + LOAD_TIMEOUT_MS;
    while (typeof globalThis.runWeft !== "function" ||
           (previousRunWeft && globalThis.runWeft === previousRunWeft)) {
      if (runError.value) throw runError.value;
      if (Date.now() >= deadline) {
        throw new Error("timeout waiting for Weft Wasm runtime");
      }
      await new Promise(function (resolve) { setTimeout(resolve, 10); });
    }
  }

  async function loadOnce(wasmURL) {
    if (typeof Go !== "function") {
      throw new Error("wasm_exec.js not loaded — include it before weft.js");
    }
    const previousRunWeft = globalThis.runWeft;
    const go = new Go();
    const response = await fetchWasm(wasmURL);
    const instanceResult = await instantiate(response, wasmURL, go.importObject);
    const instance = instanceResult.instance || instanceResult;
    const runError = { value: null };

    let runPromise;
    try {
      runPromise = go.run(instance);
    } catch (error) {
      runError.value = error;
    }
    if (runPromise && typeof runPromise.catch === "function") {
      runPromise.catch(function (error) { runError.value = error; });
    }
    await waitForRuntime(previousRunWeft, runError);
    return new WeftRuntime(function () { return runError.value; });
  }

  /**
   * Load and initialize the interpreter. Concurrent calls for the same URL
   * share one initialization; a failed load is removed from the cache.
   * @param {string} wasmURL
   * @returns {Promise<WeftRuntime>}
   */
  function load(wasmURL) {
    if (typeof wasmURL !== "string" || wasmURL === "") {
      return Promise.reject(new Error("wasmURL must be a non-empty string"));
    }
    if (activeLoad && activeLoad.url === wasmURL) return activeLoad.promise;
    const promise = loadOnce(wasmURL);
    activeLoad = { url: wasmURL, promise: promise };
    promise.catch(function () {
      if (activeLoad && activeLoad.promise === promise) activeLoad = null;
    });
    return promise;
  }

  const Weft = { load: load, WeftRuntime: WeftRuntime };
  if (typeof module !== "undefined" && module.exports) module.exports = Weft;
  root.Weft = Weft;
})(typeof self !== "undefined" ? self : this);
