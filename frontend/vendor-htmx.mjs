// Copies htmx into the embedded static directory. Assets ship inside the
// binary; nothing is fetched from a CDN or read from disk at runtime.
import { copyFileSync, mkdirSync } from "node:fs";

const dest = "../internal/ui/static";
mkdirSync(dest, { recursive: true });
copyFileSync("node_modules/htmx.org/dist/htmx.min.js", `${dest}/htmx.min.js`);
copyFileSync("node_modules/tom-select/dist/js/tom-select.complete.min.js", `${dest}/tom-select.min.js`);
copyFileSync("node_modules/tom-select/dist/css/tom-select.css", `${dest}/tom-select.css`);
console.log("vendored htmx and Tom Select");
