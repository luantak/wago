//go:build (linux || darwin || windows) && amd64

package amd64

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// Untimed evidence: sizing hints need not pass full static or native admission.
func TestDirectMap32HintCorpus(t *testing.T) {
	root, out := os.Getenv("DIRECT_MAP32_CORPUS"), os.Getenv("DIRECT_MAP32_HINT_COVERAGE")
	if root == "" || out == "" {
		t.Skip("hint corpus paths not set")
	}
	type function struct{ Index, Hints, Reserve, Plans int }
	type record struct {
		Path, Hash, Error string
		Functions         int
		Hinted            []function
	}
	var records []record
	opts := CompileOptions{AMD64Features: shared.AMD64ModernBaseline}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".wasm") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rec := record{Path: path, Hash: fmt.Sprintf("%x", sha256.Sum256(raw))}
		m, err := wasm.DecodeModule(raw)
		if err != nil {
			rec.Error = err.Error()
			records = append(records, rec)
			return nil
		}
		rec.Functions = len(m.Code)
		hints, _, _, err := computeModuleHints(m, 0, 0, nil, false)
		if err != nil {
			rec.Error = err.Error()
			records = append(records, rec)
			return nil
		}
		classifier := wasm.NewModuleInstructionClassifier(m, true)
		for fi, h := range hints {
			reserve := directMap32CodeReserve(m, fi, h, &opts)
			if reserve == 0 {
				continue
			}
			f := function{Index: fi + m.ImportedFuncCount(), Hints: h.map32HintCount(), Reserve: reserve}
			ft, ok := m.LocalFuncType(fi)
			if !ok {
				t.Fatal("missing function type")
			}
			var types []machineType
			for _, typ := range ft.Params {
				types = append(types, mtOf(typ))
			}
			for _, run := range m.Code[fi].Locals.Runs {
				for i := uint32(0); i < run.Count; i++ {
					types = append(types, mtOf(run.Type))
				}
			}
			r := wasm.ReaderFrom(m.Code[fi].BodyBytes)
			for r.BytesLeft() > 0 {
				op, err := r.Byte()
				if err != nil {
					t.Fatal(err)
				}
				look := r
				var imm wasm.InstructionImmediate
				if err := classifier.ClassifyInto(&r, op, &imm); err != nil {
					t.Fatal(err)
				}
				if op == 3 {
					bt, err := look.Byte()
					if err == nil && bt == 0x40 {
						var p regionLoopPlan
						if inspectRegionLoop(look, types, classifier, &p) && p.map32 && p.qualifyMap32() {
							f.Plans++
						}
					}
				}
			}
			rec.Hinted = append(rec.Hinted, f)
		}
		records = append(records, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	var functions, hinted, count, reserved, plans int
	for _, r := range records {
		functions += r.Functions
		hinted += len(r.Hinted)
		for _, f := range r.Hinted {
			count += f.Hints
			reserved += f.Reserve
			plans += f.Plans
		}
	}
	t.Logf("modules=%d functions=%d hinted-functions=%d hints=%d reserve-before-module-cap=%d full-static-plans-in-hinted-functions=%d", len(records), functions, hinted, count, reserved, plans)
}
