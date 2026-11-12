# Spec: Brute-force program synthesizer

Nov 12, 2026 · @Daan

## Summary

A hobby tool that writes a program from input/output examples. It builds the program as a syntax tree out of a fixed library of known operations, finds the part of the tree that is most wrong, and replaces or mutates that part. Repair is recursive: a broken subtree is treated as its own small synthesis problem, with its own inputs and outputs. The finished tree compiles to Go, Rust, C or other languages.

In research terms this is **programming by example** (inductive program synthesis) with genetic-style search and divide-and-conquer decomposition.

## Goals and non-goals

**Goals**

- Given a set of example input/output pairs, produce a program that maps every input to its output.
- Build programs only from a known, finite library of operations, so every result can be compiled.
- Compile the resulting tree to Go, Rust and C (more back ends later).
- Localise errors to a subtree and repair that subtree in isolation, recursively.
- Handle structured data: objects in, objects out, with per-field decomposition.
- Leave room for a learned model that ranks which mutation to try next.

**Non-goals (for v1)**

- General algorithms such as sorting, search or anything needing unbounded loops or recursion.
- Side effects: I/O, network, mutable global state.
- Proving the program correct beyond the given examples.
- LLM-generated code. Search stays over the operation library; a model may only rank choices.
- Production use. This is a hobby project, so clarity beats speed.

## Core concepts

| Concept | What it is | Notes |
| --- | --- | --- |
| Example set | List of (input, output) pairs | Split into a training set and a held-out set to catch overfitting |
| Type | Int, float, bool, string, list\<T>, record{fields} | Every node is typed; types prune the search hard |
| Operation library | Typed primitives: arithmetic, comparison, string ops, field get/set, map, filter, fold, if/else | The only building blocks; each must have a code template per back end |
| Node | One operation plus its child nodes | Leaves are inputs, constants or field reads |
| Program | A typed syntax tree whose root returns the output type | Kept as a pure expression: no variables reassigned |
| Score | Distance between actual and expected outputs over all examples | Type-specific: numeric difference, edit distance for strings, per-field for records |
| Trace | Recorded value at every node for every example | Basis for error localisation and sub-block I/O |
| Cost | Program size plus a penalty for constants | Among programs that fit, the smallest wins |

Scoring needs a partial-credit metric, not pass/fail. Without it, search has no gradient to follow.

## Synthesis loop

The loop repeats until every training example passes, then checks the held-out set.

&#91;embedded content: synthesis loop · 6 steps, 1 decision\]

The recursive step is the highlighted box: it runs this same loop on a smaller example set.

1. **Seed.** Start from a trivial tree of the right type (identity, a constant, or a field copy), or from the best of N random small trees.
2. **Evaluate.** Run the tree on all examples and record a trace: the value at every node, per example.
3. **Score.** Compare outputs with expected outputs. Stop if all pass.
4. **Localise.** Rank subtrees by blame: how much the score improves if that subtree's output were replaced by the correct value (when it can be derived, see Decomposition). Pick the worst.
5. **Derive sub-problem.** Take the recorded inputs of that subtree and the derived target outputs. That is a new, smaller example set.
6. **Solve recursively.** Run the same loop on the sub-problem, bounded by depth and time. Small enough sub-problems are solved by exhaustive enumeration instead.
7. **Splice and accept.** Put the new subtree in place. Keep it only if the whole program's score improves; otherwise try the next candidate or the next-worst subtree.
8. **Grow.** If no single subtree fix helps after a budget, allow structural moves: wrap a subtree in an if/else, add a node above the root, or split by field.

After every accepted change, run a simplifier that removes dead branches and folds constants, so the tree does not bloat.

## Decomposition

Recording gives a subtree's **inputs** for free, but not its **target outputs**. The trace only shows what the current, wrong code produced. The target has to be derived from the expected output above it. This is the hardest part of the design, so v1 supports four ways.

**1. Field-to-field (records).** When the output is a record, each output field is its own target, already known from the examples. Solve one sub-program per output field, with the whole input record available to each. This is the easiest win and the first milestone.

