import { spawn } from "node:child_process";

const args = process.argv.slice(2);
const env = { ...process.env };
delete env.NO_COLOR;

const cmd = process.platform === "win32" ? "bunx.cmd" : "bunx";
const child = spawn(cmd, ["playwright", ...args], {
  env,
  stdio: "inherit",
});

child.on("error", (err) => {
  console.error("failed to start playwright:", err.message);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
