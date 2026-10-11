#!/usr/bin/env python3
"""Untimed checks after final benchmarks; preserve logs and all command exits."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[2]
lab = root / 'experiments/direct-simd-map'
out = lab / 'results/hint-checks'
backend = 'src/core/compiler/backend/railshot/amd64'
env = dict(os.environ, GOFLAGS='-buildvcs=false', GOMAXPROCS='1')
commands = []

def run(name, cmd, directory=root, extra=None):
    with (out / name).open('w') as f:
        r = subprocess.run(cmd, cwd=directory, env=dict(env, **(extra or {})), stdout=f, stderr=subprocess.STDOUT)
    commands.append(dict(output=name, command=cmd, cwd=str(directory), exit_code=r.returncode))
    print(name, r.returncode, flush=True)
    return r.returncode

for name, directory in [('main', Path('/tmp/wago-map-hints-main')), ('before', Path('/tmp/wago-map-hints-before')), ('final', root)]:
    if directory != root:
        (directory / backend / 'direct_map32_resources_test.go').write_bytes((root / backend / 'direct_map32_resources_test.go').read_bytes())
    run('resources-' + name + '.txt', ['go', 'test', '-tags=wago_codegenstats', '-count=1', '-v', './' + backend, '-run', '^TestDirectMap32CapacityResources$'], directory)

run('hint-corpus.txt', ['go', 'test', '-count=1', '-v', './' + backend, '-run', '^TestDirectMap32HintCorpus$'], extra={'DIRECT_MAP32_CORPUS': str(root / 'corpus/workloads'), 'DIRECT_MAP32_HINT_COVERAGE': str(out / 'hint-corpus.json')})
run('hint-corpus-additional.txt', ['go', 'test', '-count=1', '-v', './' + backend, '-run', '^TestDirectMap32HintCorpus$'], extra={'DIRECT_MAP32_CORPUS': str(lab / 'downloads'), 'DIRECT_MAP32_HINT_COVERAGE': str(out / 'hint-corpus-additional.json')})
native = lab / 'results/native-hints'
native.mkdir(exist_ok=True)
run('native-final.txt', ['go', 'test', '-tags=wago_codegenstats', '-count=1', '-v', './' + backend, '-run', '^TestDirectMap32(Artifacts|RealNative)$'], extra={'DIRECT_MAP32_ARTIFACTS': str(native), 'DIRECT_MAP32_REAL': str(root / 'corpus/workloads/applications/yosys/yosys.wasm')})
# Remove our copied experiment tests for the original-main full suite.
for name in ['direct_map32_test.go', 'direct_map32_edges_test.go', 'direct_map32_resources_test.go']:
    (Path('/tmp/wago-map-hints-main') / backend / name).unlink()
for name, directory in [('main', Path('/tmp/wago-map-hints-main')), ('final', root)]:
    run('backend-' + name + '.json', ['go', 'test', '-json', '-tags=wago_codegenstats,wago_regallocdebug', '-count=1', './' + backend], directory)
failures = {}
for name in ['main', 'final']:
    rows = [json.loads(line) for line in (out / ('backend-' + name + '.json')).read_text().splitlines() if line.startswith('{')]
    names = sorted({r['Test'] for r in rows if r.get('Action') == 'fail' and 'Test' in r})
    failures[name] = dict(names=names, top_level=[n for n in names if '/' not in n])
failures['new'] = sorted(set(failures['final']['names']) - set(failures['main']['names']))
(out / 'backend-failures.json').write_text(json.dumps(failures, indent=2) + '\n')

# Binary main.test is the previous-capacity build from hints-final-before.
for name, binary in [('before', lab / 'main.test'), ('final', lab / 'candidate.test')]:
    for case in ['false', 'alu']:
        profile = out / ('cpu-' + name + '-' + case + '.pprof')
        run('cpu-' + name + '-' + case + '.txt', ['taskset', '-c', '6', str(binary), '-test.run', '^$', '-test.bench', '^BenchmarkDirectMap32Compile$/' + case + '$', '-test.benchtime', '3s', '-test.cpuprofile', str(profile)], root / backend)
        run('cpu-' + name + '-' + case + '-top.txt', ['go', 'tool', 'pprof', '-top', '-cum', str(binary), str(profile)])
        run('cpu-' + name + '-' + case + '-capacity.txt', ['go', 'tool', 'pprof', '-top', '-cum', '-focus=directMap32CodeReserve|inspectRegionLoop', str(binary), str(profile)])
heap = out / 'heap-final.pprof'
run('heap-final.txt', ['taskset', '-c', '6', str(lab / 'candidate.test'), '-test.run', '^$', '-test.bench', '^BenchmarkDirectMap32CompileHeap$/i32$', '-test.benchtime', '100x', '-test.memprofilerate', '1', '-test.memprofile', str(heap)], root / backend)
run('heap-final-growth.txt', ['go', 'tool', 'pprof', '-traces', '-alloc_objects', '-focus=imm32|growPreserving', str(lab / 'candidate.test'), str(heap)])
(out / 'verification-commands.json').write_text(json.dumps(commands, indent=2) + '\n')
(out / 'profile-binary-hashes.json').write_text(json.dumps({str(f.relative_to(lab)): hashlib.sha256(f.read_bytes()).hexdigest() for f in [lab / 'main.test', lab / 'candidate.test']}, indent=2) + '\n')
