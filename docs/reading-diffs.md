# Reading Diffs

A reading diff is a patch with the boring parts taken out. Import churn,
error-message prose, generated files, and the fifth identical call-site edit
all disappear; the rows that change behavior stay. In the command center a diff
message opens abridged, with every hidden region one click from view.

The idea comes from [meat](https://github.com/boldsoftware/meat), which frames
it well: models write good code now, so reviewing for nil-checks and import
ordering wastes the attention you need for concepts, algorithms, and
architecture. Substrate's implementation is its own, and smaller.

## The guarantee

A language model chooses what to hide. That is only safe because the model
cannot write the output.

The model never sees a blank page. It receives the patch with a line-number
gutter and must answer in **coordinates** into that patch:

| Operation | What it does |
|---|---|
| `remove` | Drop an inclusive range of lines entirely. |
| `fold` | Collapse two or more contiguous rows into one `...` row. |
| `replace` | Elide part of a single row, marking what was cut. |

A compiler in `internal/readingdiff` validates every coordinate and applies the
plan itself. The model supplies numbers; the bytes come from the input. So the
result is provably the input minus checked deletions, and no plan can introduce
an identifier, a comment, or a behavior the patch did not contain.

Three rules do most of the work:

- A **fold** takes only coordinates. The compiler derives the marker and
  indentation from the first row it replaces and emits a fixed `...`. The model
  cannot choose that text, so a fold can never summarize what it hid.
- A **replace** must be an *elision projection*: characters may only be
  removed, and every removed span must be visibly marked. `t.Errorf("route ID
  = %d, want %d", a, b)` may become `t.Errorf(...)`. It may not become
  `t.Errorf("bad id")`, because that invents prose, nor `t.Errorf(a, b)`,
  because that drops characters silently.
- Coordinates address the **original** patch and never shift as earlier rows
  disappear. A plan is absolute rather than a sequence of edits, so validating
  it needs no simulation.

`internal/readingdiff/property_test.go` asserts this over generated patches and
plans: every rendered row must be derivable from an original row, in order, by
deletion alone.

## Imports are removed mechanically

The compiler strips imports itself rather than asking for them. An empty plan
still yields an import-free diff.

Recognition runs **per side of each hunk**. The old-side rows are scanned
independently of the new-side rows, so swapping `math/rand` for `crypto/rand`
hides both halves; scanning the merged hunk would hide the addition and leave
the deletion, making a substitution read as a removal. An unchanged context row
is hidden only when both sides agree it is scaffolding.

Go, Python, JavaScript and TypeScript, and Rust are recognized, including
grouped blocks, aliases, and multi-line member lists. Files with an unknown
extension keep every row, because guessing import syntax risks hiding real
code — the failure that actually costs the reader something.

The recognizers are deliberately conservative, and one Go rule is worth
knowing. A bare quoted string on its own line is treated as an import member
only when the path has no spaces, carries no trailing comma, and is not
preceded by a keyword. Without the keyword check, `return ""` reads as an
aliased import of the empty path and vanishes from the diff.

## Expanding what was hidden

Because a reading diff is a projection, the server can say exactly which
original lines it hid. Each response carries a **segment map** tiling every
line of the input:

```json
{"kind": "kept",    "start_line": 1,  "end_line": 23}
{"kind": "removed", "start_line": 24, "end_line": 31}
{"kind": "folded",  "start_line": 37, "end_line": 44}
```

The viewer walks that map alongside the abridged text. Opening a region splices
the original lines back into the patch and re-renders, so nothing is refetched
and nothing needs a second abridgement.

Each fold owns its own segment even when two folds touch. One fold emits
exactly one `...` row, and the viewer relies on that correspondence to stay in
step; merging adjacent folds would shift every block after them.

## What you see

`web/frontend/src/components/reviews/ReadingDiffView.tsx` renders through the
ordinary `DiffViewer`, so a reading diff has the same syntax highlighting, file
sidebar, and unified/split toggle as a full one. Above the diff sit the
one-line summary, the retention manifest, and a chip per elided region.

The manifest — `kept 31/40 changed lines in 1/1 files` — is counted by the
server from the compiled result, never taken from the model's own claim about
how much it removed.

## Cost and caching

Abridging runs an agent over the whole patch, so it costs tokens and takes up
to four minutes. Results live in the `reading_diffs` table, keyed by

```
sha256(rubric_hash ‖ model ‖ patch)
```

Every input that shapes the answer is in the key, so a hit is always a result
the current code would reproduce. Editing the rubric or switching models misses
and recomputes instead of serving output the prompt would no longer produce.

Two details keep the cache honest:

- The patch is **trimmed before hashing**. The web client trims the text it
  slices out of a message body while a CLI keeps the trailing newline, and
  hashing raw bytes made those two spellings miss each other and pay twice.
- The whole statistics record is stored, not just the headline counters.
  Persisting four of the seven left a cache hit reporting zero folds where a
  miss reported three.

`Service.Get` also collapses concurrent requests for the same patch, so opening
one diff in two browser tabs runs one agent rather than two.

## When the plan is rejected

Strict validation and a fallible generator coexist through a feedback loop. A
rejected plan is not a failure: the compiler's error names the offending
coordinate, `Abridge` hands it back, and the generator gets another attempt
(three by default). If every attempt is rejected, the identity plan still
strips imports and prunes emptied sections, because a slightly noisy reading
diff beats an error.

## The agent

Plan generation lives in `internal/readingdiff/plangen`, separate from the
compiler so the guarantee above carries no model dependency and can be tested
on its own.

The agent runs with the repository as its working directory under a
**default-deny** permission policy: only `Read`, `Grep`, `Glob`, and `LS`, all
confined to the repo root. Denying by default rather than blocking known-bad
tools means a tool a future SDK adds is refused without this code knowing it
exists.

Setting sources and skills are disabled, which stops the operator's own hooks
from running inside what should be a read-only analysis — including the Stop
hook, which would otherwise hold the subprocess open for minutes after the plan
is ready.

Isolating the agent's **config directory** is off by default. Relocating that
directory also relocates where the CLI looks for credentials, so the agent
fails with "Not logged in" on a machine whose login lives in the system
keychain. Turn it on with `Config.IsolateConfigDir` when credentials come from
the environment instead.

## API

```
POST /api/v1/reading-diff
{"patch": "<unified diff>", "repo_path": "/path/to/repo"}
```

The patch travels in the body because a diff routinely runs to hundreds of
kilobytes and is the cache key rather than a resource identifier. `repo_path`
is optional; without it the agent judges from the diff text alone and every
file tool is denied.

```json
{
  "reading_diff": "diff --git ...",
  "summary": "Dial the gateway over the IPv4 loopback.",
  "elision": "kept 31/40 changed lines in 1/1 files",
  "segments": [{"kind": "kept", "start_line": 1, "end_line": 23}],
  "stats": {"raw_changed": 40, "visible_changed": 31, "fold_count": 1}
}
```

A daemon started without a generator answers `503`. A patch the compiler
refuses answers `422` with the reason.

The reading diff keeps unified-diff shape so an ordinary viewer can colour it,
but **it is not an applicable patch**: hidden rows leave the hunk headers' line
counts deliberately stale.

## Limits

Patches over 400 KB are refused with advice to narrow the range, rather than
failing deep in a run when the model's context fills.

Combined diffs (`diff --cc`, `@@@`) are unsupported; use a first-parent or
two-tree diff.

Coverage is about 89% for the compiler and 41% for `plangen`. The lower figure
is deliberate rather than neglected: plan parsing, the permission policy, and
rubric hashing are covered, while the SDK client wiring needs a live agent and
is exercised end to end instead.

## Working on it

```bash
make unit pkg=./internal/readingdiff          # compiler and service
make unit pkg=./internal/web case=TestReadingDiff   # HTTP surface
```

`cmd/readingdiff-seed` compiles a patch with a hand-written JSON plan and
writes the result straight into the cache. It makes UI work fast and
deterministic, and doubles as a way to reproduce a specific abridgement by hand
when investigating one the generator produced:

```bash
readingdiff-seed -db ~/.subtrate/subtrate.db \
    -patch /tmp/change.diff -plan /tmp/plan.json
```

Changing the rubric or the plan format means bumping `protocolVersion` in
`internal/readingdiff/plangen/rubric.go`. `RubricHash` covers the prompt text,
so every cached entry invalidates on its own once that changes.
