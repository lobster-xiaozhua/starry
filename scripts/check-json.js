#!/usr/bin/env node
/**
 * 静态配置守卫：确保仓库内的 JSON 文件都可被解析。
 *
 * 起因：frontend/Dockerfile 曾用 `sed` 删除 services/* workspace 行，留下尾随逗号，
 * 于是镜像构建到 `npm install` 才抛 EJSONPARSE——反馈周期长达数分钟。
 * 这类「结构性损坏」应该在一秒内能验出来，而不是等到 CI 的构建步骤。
 *
 * 用法：node scripts/check-json.js
 */
const fs = require("fs");
const path = require("path");

const ROOT = path.resolve(__dirname, "..");
const SKIP_DIRS = new Set([
  "node_modules",
  ".git",
  "dist",
  "build",
  "coverage",
  "backups",
  ".next",
  ".turbo",
  ".cache",
]);

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue;
      walk(path.join(dir, entry.name), out);
    } else if (entry.name.endsWith(".json")) {
      out.push(path.join(dir, entry.name));
    }
  }
  return out;
}

// package-lock.json 由 npm 生成且有专门校验，这里跳过以免噪声。
const files = walk(ROOT).filter((f) => !f.endsWith("package-lock.json"));

const bad = [];
for (const f of files) {
  try {
    JSON.parse(fs.readFileSync(f, "utf8"));
  } catch (e) {
    bad.push(`${path.relative(ROOT, f)}: ${e.message}`);
  }
}

if (bad.length > 0) {
  console.error(`配置文件解析失败 ${bad.length} 个：`);
  for (const line of bad) console.error("  - " + line);
  process.exit(1);
}

console.log(`JSON 配置检查通过：${files.length} 个文件可正常解析。`);
