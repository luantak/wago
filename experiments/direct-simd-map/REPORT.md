# Direct SIMD scalar maps

**ITERATE. Hint reuse removes the repeated capacity scan and keeps the allocation target. The final timings do not prove that total compile cost or ALU cost is now equal to main. Keep the implementation on the experiment branch.**

The final source is `d6bca7168c33a25f50e831140d1c8c39c98b78cf`, on `codex/direct-simd-map`. Main is pinned to `cea8f83f101fbdd0fcad634e9c7faad0377b676e`, fetched at the start of this follow-up. The previous capacity build is `1ca658021`. Both comparisons have matched host code. The failing hint-reuse test was committed first in `3caa8044e`; implementation followed in `eea214d06`. Work stayed in the experiment worktree.

The existing hint scan now records small map shapes in unused bits of the density field. Density, map counts and the fixed-register lease have separate masks. Retained hint headers stay at 28 bytes; sidecars are unchanged. Capacity reads these counts during the existing module body-length walk. It no longer decodes the body, builds a local-type array or calls the planner. Functions with zero map hints skip the helper. The options argument is passed by pointer, and the transient sizing field has been removed. No allocation or retained tape was added.

Reserve remains bounded: 512 bytes per hint, four hints / 2 KiB per function, and 1 MiB per initial module buffer. Only bodies up to 512 bytes and 256 locals receive hints. CPU, memory, EH, interruptible, custom-instruction and GC-root gates remain. The hint is a conservative sizing estimate. Full planning, native register checks and runtime guards still control SIMD emission. Native output is unchanged from the previous capacity build.

The final full comparison has 64 backend cases and six public cases, with six alternating samples per version at 150 ms per case. The final previous-build comparison has seven compile/hint cases and six public cases, with six samples at one second per case. Both use CPU 6, Ryzen 7 8845HS, Go 1.27.1 and `GOMAXPROCS=1`. Inputs, feature masks, source hashes, harness hashes and binary hashes are retained in [hints-final-main](results/hints-final-main/) and [hints-final-before](results/hints-final-before/). Public CPU selection remains `0x7ff`. Other Go, TinyGo and QEMU activity was observed on the host. Timing variation is large; no samples were removed.

| Allocation measurement | Main | Previous capacity | Final |
|---|---:|---:|---:|
| i32 heap bytes / allocations | 12,488 / 30 | 13,096 / 30 | 13,096 / 30 |
| f32 heap bytes / allocations | 12,520 / 30 | 13,096 / 30 | 13,096 / 30 |
| Public bytes / allocations, both types | 22,465–22,497 / 104 | 23,073 / 104 | 23,073 / 104 |

All final-main backend and public rows keep main's allocation count. The focused comparison keeps the previous build's bytes and counts. The earlier capacity fix's two-allocation reduction, 32→30 for heap compilation and 106→104 for public compilation, is retained. Execution uses zero bytes and allocations. The final heap profile has no samples at either former growth site; this confirms the measured fixture, not every possible shape.

| Final comparison with main | Main | Final | Result |
|---|---:|---:|---:|
| i32 execution, 8,192 elements | 6.940 µs | 0.764 µs | −89.00%, p=0.002 |
| f32 execution, 8,192 elements | 4.403 µs | 1.424 µs | −67.65%, p=0.002 |
| Original Yosys routine, 8,192 elements | 15.061 µs | 2.289 µs | −84.80%, p=0.002 |
| i32 heap compile | 9.611 µs | 12.352 µs | no proved difference, p=0.132 |
| f32 heap compile | 9.322 µs | 15.501 µs | +66.28%, p=0.041 |
| ALU compile | 35.810 µs | 35.200 µs | no proved difference, p=0.937 |
| ALU hint scan | 6.958 µs | 6.591 µs | no proved difference, p=0.699 |

At 4,194,304 elements, i32 improves 56.33% and f32 improves 52.80%. Small-count and ordinary compile rows do not establish equal speed. Mapped i32/f32 compilation is 18.68→20.92 µs / 17.85→20.50 µs, with no proved difference. The f32 heap increase remains a concern despite host noise. Public compile–instantiate–invoke–close timing proves no gain in either final comparison.

Against previous capacity, mapped i32/f32 compilation is 18.10→20.62 µs / 19.81→18.76 µs. Heap i32/f32 is 10.45→10.82 µs / 10.28→14.00 µs. ALU compilation is 28.747→30.607 µs and isolated hints are 5.229→5.787 µs. All seven backend timing comparisons have p≥0.589. These results do not prove that the compile penalty is removed.

The ALU investigation found two former costs: an unconditional capacity-helper call with a large options copy and a larger transient hint view. Both are removed. The first hint implementation also changed opcode dispatch and measured a 5.68% isolated scan increase. Two switch-layout trials moved that cost. The selected final layout keeps the original dispatch and puts shape updates inside the existing immediate-free case. Constants bypass shape work; add still executes a shape load, test and skip branch. The scan wrapper uses eight more stack bytes than main. Separate CPU profiles show the capacity helper falling from 9.38% to 0.29% of samples; the remaining emission planner is 3.54%. This supports removal of duplicate work, not a net wall-time claim. Profile timings are excluded from ordinary comparisons.

Conservative hints exchange repeated proof for some unused storage. One rejected XOR-zero shape keeps native output at 246 bytes but grows heap staging 431→943 bytes. Four rejected shapes in a 512-byte body keep native output at 531 bytes but grow the compiler arena 4,096→8,192 and heap staging 2,640→4,688 bytes. Allocation counts stay equal. The executable-page projection from live native bytes stays 4,096; it is separate from actual compiler arena capacity. Accepted staging remains 928/933 bytes, and 1,024 maps remain at 793,904 bytes. Hint storage is unchanged. See [resource, profile and verification evidence](results/hint-checks/).

The combined hint scan covers the same 124 modules and 282,805 functions as the broader search. It finds 12 hinted functions, 23 hints and 11,776 reserve bytes before module caps. Eleven hinted functions have no full static plan. One contains three static plans. These counts do not prove native admission. The broader search still has seven planner matches; only three emit in Yosys function 5011. All seven saved native/input artifact hashes match previous capacity. i32/f32 native functions stay at 758/760 bytes, frame 136, zero reported spills/reloads. Yosys 5011 stays at 1,345 bytes, frame 136, zero spills and three reloads. Whole-Yosys speed and normal-use hotness remain unproved. [COVERAGE.md](COVERAGE.md) records the search.

Focused correctness, packing, density saturation, lease, nesting, parallel scan, capacity-boundary, allocation, native-admission, scalar fallback, source-position and public admission checks pass. ARM64 cross-compilation passes. Full diagnostic backend runs have the same 24 failing top-level tests / 86 failing names on main and final, with no new failures. Full CI is not green; the earlier root run also lacked staged conformance inputs.

Independent GPT correctness review found no blocker. Independent GPT performance review prefers this implementation but recommends ITERATE the performance validation. The author checked hashes, counts and findings. Claude Code was attempted with the required read-only command and exited with “Not logged in”; no Claude review occurred. [REVIEW.md](REVIEW.md) records both reviews. No PR or opt-in mode was created.

The next useful check is a quiet-host repeat of the longer ALU, accepted heap and mapped compile cases against both baselines. Retain the false-positive resource checks. [REPRODUCE.md](REPRODUCE.md) gives the commands. [The previous report](REPORT-before-hint-reuse.md) preserves the capacity-only results.