- Do not restrict a field's program to one input field: `fullName = first + " " + last` needs two.
- Start each field by trying a direct copy or single-op mapping from every input field, then search wider.
- Nested records recurse the same way.

**2. Inverse operations (top-down).** Each operation may declare an inverse, often called a witness function: given the parent's target output and the other children's values, what must this child produce? Examples:

- `a + b = 10` and `a = 3` → target for `b` is 7.
- `concat(a, b) = "John Smith"` and `a = "John"` → target for `b` is `" Smith"`.
- `if c then x else y`: examples where `c` is true give targets for `x`; the rest give targets for `y`.

Operations without an inverse (hash, lossy rounding) cannot pass targets down; subtrees under them are solved as a whole.

**3. Candidate targets.** When no inverse exists, enumerate small sets of plausible intermediate values, solve for each, and keep whichever makes the parent fit. Expensive; a fallback only.

**4. Freeze the bottom, regenerate the top.** Pick a cut point and make the block everything from that point up to the root. The block's output is then the program output, which the examples already give. Its inputs are the recorded values of the subtrees below the cut, which are kept frozen. This turns any cut into a fully specified sub-problem without needing inverses. It assumes the frozen subtrees are correct, the opposite of modes 1–3, which assume everything above the block is correct.

**Consistency check before searching.** A cut can be rejected instantly: if two examples have identical recorded values at the cut but different expected outputs, no block above that cut can ever fit. The frozen values have lost information the output needs, so the cut must move lower.

**Scope widening.** Start with a small block. If it fails the consistency check, or the search hits its time budget without passing (say 99.9% of examples), assume the block is scoped wrong: widen it by pulling the next layer of frozen nodes into the block, and retry. In the limit the block is the whole tree, which is synthesis from scratch. Widening costs search time exponentially, so the consistency check matters: it skips most hopeless scopes without spending the budget.

**Splitting by example.** If no single program fits all examples, cluster the examples, solve each cluster, and synthesise an if/else condition that picks the right branch. This is how conditionals appear.

## Search strategy and learned guidance

Start with plain enumeration and random mutation; add a learned ranker only once there is data to train it on.

**Phase 1: no learning**

- Bottom-up enumeration for small sub-problems: build every typed tree up to size N, deduplicated by observational equivalence (two trees with identical outputs on all examples count as one).
- Mutations for larger trees: replace a subtree, swap an operation for one of the same type signature, change a constant, wrap in if/else, delete a node.
- Population-based search (genetic style) with the cost function as a tie-breaker.

**Phase 2: learned ranker**

- Log every attempted mutation with its context (types, operation, local trace values) and whether it improved the score.
- Train a small model on those logs to predict which operation or mutation is most likely to help next. A gradient-boosted tree or a small neural net is enough.
- Generate extra training data by sampling random programs, running them on random inputs, and asking the system to rediscover them.
- The model only reorders candidates. It never produces code itself, so every output still comes from the operation library.

**Budgets.** Every recursive call has a depth limit, a node limit and a time limit, so one hard sub-problem cannot stall the whole run.

## Code generation

The tree compiles to source by walking it and filling one template per operation per language. Because the tree is a pure, typed expression, this is a straightforward pretty-printer.

- Each library operation ships with a template for every back end, plus a reference interpreter used during search. A conformance test runs every operation through the interpreter and every back end and checks the outputs agree.
- Shared subtrees become named local variables; field-level programs become one function per output field, called from a constructor for the output record.
- Types map per language: records to structs, lists to slices/`Vec`/arrays with a length, strings to `string`/`String`/`char*`.
- **C is the hard one:** strings and lists need memory management. Either restrict the C back end to fixed-size buffers, or generate an arena allocator and free everything at once.
- Emit the examples as a unit test file next to the code, so the generated program proves itself in its own language.

Open question: which language is the tool itself written in? Go is a good fit: fast enough for search, simple to write the interpreter in, and it is already a target.

