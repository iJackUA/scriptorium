# UI verification

Use `make headless` to serve the interface for a verification session. It
prints the loopback URL first; open that URL in `agent-browser`. The production
desktop window is not browser-automated: its WKWebView has no DevTools
endpoint, while the headless twin serves the same handler tree as ordinary
HTML over HTTP.

Use `httptest` or `curl` for the default case: handler responses, fragments,
status codes, and Host or Origin behaviour. Those checks are cheaper and more
direct than a browser. Use `agent-browser` only for what HTTP cannot observe:
an htmx target actually swaps in the DOM, a server-sent event lands in its
intended region, or a human needs a screenshot to judge the appearance.

An `agent-browser` session has this shape (replace the URL with the line from
the headless target):

```sh
agent-browser open http://127.0.0.1:PORT
agent-browser snapshot
agent-browser screenshot .verification/library.png
```

Use the snapshot's references to perform any interaction, then take a fresh
snapshot and a screenshot of the resulting state. Keep screenshots under
`.verification/`; they are deliberately not versioned. A ticket that renders
anything is not done until its verification session and findings appear in the
ticket's `## Comments`.

## Legibility contract

Write semantics first: real headings, tables for tabular data, labelled form
controls, and actual `<button>` elements. Mark server-sent-event update
regions with `aria-live`. Use `data-testid` only when semantics cannot identify
the target — normally Status badges and progress regions.
