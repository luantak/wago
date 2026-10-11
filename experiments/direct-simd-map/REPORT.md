# Direct SIMD scalar maps

**ITERATE. The bounded capacity fix removes the two extra allocations in the measured maps. It adds a measurable compile-time cost. Keep the work on the experiment branch.**

Wago can automatically emit useful native SIMD maps from the original scalar Wasm bytes. The allocation target is now met for the tested shapes. Default promotion still needs a lower compile cost and stronger workload evidence.

Latest main is `cea8f83f101fbdd0fcad634e9c7faad0377b676e`. The final measured build is merge commit `f62d4ca2d1a1aae62563b03b0c9ed3b522e1c05b`, on `codex/direct-simd-map`. It adds the same upstream host-call change as main; compiler source is unchanged from `66d6c445b`. The isolated capacity comparison uses `9b0e87570` against `66d6c445b`, with matched host code. The failing allocation test was committed in `b6e6ebfbd`, before the implementation in `424ef3b64`. Later commits removed the repeated serial sizing scan, added a cheap multiply-byte filter, and reduced the reserve from 1,024 to 512 bytes. The user's working checkout was not changed.

The fix uses existing loop hints to skip ordinary functions. It scans functions no larger than 512 bytes with fixed arrays and the existing planner. It reserves 512 bytes per qualified map, at most four maps / 2 KiB per function and 1 MiB per initial module buffer. Retained hint headers and sidecars are unchanged. The serial encoder receives all remaining module capacity. Parallel workers use a temporary sizing hint. The large-module heap trim preserves the added reserve. Normal growth remains available for larger functions or insufficient estimates. A later native home check can reject a plan that received capacity; this estimate does not authorize emission.

The SIMD path remains automatic: four i32/f32 lanes, one load/store with stride four, invariant multiplication, and optional addition. It keeps the original checked scalar body for fallback and tails. SSE4.1 is required for `pmulld`; f32 uses separate `mulps`/`addps`, with no FMA. Recognition and emission use no module clone, rewrite, source-position tape, second compilation, or general IR. ARM64 is unchanged.

The final comparisons each contain six alternating samples for 63 backend cases and six public cases. They use CPU 6 on a Ryzen 7 8845HS, Go 1.27.1, `GOMAXPROCS=1`, identical inputs and feature masks. Raw data, benchstat, source/input/binary hashes, and settings are in [capacity-latest-main](results/capacity-latest-main/) and [capacity-before](results/capacity-before/). Public CPU selection is `0x7ff` on both versions. Several timing rows have large outliers.

| Measurement | Main | Final | Result |
|---|---:|---:|---:|
| i32 execution, 8,192 elements | 5.473 µs | 0.570 µs | −89.58% |
| f32 execution, 8,192 elements | 4.059 µs | 0.939 µs | −76.87% |
| Original Yosys routine, 8,192 elements per array | 11.309 µs | 1.849 µs | −83.65% |
| i32 heap compile | 7.779 µs | 10.640 µs | +36.79% |
| f32 heap compile | 7.474 µs | 10.302 µs | +37.85% |
| i32 heap bytes / allocations | 12,488 / 30 | 13,096 / 30 | +608 bytes, no added allocations |
| f32 heap bytes / allocations | 12,520 / 30 | 13,096 / 30 | +576 bytes, no added allocations |
| Rejected heap bytes / allocations | 12,520 / 30 | 12,520 / 30 | unchanged |

These execution and heap-compile timing changes have `p=0.002`. At 4,194,304 elements, i32 improves 65.33% and f32 improves 54.26%. Execution has zero bytes and allocations per call. Mapped compilation is 14.28→17.41 µs for i32 (+21.95%) and 14.53→17.52 µs for f32 (+20.59%), both `p=0.002`. Ordinary/rejected control allocations and bytes stay the same. ALU compilation is 3.80% slower (`p=0.009`), after a 5.82% increase against the previous experiment (`p=0.026`). That control has no loop, so the cause remains unproved; it is a repeated no-match cost to investigate. Other no-match controls and existing sum execution show no detected regression. Counts 0–9 show no detected execution change; i32 count 15 is 2.44% slower (0.70 ns, `p=0.026`).

