import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const projectDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const frontendDir = path.join(projectDir, "frontend");
const replacements = [
  ["Komari Monitor", "PierOps"],
  ["A simple server monitor tool.", "A lightweight operations platform."],
  ["A simple server monitor tool", "A lightweight operations platform"],
];

for (const relativePath of ["index.html", "vite.config.ts"]) {
  const filePath = path.join(frontendDir, relativePath);
  let source = fs.readFileSync(filePath, "utf8");
  for (const [before, after] of replacements) {
    source = source.replaceAll(before, after);
  }
  fs.writeFileSync(filePath, source);
}

