import { spawn } from "node:child_process";

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
        const error = new Error(
          "koment " + args.join(" ") + " exited with code " + code
        );
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
  return {
    "tool.execute.before": async (input, output) => {
      const tool = input.tool;
      if (tool !== "edit" && tool !== "write") return;
      const args = output.args ?? {};
      const filePath = args.filePath ?? args.path ?? args.file ?? "";
      const content = args.content ?? args.newContent ?? args.text ?? "";
      if (!filePath || typeof content !== "string") return;
      const payload = JSON.stringify({
        tool_name: "opencode_edit",
        tool_input: { filePath, content },
      });
      try {
        await run("koment", ["agents", "hook", "pre-tool"], {
          cwd: directory,
          stdin: payload,
        });
      } catch (err) {
        deny(
          "koment pre-tool hook denied edit of " +
            filePath +
            ":\n" +
            (err.stderr || err.message)
        );
      }
    },
    dispose: async () => {
      try {
        await run("koment", ["check"], { cwd: directory });
        await run("koment", ["comments", "check"], { cwd: directory });
        await run("koment", ["agents", "check"], { cwd: directory });
      } catch (err) {
        deny("koment policy gate failed:\n" + (err.stderr || err.message));
      }
    },
  };
};