## Known risks and open problems

The biggest risk is target derivation for inner subtrees; everything else has known mitigations.

| Risk | Why it bites | Mitigation |
| --- | --- | --- |
| No target for inner subtrees | Recording gives inputs, not the outputs a subtree *should* produce | Field-to-field, inverse operations, candidate targets, or freezing the bottom and regenerating the top, with scope widening (see Decomposition) |
| Blame spread across subtrees | Two subtrees can each look fine and be wrong together | Fall back to growing structure; "just add nodes" works only if the loop knows *where* to add them |
| Bloat | Adding nodes to fix errors grows trees that fit by accident | Cost function, simplifier after every change, cap on tree size |
| Overfitting | A tree can match every example and still be the wrong function | Held-out examples; prefer the smallest program; ask for more examples when two different programs both fit |
| Search explosion | Candidate trees grow exponentially with size | Strong typing, observational-equivalence dedup, small sub-problems, budgets |
| Constants | Guessing `3.14159` or `"-"` by mutation is slow | Pull constants from the examples' own values first |
| Back ends disagree | Integer overflow, float rounding and string encoding differ between Go, Rust and C | Conformance tests per operation; pin integer widths and UTF-8 |

## Milestones

Each milestone ends with something that runs; no milestone depends on the learned model until M6.

1. **M1 — Interpreter.** Types, \~20 operations, tree data structure, interpreter, trace recording, scoring.
2. **M2 — Enumerator.** Bottom-up enumeration with type pruning and observational-equivalence dedup. Solves single-value problems up to \~6 nodes, such as `x * 2 + 1`.
3. **M3 — Records.** Field-to-field decomposition. Solves JSON-to-JSON mappings: renames, concatenations, unit conversions.
4. **M4 — Go back end.** Compile trees to Go with generated unit tests. First end-to-end demo.
5. **M5 — Repair loop.** Error localisation, freeze-bottom cuts with the consistency check and scope widening, inverse operations, recursive sub-problems, if/else by example clustering.
6. **M6 — Learned ranker.** Mutation logging, self-generated training data, ranker in the loop. Measure: solve rate and time on a fixed benchmark set, with and without the model.
7. **M7 — Rust and C back ends.** Conformance tests across all three languages.

Build a benchmark set of 30–50 example problems during M1 and keep it fixed. Without it you cannot tell whether a change to search helped.

## Prior art to read

Most pieces of this design have a published, working version; read these before building the matching milestone.

| Work | What to take from it | Milestone |
| --- | --- | --- |
| [Introduction to Program Synthesis, lecture 3](https://people.csail.mit.edu/asolar/SynthesisCourse2020/Lecture3.htm) (Solar-Lezama, MIT) | Free course; bottom-up search and observational equivalence explained step by step | M2 |
| [FlashMeta / PROSE](https://www.microsoft.com/en-us/research/publication/flashmeta-a-framework-for-inductive-program-synthesis/) (Polozov, Gulwani 2015) | Witness functions: inverse semantics that push target outputs down to subexpressions, the answer to "where do sub-block outputs come from" | M5 |
| [Metric program synthesis](https://www.cs.utexas.edu/~isil/metric-synthesis.pdf) (Feser, Dillig, Solar-Lezama) | Groups programs by output *distance*, not equality, then repairs near-misses; close to your measure-the-error idea | M5 |
| [Probe: just-in-time learning for bottom-up synthesis](https://par.nsf.gov/servlets/purl/10252255) (Barke, Peleg, Polikarpova 2020) | A learned model guiding enumeration, trained on partial solutions during the run | M6 |
| [DreamCoder](https://people.csail.mit.edu/asolar/papers/EllisWNSMHCST21.pdf) (Ellis et al., PLDI 2021) | Learns a search policy and grows the operation library from solved problems | M6 and beyond |

Also worth a look: genetic programming (Koza) for mutation and crossover operators, and the SyGuS competition benchmarks as a ready-made test set.
