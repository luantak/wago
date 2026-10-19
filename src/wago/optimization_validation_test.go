package wago

import (
	"strings"
	"testing"
)

func TestKnownOptimizationSelectionsRetainValidationTrust(t *testing.T) {
	base := NewRuntimeConfig()
	infos := base.OptimizationInfos()
	if len(infos) == 0 {
		t.Fatal("missing optimization catalog")
	}
	all := make(map[string]bool, len(infos))
	for _, i := range infos {
		all[i.Name] = i.On
	}
	for name, cfg := range map[string]*RuntimeConfig{
		"single": base.WithOptimization(infos[0].Name, !infos[0].On),
		"bulk":   base.WithOptimizations(all),
		"empty":  base.WithOptimizations(nil),
	} {
		if !cfg.trustedOptimizations {
			t.Fatalf("%s lost validation trust for owned known names", name)
		}
	}
	bad := base.WithOptimization("unknown-optimization-trust-test", true)
	for _, cfg := range []*RuntimeConfig{bad, bad.WithOptimization(infos[0].Name, true), bad.WithOptimizations(all)} {
		if cfg.trustedOptimizations {
			t.Fatal("unvalidated prior name became trusted")
		}
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "unknown-optimization-trust-test") {
			t.Fatalf("unknown name was not rejected: %v", err)
		}
	}
	owned := base.WithOptimizations(all)
	all["external-mutation-trust-test"] = true
	if !owned.trustedOptimizations {
		t.Fatal("caller map mutation affected trust")
	}
	if _, ok := owned.optimizations["external-mutation-trust-test"]; ok {
		t.Fatal("caller map leaked into config")
	}
	if base.WithOptimization("stack-fence", false).Validate() == nil {
		t.Fatal("trusted names bypassed required stack-fence policy")
	}
}

func BenchmarkRuntimeOptimizationValidation(b *testing.B) {
	base := NewRuntimeConfig()
	all := map[string]bool{}
	for _, i := range base.OptimizationInfos() {
		all[i.Name] = i.On
	}
	for name, cfg := range map[string]*RuntimeConfig{"default": base, "explicit": base.WithOptimizations(all)} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := cfg.Validate(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
