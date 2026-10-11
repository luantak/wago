//go:build linux && amd64

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

	"github.com/wago-org/wago/src/core/compiler/wasm"
)

// This evidence tool inspects the original instructions, including loops that
// the bounded planner rejects. It does not change or execute guest modules.
func TestDirectMap32Corpus(t *testing.T) {
	root := os.Getenv("DIRECT_MAP32_CORPUS")
	if root == "" {
		t.Skip("corpus root not set")
	}
	type candidate struct {
		Function, PC int
		Reason       string
		Ops          string
	}
	type record struct {
		Path, Hash                 string
		Functions, Loops, Accepted int
		Candidates                 []candidate
		Error                      string
	}
	var records []record
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
		classifier := wasm.NewModuleInstructionClassifier(m, true)
		rec.Functions = len(m.Code)
		for fi, fn := range m.Code {
			ft, ok := m.LocalFuncType(fi)
			if !ok {
				continue
			}
			var types []machineType
			for _, typ := range ft.Params {
				types = append(types, mtOf(typ))
			}
			for _, run := range fn.Locals.Runs {
				for i := uint32(0); i < run.Count; i++ {
					types = append(types, mtOf(run.Type))
				}
			}
			r := wasm.ReaderFrom(fn.BodyBytes)
			for r.BytesLeft() > 0 {
				pc := r.Offset()
				op, err := r.Byte()
				if err != nil {
					break
				}
				look := r
				var imm wasm.InstructionImmediate
				if classifier.ClassifyInto(&r, op, &imm) != nil {
					break
				}
				if op != 3 {
					continue
				}
				rec.Loops++
				bt, err := look.Byte()
				if err != nil || bt != 0x40 {
					continue
				}
				var p regionLoopPlan
				accepted := inspectRegionLoop(look, types, classifier, &p) && p.map32 && p.qualifyMap32()
				if accepted {
					rec.Accepted++
				}
				walk := regionLoopReader(look)
				var ops []byte
				load, mul, add, store := false, false, false, false
				for step := 0; step < regionLoopMaxOps && walk.BytesLeft() > 0; step++ {
					x, err := walk.Byte()
					if err != nil {
						break
					}
					var in wasm.InstructionImmediate
					if classifier.ClassifyInto(&walk, x, &in) != nil {
						break
					}
					ops = append(ops, x)
					load = load || x == 0x28 || x == 0x2a
					mul = mul || x == 0x6c || x == 0x94
					add = add || x == 0x6a || x == 0x92
					store = store || x == 0x36 || x == 0x38
					if x == 0xb {
						break
					}
				}
				if accepted || load && mul && add && store {
					reason := "bounded grammar or lane proof"
					if accepted {
						reason = "plan accepted; native register and exit checks remain"
					}
					rec.Candidates = append(rec.Candidates, candidate{fi + m.ImportedFuncCount(), pc + int(fn.LocalDeclBytes), reason, fmt.Sprintf("%x", ops)})
				}
			}
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
	out := os.Getenv("DIRECT_MAP32_COVERAGE")
	if out == "" {
		t.Fatal("coverage output not set")
	}
	if err := os.WriteFile(out, data, 0644); err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		t.Logf("%s functions=%d loops=%d plans=%d near=%d error=%s", r.Path, r.Functions, r.Loops, r.Accepted, len(r.Candidates), r.Error)
	}
}
