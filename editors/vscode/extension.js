// Weft VS Code extension — syntax + Language Server (`weft lsp`) + DAP debugger.
"use strict";

const vscode = require("vscode");
const { LanguageClient, TransportKind, RevealOutputChannelOn } = require("vscode-languageclient/node");

/** @type {import("vscode-languageclient/node").LanguageClient | undefined} */
let client;
/** @type {vscode.OutputChannel | undefined} */
let output;

function getOutput() {
  if (!output) {
    output = vscode.window.createOutputChannel("Weft Language Server");
  }
  return output;
}

function serverCommand() {
  const cfg = vscode.workspace.getConfiguration("weft");
  const bin = (cfg.get("lspPath") || "weft").trim() || "weft";
  /** @type {string[]} */
  let args = cfg.get("lspArgs") || ["lsp"];
  if (!Array.isArray(args) || args.length === 0) {
    args = ["lsp"];
  }
  return { bin, args };
}

/**
 * @param {vscode.ExtensionContext} context
 */
async function startClient(context) {
  const { bin, args } = serverCommand();
  const log = getOutput();
  log.appendLine(`Starting: ${bin} ${args.join(" ")}`);

  const serverOptions = {
    run: { command: bin, args, transport: TransportKind.stdio },
    debug: { command: bin, args, transport: TransportKind.stdio },
  };

  const clientOptions = {
    documentSelector: [
      { scheme: "file", language: "weft" },
      { scheme: "untitled", language: "weft" },
    ],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher("**/*.{weft,loom}"),
    },
    outputChannel: log,
    revealOutputChannelOn: RevealOutputChannelOn.Error,
    middleware: {},
  };

  client = new LanguageClient("weft", "Weft Language Server", serverOptions, clientOptions);

  try {
    await client.start();
    log.appendLine("Language server started.");
  } catch (err) {
    const msg = err && err.message ? err.message : String(err);
    log.appendLine("Failed to start weft lsp: " + msg);
    vscode.window
      .showErrorMessage(
        `Weft LSP failed to start (${bin}). Is \`weft\` on PATH? Set weft.lspPath if needed.`,
        "Show Output",
        "Open Settings"
      )
      .then((choice) => {
        if (choice === "Show Output") {
          log.show(true);
        } else if (choice === "Open Settings") {
          vscode.commands.executeCommand("workbench.action.openSettings", "weft.lspPath");
        }
      });
    throw err;
  }
}

/**
 * Debug adapter: spawn `weft debug --dap` over stdio.
 * @implements {vscode.DebugAdapterDescriptorFactory}
 */
class WeftDebugAdapterFactory {
  /**
   * @param {vscode.DebugSession} session
   * @returns {vscode.ProviderResult<vscode.DebugAdapterDescriptor>}
   */
  createDebugAdapterDescriptor(session) {
    const launchBin = session.configuration && session.configuration.weftPath;
    const cfg = vscode.workspace.getConfiguration("weft");
    const bin = (launchBin || cfg.get("lspPath") || "weft").toString().trim() || "weft";
    return new vscode.DebugAdapterExecutable(bin, ["debug", "--dap"]);
  }
}

/**
 * @param {vscode.ExtensionContext} context
 */
async function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand("weft.restartLsp", async () => {
      if (client) {
        try {
          await client.stop();
        } catch (_) {
          /* ignore */
        }
        client = undefined;
      }
      await startClient(context);
      vscode.window.setStatusBarMessage("Weft LSP restarted", 3000);
    }),
    vscode.commands.registerCommand("weft.showLspOutput", () => {
      getOutput().show(true);
    })
  );

  // DAP: weft debug --dap
  context.subscriptions.push(
    vscode.debug.registerDebugAdapterDescriptorFactory("weft", new WeftDebugAdapterFactory())
  );

  // Restart when config changes
  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration(async (e) => {
      if (e.affectsConfiguration("weft.lspPath") || e.affectsConfiguration("weft.lspArgs")) {
        if (client) {
          try {
            await client.stop();
          } catch (_) {
            /* ignore */
          }
          client = undefined;
        }
        try {
          await startClient(context);
        } catch (_) {
          /* shown above */
        }
      }
    })
  );

  try {
    await startClient(context);
  } catch (_) {
    /* error UI already shown */
  }
}

async function deactivate() {
  if (!client) {
    return undefined;
  }
  const c = client;
  client = undefined;
  return c.stop();
}

module.exports = { activate, deactivate };
