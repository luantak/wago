# Reproduce the experiment

Use the experiment repository root. Requirements: Linux AMD64, Go 1.27.1, Python 3, `taskset`, GNU `objdump`, and `benchstat`. Use a CPU with SSE4.1 and AVX for the matched modern-mask measurements. The original run uses Ryzen 7 8845HS logical CPU 6. The script uses CPU 6 if available; otherwise it uses the first available CPU. It runs each version in serial, alternates their order, uses `GOMAXPROCS=1`, and excludes build time from benchmark time. Do not run other CPU-heavy work at the same time.

Create a disposable main worktree. The script copies only experiment test harnesses into it; production code remains at the baseline commit.

```bash
git worktree add --detach /tmp/wago-direct-map-main cea8f83f101fbdd0fcad634e9c7faad0377b676e
python3 experiments/direct-simd-map/run.py final \
  --baseline /tmp/wago-direct-map-main --label reproduce-capacity-main --samples 6
```

To isolate the capacity change from SIMD emission, repeat against the previous experiment:

```bash
git worktree add --detach /tmp/wago-direct-map-before-capacity 9b0e87570
git checkout --detach 66d6c445b39e9f7f6f6042b7c9d5262399587d5a
python3 experiments/direct-simd-map/run.py final \
  --baseline /tmp/wago-direct-map-before-capacity --label reproduce-capacity-before --samples 6
git checkout codex/direct-simd-map
```

Use merge commit `f62d4ca2d1a1aae62563b03b0c9ed3b522e1c05b` for the latest-main comparison. It contains the final capacity source from `66d6c445b39e9f7f6f6042b7c9d5262399587d5a` plus the same upstream host-call change as latest main. For the isolated capacity comparison against `9b0e87570`, use `66d6c445b` so both versions use the same host code. A documentation-only successor with matching source hashes is also valid. `capacity-latest-main` and `capacity-before` are the authoritative final comparisons. Earlier capacity variants remain as diagnostic evidence.

The current harness closes every compile code image and includes heap staging. It also checks direct execution results after timing. Public tests use the same original exported Wasm fixtures from `results/native-candidate`. The script refuses to reuse a nonempty result directory. Select a new label for each run. Verify six rows for each benchmark case before using benchstat. Manifests identify both Git commits, source hashes (including capacity, compile, and hint code), Wasm input hashes, harness hash, feature masks, build settings, selected environment, Go version, and CPU affinity. Binary hashes are saved after completion. Source commits in older manifests may predate uncommitted test refinements; the recorded source hashes identify those runs.

To repeat only the final closed-image cases, use separate regexes for their different benchmark name depths:

```bash
python3 experiments/direct-simd-map/run.py final \
  --baseline /tmp/wago-direct-map-main --label reproduce-closed --samples 6 \
  --backend-regex '^BenchmarkDirectMap32Compile' \
  --backend-regex '^BenchmarkDirectMap32Execute$/(false|true)$/8192$' \
  --backend-regex '^Benchmark(DirectMap32YosysExecute|LinearSumNoWrapAMD64)$/8192$' \
  --backend-regex '^BenchmarkCompileLinearSumAMD64$'
```

`DirectMap32` compiler and execution cases use the explicit `AMD64ModernBaseline` mask. Existing sum controls use the compiler's unchanged default mask on both versions. Public cases use the same detected host features. In the first `final` run, ordinary compile controls used the existing helper and its unchanged default mask; those rows also retained native mappings. Do not use them as the final compile controls.

The threshold mode builds variants with count cutoffs 4, 8, 16, 32, and 64. It restores the original production source even when a build fails. It modifies only the experiment worktree while it builds; do not use that checkout concurrently.

```bash
python3 experiments/direct-simd-map/run.py thresholds \
  --baseline /tmp/wago-direct-map-main --label reproduce-thresholds --samples 3
```

Run the focused correctness and source-position checks:

```bash
DIRECT_MAP32_YOSYS="$PWD/corpus/workloads/applications/yosys/yosys.wasm" \
go test -tags='wago_codegenstats wago_regallocdebug wago_profile' -count=1 \
  ./src/core/compiler/backend/railshot/amd64 -run '^TestDirectMap32' \
  -bench '^BenchmarkDirectMap32Execute$' -benchtime=1x
```

The one-iteration benchmark run checks outputs through 4,194,304 elements. It is validation, not performance evidence. Tests that need corpus/artifact environment variables skip when those variables are absent. `regionLoopTestFast` is an existing diagnostic assertion; it is off in all timed runs. No new opt-in compiler switch is used.

Run a corpus scan or collect native artifacts:

```bash
DIRECT_MAP32_CORPUS="$PWD/corpus/workloads" \
DIRECT_MAP32_COVERAGE="$PWD/experiments/direct-simd-map/results/reproduce-coverage.json" \
go test -count=1 -v ./src/core/compiler/backend/railshot/amd64 -run '^TestDirectMap32Corpus$'

mkdir -p experiments/direct-simd-map/results/reproduce-native
DIRECT_MAP32_ARTIFACTS="$PWD/experiments/direct-simd-map/results/reproduce-native" \
DIRECT_MAP32_REAL="$PWD/corpus/workloads/applications/yosys/yosys.wasm" \
go test -tags=wago_codegenstats -count=1 -v \
  ./src/core/compiler/backend/railshot/amd64 -run '^TestDirectMap32(Artifacts|RealNative)$'

objdump -D -b binary -m i386:x86-64 \
  experiments/direct-simd-map/results/reproduce-native/map-false.bin
```

