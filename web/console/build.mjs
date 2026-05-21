import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import * as esbuild from "esbuild";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const result = await esbuild.build({
  entryPoints: [path.join(root, "web/console/src/main.jsx")],
  bundle: true,
  format: "iife",
  minify: true,
  outdir: path.join(root, "internal/daemon/.console-build"),
  write: false,
  loader: {
    ".css": "css",
  },
});

const js = result.outputFiles.find((file) => file.path.endsWith(".js"))?.text ?? "";
const css = result.outputFiles.find((file) => file.path.endsWith(".css"))?.text ?? "";

if (!js || !css) {
  throw new Error("console build did not produce both JavaScript and CSS output");
}

const html = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ACPA Console</title>
  <style>${css}</style>
</head>
<body>
  <div id="root"></div>
  <script>${js}</script>
</body>
</html>
`;

const outPath = path.join(root, "internal/daemon/console.html");
await mkdir(path.dirname(outPath), { recursive: true });
await writeFile(outPath, html);
console.log(`built ${path.relative(root, outPath)} (${Buffer.byteLength(html)} bytes)`);
