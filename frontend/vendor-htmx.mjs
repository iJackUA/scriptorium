// Copies htmx into the embedded static directory. Assets ship inside the
// binary; nothing is fetched from a CDN or read from disk at runtime.
import { copyFileSync, mkdirSync } from "node:fs";

const dest = "../internal/ui/static";
mkdirSync(dest, { recursive: true });
copyFileSync("node_modules/htmx.org/dist/htmx.min.js", `${dest}/htmx.min.js`);
console.log("vendored htmx.min.js");
