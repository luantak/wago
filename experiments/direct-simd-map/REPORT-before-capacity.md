# Direct SIMD scalar maps

**ITERATE. Direct native emission gives repeatable execution gains. This build does not meet the compiler allocation target. Keep it on the experiment branch. Do not promote it yet.**

The experiment uses current main `b00d96f8f0ac75092481d84ffb7cabf01332abc8`. A final fetch returned the same main commit. Branch: `codex/direct-simd-map`. Commit `36a70be93` contains the failing tests. The next commit, `2660962a7`, makes them pass. The final production logic is in `ff1829b18`; later commits add tests and measurement tools. The user's working checkout was not changed.

The normal AMD64 compiler selects four-lane i32 and f32 maps automatically. It extends the existing bounded region planner and emitter. It uses the original Wasm bytes, one scalar compilation, fixed scratch arrays, and the original checked body for fallback and tails. There is no new option, module clone, bytecode rewrite, position tape, SSA, or general IR. Recognition is bounded by 64 operations, 512 bytes, eight locals, and eight loads. This map path further requires one load and one store, both with stride four, a multiply by an invariant, and an optional add. It admits ascending equality-counter loops and decrement-to-zero loops. It rejects unsupported control flow and uncertain local homes. ARM64 code is unchanged.

The CPU gate requires SSE4.1 for i32 packed multiplication. Assembly shows `pmulld` and `paddd`. Float maps use separate `mulps` and `addps`, with no FMA. Both use 128-bit registers. The SIMD body has no stack access. Setup and exit still have substantial stack traffic.

The table uses the six alternating, closed-image samples in [verified-closed](results/verified-closed/). Tests used one pinned Ryzen 7 8845HS CPU core, Go 1.27.1, `GOMAXPROCS=1`, identical inputs, and identical feature masks. Raw results, source hashes, binary hashes, and benchstat output are saved there. Each row has six samples.

| Measurement | Main | Experiment | Change |
|---|---:|---:|---:|
| i32 execution, 8,192 elements | 5.469 µs | 0.622 µs | −88.62% |
| f32 execution, 8,192 elements | 4.037 µs | 0.925 µs | −77.08% |
| Original Yosys routine, 8,192 elements per array | 11.251 µs | 1.842 µs | −83.63% |
| i32 mapped-image compile | 14.29 µs | 16.76 µs | +17.26% |
| f32 mapped-image compile | 14.53 µs | 16.85 µs | +15.99% |
| i32 heap-image compile | 7.563 µs | 10.491 µs | +38.71% |
| f32 heap-image compile | 7.290 µs | 11.319 µs | +55.28% |
| i32 heap-image bytes / allocations | 12,488 / 30 | 14,152 / 32 | +1,664 / +2 |
| f32 heap-image bytes / allocations | 12,520 / 30 | 14,184 / 32 | +1,664 / +2 |
| Rejected heap-image bytes / allocations | 12,520 / 30 | 12,520 / 30 | unchanged |

The execution gains repeat in the earlier six-sample run. That run also measures streaming inputs up to 4,194,304 elements: i32 improves 64.51%, and f32 improves 56.09%. All direct execution rows have zero bytes and allocations per call. The earlier compile controls used a helper that left native mappings open; use the closed-image follow-up for those controls. Several timing rows have large outliers. The gain is clear, but small differences need caution. No significant compile regression was detected in the closed-image ordinary, rejected, AssemblyScript, existing-f64, or whole-Yosys controls. Their heap allocation counts and bytes are unchanged. This is not proof of zero compile overhead. Existing sum execution and compile results do not show a significant regression in the repeated 8,192-element case.

The public compile → instantiate → one call → close test adds two allocations, from 104 to 106, and 1,664 bytes for both map types. Counts zero and 16 cost 15–23% more in the closed-image run. At 8,192 elements, i32 costs 7.48% more; the f32 timing difference is not significant. Thus direct-call speed does not establish a one-call program gain. PR #909 reported about 166% extra compile time and 25 extra allocations. The new measured costs are smaller, but the experiments use different fixtures and compiler versions.

The allocation profile identifies the next step. Heap image capacities for the two fixtures are 416 and 421 bytes. The generated functions need 758 and 760 bytes. During range guards, the encoder grows its detached buffer by one allocation. The final image then grows and copies that code with a second allocation. The i32 profile records 768 and 896 bytes at these sites, which sum to the measured 1,664 bytes. Page-rounded mapped code storage hides these two heap growths. See [growth traces](results/heap-growth-traces.txt). Recognition itself allocates zero objects for accepted and rejected fixtures.

Test a bounded capacity estimate for eligible map functions through the existing compile hints. Require the heap-image allocation count to return to 30, and measure reserved bytes in rejected and ordinary functions. Also test less snapshot and repeated range-length work in the existing guards. Do not increase all function capacities or add a new compiler path without evidence. These are proposed mitigations; this experiment does not claim that they work.

The real corpus search covers 124 original modules, 282,805 defined functions, and 222,984 loops. Seven planner matches occur in Yosys. Three become native SIMD loops in function 5011; four fail the native exit/home proof. The unchanged Yosys routine gives the measured gain, using the same native host adapter on both versions. There is no whole-Yosys command speedup or evidence that this routine is hot in normal use. DuckDB, SQLite, Rust, AssemblyScript, PolyBench, image/audio libraries, and the added CMSIS-DSP builds give no new emitted maps. [COVERAGE.md](COVERAGE.md) records locations, provenance, structures, and blockers. Wider practical benefit remains unproved.

The selected i32/f32 test functions grow 246→758 and 258→760 bytes. Their frames grow 56→136 bytes; reported spills and reloads remain zero. Yosys function 5011 grows 570→1,345 bytes, with frame 56→136, zero spills, and three reloads on both versions. The full native Yosys image grows by 848 bytes. The four rejected plans add no private slots or range guards; their enclosing functions each grow 32 bytes from fallback jumps. [Assembly and paired statistics](results/native-final-candidate.txt) preserve the evidence.

Focused tests pass for counts zero through nine, thresholds, tails, large inputs, unaligned addresses, exact and partial overlap, full 4 GiB memory, wraparound, immediate offsets, OOB effects, trap positions, stores before a later trap, live locals, integer overflow, float rounding/NaNs/infinities/signed zero, pinned registers, pressure, scalar fallback, and CPU rejection. Profile tests retain original instruction positions. Added counter and live-prefix tests assert native admission and fast execution. Untimed output checks pass through 4,194,304 elements. ARM64 cross-compilation passes; no native ARM64 performance claim is made.

Full CI is not green. The full diagnostic AMD64 backend run has the same 24 failing test names as main, with no new failures. The root test run also fails on missing staged conformance files and backend tests. These limits are saved. No unrelated fixes were made. Independent correctness and performance reviews inspected code and raw results. Confirmed test-proof and benchmark defects were fixed. Claude Code returned “Not logged in”; no Claude review occurred. See [REVIEW.md](REVIEW.md).

Wago can automatically emit SIMD maps directly with much less cost than the prior rewrite approach. The current build still fails the zero-added-compiler-allocation target and lacks broad workload evidence. The capacity-growth cause gives a specific, measurable reason to iterate. No PR or opt-in optimization was created.
