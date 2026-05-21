import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const html = await readFile(path.join(root, "internal/daemon/console.html"), "utf8");

const required = [
  "ACPA Console",
  "id=\"root\"",
  "<style>",
  "<script>",
  "Start QR Setup",
  "Run Doctor",
];

for (const token of required) {
  if (!html.includes(token)) {
    throw new Error(`console artifact is missing ${token}`);
  }
}

if (/<script[^>]+src=|<link[^>]+href=/i.test(html)) {
  throw new Error("console artifact must not reference external runtime assets");
}

console.log("console artifact check passed");
