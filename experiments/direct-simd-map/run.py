#!/usr/bin/env python3
"""Build matched harnesses, then run serial, alternating measurements."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

p = argparse.ArgumentParser()
p.add_argument("mode", choices=["thresholds", "final"])
p.add_argument("--baseline", required=True, type=Path)
p.add_argument("--samples", type=int, default=6)
p.add_argument("--duration", default="150ms", help="duration per final benchmark case")
p.add_argument("--label", help="new results directory name")
p.add_argument("--backend-regex", action="append", help="repeat for different benchmark depths")
a = p.parse_args()
a.backend_regex = a.backend_regex or ["^Benchmark(DirectMap32|LinearSumNoWrapAMD64|CompileLinearSumAMD64)"]
root = Path(__file__).resolve().parents[2]
lab = root / "experiments/direct-simd-map"
out = lab / "results" / (a.label or a.mode)
if out.exists() and any(out.iterdir()):
    p.error(f"results directory is not empty: {out}; select a new --label")
out.mkdir(parents=True, exist_ok=True)
backend = "src/core/compiler/backend/railshot/amd64"
env = dict(os.environ, GOMAXPROCS="1", GOFLAGS="-buildvcs=false",
           DIRECT_MAP32_ROOT=str(root),
           DIRECT_MAP32_YOSYS=str(root / "corpus/workloads/applications/yosys/yosys.wasm"),
           DIRECT_MAP32_INPUTS=str(lab / "results/native-candidate"))
cpu = min(set(os.sched_getaffinity(0)) & {6}) if 6 in os.sched_getaffinity(0) else min(os.sched_getaffinity(0))

def build(directory, kind, dest):
    pkg = backend if kind == "backend" else "src/wago"
    subprocess.run(["go", "test", "-c", "-o", str(dest), "./" + pkg],
                   cwd=directory, env=env, check=True)

def measure(binary, kind, regex, name, count, duration):
    cwd = root / (backend if kind == "backend" else "src/wago")
    cmd = ["taskset", "-c", str(cpu), str(binary), "-test.run", "^$",
           "-test.bench", regex, "-test.benchmem", "-test.benchtime", duration,
           "-test.count", str(count)]
    with (out / (name + ".txt")).open("a") as f:
        subprocess.run(cmd, cwd=cwd, env=env, stdout=f, stderr=subprocess.STDOUT, check=True)

for name in ["direct_map32_test.go", "direct_map32_edges_test.go"]:
    target = a.baseline / backend / name
    target.write_bytes((root / backend / name).read_bytes())
(a.baseline / "src/wago/direct_map32_bench_test.go").write_bytes(
    (root / "src/wago/direct_map32_bench_test.go").read_bytes())
manifest = {"baseline": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=a.baseline, text=True).strip(),
            "experiment": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(),
            "cpu": cpu, "features_backend": "SSSE3|SSE4.1|SSE4.2|AVX",
            "features_public": "unchanged detected host features on both versions",
            "features_existing_sum": "unchanged backend default AMD64ModernBaseline|BMI2 on both versions",
            "backend_regex": a.backend_regex,
            "samples": a.samples, "duration": a.duration, "environment": {k: env[k] for k in ["GOMAXPROCS", "GOFLAGS"]},
            "sources": {}}
for directory in [root / backend, root / "src/wago"]:
    for f in sorted(directory.glob("*map32*.go")):
        manifest["sources"][str(f.relative_to(root))] = hashlib.sha256(f.read_bytes()).hexdigest()
for name in ["vector_map_loop_plan.go", "vector_map_loop_emit.go", "compile.go", "hints.go"]:
    f = root / backend / name
    manifest["sources"][str(f.relative_to(root))] = hashlib.sha256(f.read_bytes()).hexdigest()
manifest["baseline_sources"] = {}
for relative in manifest["sources"]:
    f = a.baseline / relative
    if f.exists():
        manifest["baseline_sources"][relative] = hashlib.sha256(f.read_bytes()).hexdigest()
manifest["inputs"] = {}
for relative in ["corpus/workloads/assemblyscript/blake-as-simd.wasm", "corpus/workloads/polybench/jacobi-2d.wasm", "corpus/workloads/applications/yosys/yosys.wasm", "experiments/direct-simd-map/results/native-candidate/map-false.wasm", "experiments/direct-simd-map/results/native-candidate/map-true.wasm"]:
    manifest["inputs"][relative] = hashlib.sha256((root / relative).read_bytes()).hexdigest()
manifest["environment"].update({k: v for k, v in env.items() if k.startswith("WAGO_") or k.startswith("DIRECT_MAP32_")})
manifest["go_version"] = subprocess.check_output(["go", "version"], text=True).strip()
manifest["go_environment"] = json.loads(subprocess.check_output(["go", "env", "-json", "GOARCH", "GOOS", "GOAMD64", "CGO_ENABLED", "GOTOOLCHAIN"], env=env, text=True))
manifest["harness_sha256"] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
(out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
base = lab / "main.test"
build(a.baseline, "backend", base)
if a.mode == "thresholds":
    source = root / backend / "direct_map32.go"
    original = source.read_text()
    binaries = {"main": base}
    try:
        for n in [4, 8, 16, 32, 64]:
            trial = re.sub(r"const map32MinCount = \d+", f"const map32MinCount = {n}", original)
            source.write_text(trial)
            (out / f"source-{n}.go.txt").write_text(trial)
            binary = lab / f"threshold-{n}.test"
            build(root, "backend", binary)
            binaries[str(n)] = binary
    finally:
        source.write_text(original)
    regex = "^BenchmarkDirectMap32Execute$/(false|true)$/(0|1|3|4|7|8|9|15|16|17|31|32|33|64|128)$"
    for i in range(a.samples):
        order = list(binaries)
        if i % 2:
            order.reverse()
        for name in order:
            measure(binaries[name], "backend", regex, name, 1, "50ms")
else:
    candidate = lab / "candidate.test"
    main_api, candidate_api = lab / "main-api.test", lab / "candidate-api.test"
    build(root, "backend", candidate)
    build(a.baseline, "api", main_api)
    build(root, "api", candidate_api)
    for i in range(a.samples):
        order = [("main", base, main_api), ("candidate", candidate, candidate_api)]
        if i % 2:
            order.reverse()
        for name, binary, api in order:
            for regex in a.backend_regex:
                measure(binary, "backend", regex, name, 1, a.duration)
            measure(api, "api", "^BenchmarkDirectMap32WholeProgram$", name + "-whole", 1, a.duration)
    for left, right, label in [("main", "candidate", "backend"), ("main-whole", "candidate-whole", "whole")]:
        with (out / f"benchstat-{label}.txt").open("w") as f:
            subprocess.run(["benchstat", str(out / (left + ".txt")), str(out / (right + ".txt"))], stdout=f, check=True)
    hashes = {str(f.relative_to(lab)): hashlib.sha256(f.read_bytes()).hexdigest()
              for f in [base, candidate, main_api, candidate_api]}
    (out / "binary-sha256.json").write_text(json.dumps(hashes, indent=2) + "\n")
