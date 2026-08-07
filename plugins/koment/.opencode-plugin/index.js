import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";

function startMCP(directory) {
  const child = spawn("koment", ["mcp", "--write"], {
    cwd: directory,
    env: process.env,
    stdio: ["pipe", "pipe", "pipe"],
  });

  const pending = new Map();
  let nextID = 1;
  let buffer = "";

  child.stderr.on("data", (chunk) => {
    const text = chunk.toString("utf8");
    for (const line of text.split("\n")) {
      if (line) process.stderr.write(`[koment-mcp] ${line}\n`);
    }
  });

  child.stdout.on("data", (chunk) => {
    buffer += chunk.toString("utf8");
    let newline;
    while ((newline = buffer.indexOf("\n")) !== -1) {
      const line = buffer.slice(0, newline);
      buffer = buffer.slice(newline + 1);
      if (!line) continue;
      let message;
      try {
        message = JSON.parse(line);
      } catch {
        continue;
      }
      const id = message.id;
      const waiter = pending.get(id);
      if (!waiter) continue;
      pending.delete(id);
      if (message.error) {
        waiter.reject(new Error(message.error.message || "mcp error"));
      } else {
        waiter.resolve(message.result || {});
      }
    }
  });

  child.on("exit", (code) => {
    for (const waiter of pending.values()) {
      waiter.reject(new Error(`koment mcp exited (code=${code})`));
    }
    pending.clear();
  });

  function request(method, params) {
    const id = nextID++;
    const payload = JSON.stringify({ jsonrpc: "2.0", id, method, params: params || {} }) + "\n";
    return new Promise((resolve, reject) => {
      pending.set(id, { resolve, reject });
      child.stdin.write(payload, (err) => {
        if (err) {
          pending.delete(id);
          reject(err);
        }
      });
    });
  }

  return {
    child,
    request,
    initialize() {
      return request("initialize", {
        protocolVersion: "2024-11-05",
        capabilities: {},
        clientInfo: { name: "@koment/opencode-koment", version: "0.1.0" },
      });
    },
    notify(method, params) {
      const payload = JSON.stringify({ jsonrpc: "2.0", method, params: params || {} }) + "\n";
      child.stdin.write(payload);
    },
    async callTool(name, args) {
      const result = await request("tools/call", { name, arguments: args || {} });
      if (Array.isArray(result.content)) {
        for (const block of result.content) {
          if (block && block.type === "text" && block.text) {
            process.stderr.write(`[koment-mcp:${name}] ${block.text}\n`);
          }
        }
      }
      return result;
    },
    close() {
      try {
        child.stdin.end();
      } catch {
        // swallow: child.stdin may already be closed by an earlier exit
      }
    },
  };
}

function run(cmd, args, { cwd, stdin } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, {
      cwd,
      env: process.env,
      stdio: ["pipe", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => (stdout += chunk.toString("utf8")));
    child.stderr.on("data", (chunk) => (stderr += chunk.toString("utf8")));
    child.on("error", reject);
    if (stdin !== undefined && stdin !== null) {
      child.stdin.end(stdin);
    } else {
      child.stdin.end();
    }
    child.on("close", (code) => {
      if (code === 0) resolve({ stdout, stderr });
      else {
        const error = new Error("koment " + args.join(" ") + " exited with code " + code);
        error.stdout = stdout;
        error.stderr = stderr;
        error.code = code;
        reject(error);
      }
    });
  });
}

function deny(reason) {
  throw new Error(reason);
}

export default async ({ directory }) => {
  const sessionID = randomUUID();
  let mcp;
  try {
    mcp = startMCP(directory);
    await mcp.initialize();
    mcp.notify("notifications/initialized");
  } catch (err) {
    throw new Error(
      `koment plugin failed to start the MCP server in ${directory}:\n` +
        (err && err.message ? err.message : String(err)) +
        "\nInstall the \`koment\` binary on PATH: https://github.com/koment-dev/koment/releases"
    );
  }

  return {
    "tool.execute.before": async (input, output) => {
      const tool = input.tool;
      if (tool !== "edit" && tool !== "write") return;
      const args = output.args ?? {};
      const filePath = args.filePath ?? args.path ?? args.file ?? "";
      const content = args.content ?? args.newContent ?? args.text ?? "";
      if (!filePath || typeof content !== "string") return;
      try {
        const result = await mcp.callTool("koment_pre_tool", {
          tool_name: "opencode_edit",
          filePath,
          content,
        });
        const structured = result.structuredContent || {};
        if (structured.decision === "deny") {
          deny(structured.reason || "koment policy denied this edit");
        }
      } catch (err) {
        deny("koment pre-tool MCP call failed: " + (err && err.message ? err.message : String(err)));
      }
    },
    dispose: async () => {
      const failures = [];
      for (const command of [
        ["check"],
        ["comments", "check"],
        ["agents", "check"],
      ]) {
        try {
          await run("koment", command, { cwd: directory });
        } catch (err) {
          failures.push(
            "koment " + command.join(" ") + ":\n" + (err.stderr || err.message || String(err))
          );
        }
      }
      mcp.close();
      if (failures.length > 0) {
        deny("koment policy gate failed:\n" + failures.join("\n"));
      }
    },
  };
};