The capacity fix exchanges allocation churn for compile work:

| Measurement | Before fix | Final | Result |
|---|---:|---:|---:|
| i32 heap bytes / allocations | 14,152 / 32 | 13,096 / 30 | −1,056 bytes, −2 allocations |
| f32 heap bytes / allocations | 14,184 / 32 | 13,096 / 30 | −1,088 bytes, −2 allocations |
| Public bytes / allocations, i32 | 24,129 / 106 | 23,073 / 104 | −1,056 bytes, −2 allocations |
| Public bytes / allocations, f32 | 24,161 / 106 | 23,073 / 104 | −1,088 bytes, −2 allocations |
| i32 mapped compile | 17.20 µs | 18.78 µs | +9.21%, p=0.002 |
| f32 mapped compile | 17.41 µs | 19.45 µs | +11.69%, p=0.002 |
| i32 heap compile | 9.996 µs | 11.763 µs | +17.67%, p=0.041 |

The f32 heap timing change is inconclusive. Public one-call comparisons have no statistically significant change. Latest candidate medians are 11–21% higher; the baseline has a shared final-sample spike. This does not establish equal timing or a one-call speed gain. Public allocations meet the target.

Actual heap staging grows 832/842→928/933 bytes against the previous build. These capacities include the reserve, not just live native bytes. Mapped single-function pages stay at 4,096 bytes. For 1,024 maps, staging falls 1,078,464→793,904 bytes. Ordinary/rejected capacities and hint storage are unchanged. The heap profile has no samples at either former growth site. A separate CPU profile assigns 6.89% of samples to the new capacity check, including 5.23% to the planner. Profile timings are excluded from normal results. See [resource and profile evidence](results/capacity-checks/).

The next practical step is to reuse recognition information from the existing hint scan instead of repeating static map recognition for capacity. Test whether this removes the measured compile penalty while keeping 30 heap allocations, 104 public allocations, bounded memory, and unchanged no-match costs. This is a proposed mitigation, not a demonstrated result.

Native bytes are unchanged by the capacity fix. The i32/f32 functions remain 758/760 bytes with 136-byte frames and zero reported spills/reloads. Yosys function 5011 remains 1,345 bytes, frame 136, zero spills, three reloads. Compared with main, those frames and functions remain larger. The corpus still has 124 modules, 282,805 functions, and 222,984 loops. Seven Yosys plans match; three emit SIMD in function 5011. Its original routine benefits, but whole-Yosys speed and normal-use hotness remain unproved. [COVERAGE.md](COVERAGE.md) retains the broader search and rejection reasons.

Focused correctness, allocation, native-admission, capacity-boundary, and profile tests pass. They include counter variants, affine operands, multiple last-value homes, four/five maps, 2,048/2,049 functions, serial/parallel output, entry offsets, and CPU metadata. ARM64 cross-compilation passes. The diagnostic suite at `66d6c445b` has the same 24 failing top-level tests as the initial main baseline, with no new failures. The latest-main merge changes no backend code; focused backend and public admission tests pass after the merge. Full CI is not green; the earlier root run also lacked staged conformance inputs.

Independent GPT correctness review found no blocking defect. The performance review confirmed the allocation repair and recommended ITERATE because of the measured compile penalty and no-match control cost. The author verified the raw results and profile evidence. Claude Code was attempted again with read-only tools; it exited with “Not logged in.” No Claude review occurred. [REVIEW.md](REVIEW.md) records the findings. No PR or opt-in mode was created. [The earlier report](REPORT-before-capacity.md) preserves the pre-fix evidence.
