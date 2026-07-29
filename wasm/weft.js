/**
 * Weft browser runtime — loads weft.wasm (Go GOOS=js GOARCH=wasm).
 *
 * Usage:
 *   <script src="wasm_exec.js"></script>
 *   <script src="weft.js"></script>
 *   <script>
 *     const weft = await Weft.load('/wasm/weft.wasm');
 *     const { output, error } = weft.run('fn main { say("hi") }');
 *   </script>
 *
 * Requires a global `runWeft` installed by the Wasm module (cmd/weft-wasm).
 */
(function (root) {
  "use strict";

  /**
   * @typedef {{ output: string, error: string|null }} RunResult
   */

  class WeftRuntime {
    /**
     * @param {WebAssembly.Instance} _instance
     */
    constructor() {
      this.ready = true;
    }

    /**
     * Run Weft source in the browser.
     * @param {string} code
     * @param {{ timeoutMs?: number }} [opts]
     * @returns {RunResult}
     */
    run(code, opts) {
      if (typeof globalThis.runWeft !== "function") {
        return { output: "", error: "Weft Wasm not initialized (runWeft missing)" };
      }
      const timeoutMs = (opts && opts.timeoutMs) || 5000;
      try {
        const r = globalThis.runWeft(code, timeoutMs);
        if (!r) {
          return { output: "", error: "runWeft returned empty" };
        }
        // Go js.Value object
        const output = r.output != null ? String(r.output) : "";
        let error = null;
        if (r.error != null && r.error !== "" && String(r.error) !== "null") {
          error = String(r.error);
        }
        return { output, error };
      } catch (e) {
        return { output: "", error: e && e.message ? e.message : String(e) };
      }
    }

    /** @returns {string} */
    version() {
      if (typeof globalThis.weftVersion === "string") {
        return globalThis.weftVersion;
      }
      if (globalThis.weftVersion != null) {
        return String(globalThis.weftVersion);
      }
      return "unknown";
    }
  }

  /**
   * Load weft.wasm. Requires wasm_exec.js (Go) to define global `Go`.
   * @param {string} wasmURL
   * @returns {Promise<WeftRuntime>}
   */
  async function load(wasmURL) {
    if (typeof Go === "undefined") {
      throw new Error("wasm_exec.js not loaded — include it before weft.js");
    }
    const go = new Go();
    const resp = await fetch(wasmURL);
    if (!resp.ok) {
      throw new Error("failed to fetch " + wasmURL + ": " + resp.status);
    }
    const result = await WebAssembly.instantiateStreaming
      ? await WebAssembly.instantiateStreaming(resp, go.importObject)
      : await WebAssembly.instantiate(await resp.arrayBuffer(), go.importObject);
    // Run Go main (installs runWeft); do not await — main blocks on select{}.
    go.run(result.instance);
    // Wait until runWeft is registered
    const deadline = Date.now() + 15000;
    while (typeof globalThis.runWeft !== "function") {
      if (Date.now() > deadline) {
        throw new Error("timeout waiting for runWeft export");
      }
      await new Promise((r) => setTimeout(r, 10));
    }
    return new WeftRuntime();
  }

  const Weft = { load, WeftRuntime };
  if (typeof module !== "undefined" && module.exports) {
    module.exports = Weft;
  }
  root.Weft = Weft;
})(typeof self !== "undefined" ? self : this);
