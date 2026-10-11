# Original module coverage

The bounded scanner reads original Wasm bytes. It saves module SHA-256, global function index, Wasm byte position including local declarations, and opcode sequences. `results/coverage-expanded.json` covers the 121 existing corpus modules. `results/coverage-additional-final.json` covers DuckDB and the two CMSIS builds. Totals: 124 modules, 282,805 defined functions, 222,984 loops, seven planner matches, three native selections.

The 4,746 recorded probe entries are broad near-match evidence. A probe records co-occurrence of a 32-bit load, multiply, add, and store within the bounded scan. Address arithmetic can cause false positives; this is not a count of scalar maps. The scanner stops at the first `end` and can miss candidates inside larger nested bodies. `results/candidate-catalog.json` adds unsupported-instruction classes to each probe. Those classes identify an instruction that prevents this grammar; they do not identify the exact first planner rejection. Seventy entries use only supported opcodes but fail an affine, latch, event, or single-stream proof. Their precise first failure is not instrumented. The evidence does not claim exhaustive coverage of arbitrary compiler output.

| Original input | Function / Wasm PC | Structure | Main / experiment |
|---|---|---|---|
| Yosys | 5011 / 30, 90, 188 | In-place f32 load × invariant parameter, store, pointer +4, counter −1, `br_if 0` | Scalar / three native SIMD loops |
| Yosys | 6688 / 4233, 4371 | In-place f32 load × constant, store, decrement counter | Scalar / planner accepts; native exit/home proof rejects |
| Yosys | 11168 / 4223, 4361 | Same constant scale form | Scalar / planner accepts; native exit/home proof rejects |
| CMSIS `-Oz` | 1 / 1 | Top `if(count)` inside loop; scale is the left multiply operand; countdown and `br 1` | Scalar / rejects nested `if`; reversed operand also fails qualification |
| CMSIS `-O2` | 1 / 33 | Scalar remainder, scale × load | Scalar / reversed multiply operand fails qualification |
| CMSIS `-O2` | 1 / 90 | Four scalar loads/multiplies/stores per iteration | Scalar / rejects multiple streams and stores; reversed operands also fail qualification |
| SQLite | 30 / 6388, 6470 | Integer map-like address work with i64 conversion and nested control | Existing scalar compilation / rejects i64 conversion or nested block |
| Rust coreutils | 74 / 2307 | Integer shifts/division in a store loop | Existing scalar compilation / unsupported shifts/division |
| Clang/LLVM KissFFT | 1 / 231, 426 | Complex f32 arithmetic with multiple loads/stores and subtraction | Existing scalar compilation / rejects subtraction and stream count |
| NanoSVG | 5 / 28, 136 | Multiple f32 coordinate outputs, nested control, long bodies | Existing scalar compilation / rejects control, stream count, or 64-operation bound |
| LodePNG | 36 / 1298, 2242 | Nested loops and calls in pixel/address work | Existing scalar compilation / rejects nested control and calls |
| DuckDB | 602 / 69; 843 / 98 | Scalar integer work with conditionals, selects, and calls | Existing scalar compilation / rejects nested control and unsupported operations |
| AssemblyScript Blake/JSON/UTF | No planner match | Scalar or guest SIMD loops | Existing compilation / no added emitted map |
| PolyBench | No planner match | Mostly f64 stencils, reductions, and multiple inputs | Existing optimized f64 paths retained; no added 32-bit map |

Only Yosys function 5011 has measured new native execution. It scales two flat arrays and one nested array. The same native host adapter calls the original internal function on each compiler version. It changes no guest bytes or exports. Counts zero, one, four, eight, 16, 17, 32, 33, and 8,192 pass output checks for all three arrays. At 8,192 elements, the six-sample closed-image gain is 83.63%. This is an original routine benchmark; normal Yosys command frequency and command speed remain unmeasured. Rejected candidates have no new SIMD gain. Other near-match functions were not independently timed; their existing optimization status beyond this map path is not claimed.

Existing corpus provenance and licenses are in each module's README and `corpus/catalog.json`. Yosys comes from `yowasp-yosys==0.32.0.0.post560`, ISC license. Its original module hash is `7668d2963f8cc276ccd32980126dfddac961db1cd002cb93073506a694059493`. Native diagnostic statistics record the two rejected functions' `region-loop-exit-reject:2`. They do not identify which individual home or register check failed, so no narrower cause is claimed.

The additional [CMSIS-DSP](https://github.com/ARM-software/CMSIS-DSP) source is revision `83a2d7bc98c81b4bbe4a6f48b1f2ecf179868a0b`, `Source/BasicMathFunctions/arm_scale_f32.c`, Apache-2.0. The unchanged source, license, and two small Wasm files are retained under `inputs/`. Emscripten 6.0.12 builds use the supported portable `__GNUC_PYTHON__` header configuration. The failed initial build attempts and the successful build record are retained. See the [original scale routine documentation](https://arm-software.github.io/CMSIS-DSP/main/group__BasicScale.html).

DuckDB is the unchanged MVP module from [DuckDB Wasm](https://duckdb.org/docs/stable/clients/wasm/overview), package `@duckdb/duckdb-wasm@1.32.0`. Download: `https://cdn.jsdelivr.net/npm/@duckdb/duckdb-wasm@1.32.0/dist/duckdb-mvp.wasm`. SHA-256: `45d72a81fba8e57693d890da837c7041310e385e75619a8559839b15388dfe97`. It has 62,171 defined functions and 58,874 loops, zero accepted plans, and 1,067 near probes. Its 37.5 MiB download is not added to Git.

A small future grammar extension could admit the CMSIS top `if` and reversed multiply form, while preserving f32 operand order. It was not added after the heap-allocation failure was found. No guest was changed to manufacture a match. The small CMSIS artifacts are retained as real rejected inputs for later tests; they are not promoted to the production benchmark catalog.