The artifact test also writes the original exported fixtures for the public benchmark. Collect baseline artifacts with the same copied harness and feature mask. The native adapter for unexported Yosys function 5011 is used on both versions and is excluded from compile timing. Never call its internal register-ABI entry directly from the host engine.

Build the retained original CMSIS routine with the upstream headers:

```bash
git clone https://github.com/ARM-software/CMSIS-DSP.git /tmp/wago-map-cmsis
git -C /tmp/wago-map-cmsis checkout 83a2d7bc98c81b4bbe4a6f48b1f2ecf179868a0b
emcc -Oz -D__GNUC_PYTHON__ -I /tmp/wago-map-cmsis/Include \
  -I /tmp/wago-map-cmsis/PrivateInclude \
  /tmp/wago-map-cmsis/Source/BasicMathFunctions/arm_scale_f32.c \
  -s STANDALONE_WASM -s EXPORTED_FUNCTIONS='["_arm_scale_f32"]' --no-entry \
  -o /tmp/cmsis-scale-Oz.wasm
```

Use `-O2` for the second build. The recorded version is Emscripten 6.0.12; other compiler versions can produce different forms. The original source, license, Wasm inputs, and artifact hashes are retained under `inputs/` and `results/artifact-sha256.json`. DuckDB's fixed download URL and SHA-256 are in `COVERAGE.md`.

To inspect heap growth after a benchmark build:

```bash
GOMAXPROCS=1 taskset -c 6 experiments/direct-simd-map/candidate.test \
  -test.run '^$' -test.bench '^BenchmarkDirectMap32CompileHeap$/i32$' \
  -test.benchtime=100x -test.memprofilerate=1 \
  -test.memprofile=/tmp/direct-map32-heap.pprof
go tool pprof -traces -alloc_objects -focus='imm32|growPreserving' \
  experiments/direct-simd-map/candidate.test /tmp/direct-map32-heap.pprof
```

Profiling changes execution cost. Do not compare its times with ordinary benchmark times.

For an untimed staging-capacity check, run this command in each version with the same copied test harness:

```bash
go test -tags=wago_codegenstats -count=1 -v \
  ./src/core/compiler/backend/railshot/amd64 -run '^TestDirectMap32CapacityResources$'
```

The saved resource test reports actual compiler staging capacity, native bytes, projected mapped bytes, and retained hint storage. `resources-final.txt` is the final 512-byte reserve; `resources-candidate.txt` is an earlier 1,024-byte reserve.

For the added scan's CPU profile, use the same candidate binary from the benchmark build:

```bash
GOMAXPROCS=1 taskset -c 6 experiments/direct-simd-map/candidate.test \
  -test.run '^$' -test.bench '^BenchmarkDirectMap32Compile$/i32$' \
  -test.benchtime=3s -test.cpuprofile=/tmp/direct-map32-cpu.pprof
go tool pprof -top -cum -focus='directMap32CodeReserve|inspectRegionLoop' \
  experiments/direct-simd-map/candidate.test /tmp/direct-map32-cpu.pprof
```

Saved evidence groups:

- `capacity-latest-main`: final 512-byte reserve plus the upstream host-call change versus latest main; six samples for 63 backend cases and six public cases.
- `capacity-tight-main`: the same capacity source versus the main commit at the start of this follow-up (`b00d96f8f`); six samples per case.
- `capacity-before`: the same final source versus the previous experiment; six samples for each case.
- `capacity-checks`: final tests, full diagnostic failure-name comparison, resource measurements, profiles, independent review records, and Claude authentication failures. Profile times are not ordinary benchmark results.
- `native-capacity`: final native artifacts and original public Wasm fixtures; hashes match the previous native artifacts.
- `capacity-main`: initial 1,024-byte reserve with two serial sizing scans; diagnostic only.
- `capacity-final-main`: 1,024-byte reserve after removal of the repeated serial sizing scan and addition of the multiply-byte filter; diagnostic only.

- `verified-closed`: previous experiment closed-image controls, heap staging, repeated direct calls, original Yosys routine, and public one-call cost; six samples each.
- `final`: full input-size execution sweep and first public pass; compile controls have the documented mapping-retention limit.
- `final-closed`: a second direct 8,192-element/public pass. Its three-part regex omitted compile and Yosys rows; it is not compile evidence.
- `verified`: an aborted heap harness run. It called `Close` through a nil image interface. The harness was fixed; this is not a compiler failure or timing comparison.
- `thresholds`: three-sample cutoff study. Long counts also matched Go's sub-benchmark selection and are present in the raw files. Use the short rows for cutoff choice.
- `*-pilot*`, `native-candidate.txt`, and `yosys-native.txt`: early evidence. One pilot used partial overlap, and early native sizes differ. Use `native-final-main.txt`, `native-final-candidate.txt`, and current assembly for final sizes.
- `yosys-execution.txt`: an initial evidence adapter crash caused by calling an internal ABI entry directly. The corrected native adapter passes and is used on both benchmark versions.

The full backend/root failures and missing staged conformance inputs are retained. These files do not establish a passing full CI run. ARM64 has only cross-build evidence. Remove the disposable main worktree when finished:

```bash
git worktree remove --force /tmp/wago-direct-map-main
git worktree remove --force /tmp/wago-direct-map-before-capacity
```
