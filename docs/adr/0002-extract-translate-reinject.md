# Translate extracted text, never the markup

The Agent is never shown the source file's markup. Scriptorium parses the Source File into an ordered list of Text Nodes, sends only plain prose, and splices the translations back into the original document tree. The output is therefore structurally identical to the input by construction, rather than by the model's good behaviour.

The alternative — handing the model fb2 XML and asking for translated XML back — is simpler to build and certain to fail: over hundreds of requests it will eventually drop a tag, lose an attribute, or invent structure, and on a 300-page book "eventually" arrives early.

## Consequences

Splicing by position introduces its own lethal failure mode: if 40 Text Nodes go out and 39 come back, every later translation lands in the wrong slot and the result is a valid file that is quietly scrambled from that point on. The wire format is therefore self-checking — each Text Node carries an explicit index marker that must come back intact, and a Chunk is rejected unless the count and indices match exactly. On rejection: retry once with a stricter instruction, then fall back to one-node-per-request (where misalignment is impossible), then mark the Chunk failed, leave the source text in place, and surface the count in the UI. A gap is never shipped silently.

This also removes a step: because translations fill slots in a document that was never taken apart, there is nothing to reassemble at the end. Joining is not a task, and certainly not one for a model.

Adding epub and docx later means writing a parser that yields Text Nodes and accepts them back — the pipeline behind that boundary does not change. Plain `.txt` is the degenerate case, one Text Node per paragraph.
