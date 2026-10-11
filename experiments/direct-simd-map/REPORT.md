# Direct SIMD scalar maps

**KEEP for PR review under the execution-first priority.** The final repeat confirms large execution gains with no added allocation count. Compile and one-call costs remain. The small final change removes unused analysis but has no proved timing gain by itself. Full PR CI remains the merge gate.

Final source: `c09376c7da085143d5a92a06429e3198eda87fdf`, on `codex/direct-simd-map`. Main: `cea8f83f101fbdd0fcad634e9c7faad0377b676e`. Previous hint build: `cf7b83f84`. The final production change is `fcf719f4b`: qualified i32/f32 maps skip `memoryForms`, which only folds f64 loads and arithmetic. Qualification runs first; its exact data-event check excludes those f64 operations. Non-map plans keep the existing analysis. All seven native/input artifact hashes match the previous hint build.

The implementation emits four-lane native SIMD for a bounded scalar load–multiply–optional-add–store loop. Range, divisibility, memory, wrap and overlap guards preserve scalar fallback and tails. The existing hint scan supplies conservative code-capacity counts. It does not repeat body decoding, construct a temporary type array, or repeat planning. Hint headers remain 28 bytes. Reserve is bounded to 512 bytes per hint, four hints / 2 KiB per function, and 1 MiB per initial module buffer. Full planning and native/runtime guards still control emission. No opt-in mode is added.

The final main comparison has 14 backend and six public cases. The previous-build comparison has seven backend and six public cases. Each row has six one-second samples, with serial alternating versions, CPU 6, Ryzen 7 8845HS, Go 1.27.1 and `GOMAXPROCS=1`. Feature masks, sources, inputs, harness and binaries are recorded in [iteration-main](results/iteration-main/) and [iteration-before](results/iteration-before/). No tests or profiles ran alongside our timed measurements. Other Git activity was observed near the main build start. The previous-build comparison has a large shared candidate outlier round; every sample is retained.

| Final comparison with main | Main | Final | Change |
|---|---:|---:|---:|
| i32 execution, 8,192 elements | 5.397 µs | 0.560 µs | −89.62%, p=0.002 |
| f32 execution, 8,192 elements | 3.941 µs | 0.914 µs | −76.80%, p=0.002 |
| Original Yosys routine, 8,192 elements | 11.143 µs | 1.831 µs | −83.57%, p=0.002 |
| i32 mapped compile | 13.35 µs | 15.44 µs | +15.66%, p=0.002 |
| f32 mapped compile | 12.99 µs | 15.25 µs | +17.44%, p=0.002 |
| i32 heap compile | 7.005 µs | 8.790 µs | +25.48%, p=0.002 |
| f32 heap compile | 6.887 µs | 8.825 µs | +28.13%, p=0.002 |
| ALU compile | 26.586 µs | 27.382 µs | +2.99%, p=0.004 |
| Isolated ALU hint scan | 4.910 µs | 5.019 µs | +2.24%, p=0.041 |

The ALU regression remains. The fixture has no loop. Its measured cost is consistent with the remaining shared shape check in the existing immediate-free hint path; the final emitter change cannot remove that cost. Earlier profiles confirmed removal of the repeated capacity scan, but that did not remove total compile overhead. Rejected heap/mapped compile, existing f64 compile, and linear-sum controls show no proved difference. This does not prove equal speed.

Public f32 compile–instantiate–invoke–close is slower at counts 0, 16 and 8,192 by 13.46%, 14.29% and 7.14% (p=0.002, 0.002 and 0.009). Public i32 timing remains inconclusive. This is a repeated-execution optimization; it is not a one-call public speed win. Whole-Yosys speed and normal-use hotness remain unproved.

Against the previous hint build, i32/f32 heap medians are 8.833→8.787 µs and 8.661→8.616 µs; mapped medians are 17.06→16.12 µs and 16.81→16.03 µs. No incremental speed gain is proved: all seven backend p-values are at least 0.485 and public p-values at least 0.310. One candidate round is high across all backend controls; f32 heap is 92.151 µs in that round versus about 8.5–8.7 µs in the others.

| Allocation measurement | Main | Final |
|---|---:|---:|
| i32 heap bytes / allocations | 12,488 / 30 | 13,096 / 30 |
| f32 heap bytes / allocations | 12,520 / 30 | 13,096 / 30 |
| Public bytes / allocations | 22,465–22,497 / 104 | 23,073 / 104 |
| Execution bytes / allocations | 0 / 0 | 0 / 0 |

Every measured allocation count matches main. Every byte/count matches the previous hint build. The earlier two-allocation reduction, 32→30 for accepted heap compilation and 106→104 for public compilation, remains. The extra native code still requires storage. Conservative rejected shapes can add 512 bytes of heap staging; four shapes add 2,048 bytes and can cross a compiler-arena page boundary. Allocation counts and emitted code remain equal for those rejected fixtures. These earlier resource checks remain in [hint-checks](results/hint-checks/).

Real coverage remains narrow: 124 modules / 282,805 functions, 12 hinted functions / 23 hints, and three native selections in Yosys function 5011. Hints only estimate capacity. Eleven hinted functions have no full static plan. The i32/f32 fixture functions remain 758/760 bytes, frame 136, zero reported spills/reloads. Yosys 5011 remains 1,345 bytes, frame 136, zero spills and three reloads. See [COVERAGE.md](COVERAGE.md).

Focused ordinary, checked and profile tests pass. Staticcheck baseline verification and the checked benchmark-module suite pass. Windows/macOS AMD64 test builds pass. Ordinary and checked root suites passed outside `src/wago`; that package then passed both modes with the pinned spec corpus and WABT 1.0.41. Initial missing-corpus and wrong-WABT failures are retained. Native output was compared with matching stats-only build tags. Commands and results are in [iteration-checks](results/iteration-checks/).

Independent GPT correctness review found a Windows test-memory issue. The full-memory boundary test now runs on Linux/macOS, and each subtest releases its two 4 GiB mappings before the next case. Other map tests run on all three AMD64 hosts. The reviewer confirmed the fix and found no production blocker. Independent GPT performance review recommends KEEP for the PR with the measured costs above. Claude Code was attempted with the required read-only command but could not authenticate; no Claude review occurred. See [REVIEW.md](REVIEW.md).

[REPRODUCE.md](REPRODUCE.md) gives the commands. [The previous report](REPORT-before-final-iteration.md) preserves the earlier ITERATE decision and its evidence. The decision changes because the user prioritizes the repeated-execution gain and accepts a bounded compile-time trade, not because compile overhead has disappeared.
