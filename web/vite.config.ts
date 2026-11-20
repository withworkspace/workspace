import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { basename, join, relative } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";
import { shellStyles } from "./src/plugins/tokens.ts";

// scopedName is a CSS module's class as the page has it: the file's name,
// the class's, and a hash of where the file is, which keeps two files of
// one name apart (Runs.module.css's .row is Runs_row_x1y2z).
function scopedName(local: string, file: string): string {
  const from = relative(import.meta.dirname, file.split("?")[0]);
  const hash = createHash("sha256").update(from).digest("base64url").slice(0, 5);
  return `${basename(from, ".module.css")}_${local}_${hash}`;
}

// `just web-dev <workspace>` runs this dev server against that workspace's
// running web UI (ws web starts it): the API is proxied there, signed in
// with the workspace's token, so the page here needs no sign-in.
function devTarget(): { origin: string; token: string } | undefined {
  const root = process.env.WS_ROOT;
  if (!root) return undefined;
  try {
    const info = JSON.parse(readFileSync(join(root, ".ws", "web.json"), "utf8"));
    if (info.port) return { origin: `http://127.0.0.1:${info.port}`, token: info.token };
  } catch {
    // no web UI running there yet
  }
  return undefined;
}

// KaTeX's styles name each of its fonts three times over (woff2, woff,
// ttf). Every browser this runs in reads woff2, so only those are built
// in: the binary carries the web UI.
function katexFonts(): Plugin {
  return {
    name: "katex-woff2-only",
    enforce: "pre",
    transform(code, id) {
      if (!/katex[^/]*\.css/.test(id)) return undefined;
      return code.replace(/,\s*url\([^)]*\.(?:woff|ttf)\)\s*format\("[^"]*"\)/g, "");
    },
  };
}

// internal/web/dist is gitignored but for its .gitkeep, which go:embed
// needs when the web UI hasn't been built. Building empties the folder, so
// the file is put back.
function keepDist(): Plugin {
  return {
    name: "keep-dist",
    apply: "build",
    writeBundle(options) {
      if (options.dir) writeFileSync(join(options.dir, ".gitkeep"), "");
    },
  };
}

// The page ws serves for a plugin's app (the shell) has ws's tokens in a
// <style>, so the app's first paint is in ws's colours with nothing of the
// plugin's having run. This writes what that <style> holds, as
// plugin-shell.css beside the app: ws's foundations, each token under the
// name a plugin's app knows it by, and the shell's own few rules. The
// server reads it from the built app and writes it into every shell
// (internal/web/plugins.go).
function pluginShell(): Plugin {
  const read = (file: string) => readFileSync(new URL(`./src/${file}`, import.meta.url), "utf8");
  return {
    name: "plugin-shell",
    apply: "build",
    generateBundle() {
      this.emitFile({ type: "asset", fileName: "plugin-shell.css", source: shellStyles(read("foundations.css"), read("plugins/shell.css")) });
    },
  };
}

export default defineConfig(() => {
  const target = devTarget();
  return {
    plugins: [react(), katexFonts(), pluginShell(), keepDist()],
    // A component's own styles are a CSS module beside it (Runs.module.css),
    // its classes scoped to it. The scoped name keeps the file's and the
    // class's, so a page's classes still say where they're from in the
    // browser's inspector: Runs_row_x1y2z.
    css: { modules: { generateScopedName: scopedName } },
    // Mermaid's diagram types and layouts are large chunks, loaded only by
    // a diagram that needs them.
    // Fonts are files, never written into the styles: the page's content
    // policy lets it load only what the server serves.
    build: { outDir: "../internal/web/dist", emptyOutDir: true, chunkSizeWarningLimit: 1600, assetsInlineLimit: (file: string) => (/\.woff2?$/.test(file) ? false : undefined) },
    server: {
      port: 5173,
      strictPort: true,
      // The API, and what the server serves for plugins: the shell of a
      // plugin's page, and the plugin's own HTTP.
      proxy:
        target &&
        Object.fromEntries(
          // /plugins/ with its slash: a plugin's own HTTP is under it, and
          // /plugins alone is the web UI's Plugins page.
          ["/api", "/plugin-frame", "/plugins/"].map((path) => [
            path,
            {
              target: target.origin,
              changeOrigin: true,
              headers: { origin: target.origin, authorization: `Bearer ${target.token}` },
            },
          ]),
        ),
    },
  };
});
