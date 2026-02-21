const fs = require("fs");
const path = require("path");

const targetDir = path.join(
  __dirname,
  "..",
  "node_modules",
  "html5-qrcode",
  "esm"
);

function walk(dir, files = []) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(full, files);
      continue;
    }
    if (entry.isFile() && entry.name.endsWith(".js")) {
      files.push(full);
    }
  }
  return files;
}

function main() {
  if (!fs.existsSync(targetDir)) {
    process.exit(0);
  }
  const jsFiles = walk(targetDir);
  let changed = 0;
  for (const file of jsFiles) {
    const original = fs.readFileSync(file, "utf8");
    const next = original
      .split(/\r?\n/)
      .filter((line) => !line.includes("sourceMappingURL="))
      .join("\n");
    if (next !== original) {
      fs.writeFileSync(file, next, "utf8");
      changed += 1;
    }
  }
  if (changed > 0) {
    console.log(`[fix-html5-qrcode-sourcemaps] patched ${changed} files`);
  }
}

main();
