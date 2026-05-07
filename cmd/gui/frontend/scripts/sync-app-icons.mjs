// Mirror assets/app-*.svg from the repo root into Vite's public/ tree so
// the frontend can serve them at /app-icons/<name>.svg. Source of truth
// stays in assets/; the destination is a build artifact (gitignored).
//
// Wired into npm predev / prebuild so it runs before vite ever serves or
// builds. Cross-platform via fs.cpSync with no external deps.

import { readdirSync, mkdirSync, copyFileSync, existsSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const frontendRoot = resolve(here, "..");
const repoRoot = resolve(frontendRoot, "..", "..", "..");
const sourceDir = join(repoRoot, "assets");
const destDir = join(frontendRoot, "public", "app-icons");

if (!existsSync(sourceDir)) {
  console.error(`[sync-app-icons] source dir missing: ${sourceDir}`);
  process.exit(1);
}

if (existsSync(destDir)) {
  rmSync(destDir, { recursive: true, force: true });
}
mkdirSync(destDir, { recursive: true });

const candidates = readdirSync(sourceDir).filter(
  (name) => name.startsWith("app-") && name.endsWith(".svg"),
);

if (candidates.length === 0) {
  console.warn(
    "[sync-app-icons] no app-*.svg icons found in assets/; bar will fall back to app-default.svg",
  );
}

for (const name of candidates) {
  copyFileSync(join(sourceDir, name), join(destDir, name));
}

console.log(
  `[sync-app-icons] copied ${candidates.length} icon(s) → ${destDir}`,
);
