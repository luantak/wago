//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/wasm"
	"os"
)

var inlineLeafWrapperEnabled = os.Getenv("WAGO_AMD64_INLINE_LEAF_WRAPPER") == "1"

func wrapperInlineCandidate(m *wasm.Module, hints []funcHints, i int, facts inlineFacts, policy CodegenPolicy) bool {
	if !policy.EnabledOption(optInlineLeafWrapper) || policy.CompactNative || facts.calleeCount == 0 ||
		facts.bodyBytes > 64 || facts.hasLoop || facts.declaredLocals != 0 || !facts.regABIIntOnly ||
		hints[i].hasNonDirectCall() || hints[i].callRelocSiteCount() > 2 {
		return false
	}
	// Unlike a leaf splice, a wrapper retains inner call machinery. Bound its
	// aggregate expansion across call sites, not just the size of one copy.
	if facts.bodyBytes == 0 || facts.callSites > 128/facts.bodyBytes {
		return false
	}
	imported := m.ImportedFuncCount()
	actual, err := scanInlineFacts(m, m.Code[i], i, imported, wasm.NewModuleInstructionClassifier(m, true))
	if err != nil || actual.hasControlCall || actual.moduleEH {
		return false
	}
	for _, idx := range actual.callees {
		local := int(idx) - imported
		if local < 0 || local >= len(hints) || hints[local].flags.has(hintHasCall|hintModuleEH) {
			return false
		}
		ft, ok := m.LocalFuncType(local)
		if !ok || !sigFitsRegABI(ft) || !sigIsIntOnly(ft) {
			return false
		}
	}
	return len(actual.callees) != 0
}
