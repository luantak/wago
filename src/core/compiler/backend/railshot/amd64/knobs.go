//go:build amd64

package amd64

import (
	"github.com/wago-org/wago/src/core/compiler/backend/railshot/shared"
	"github.com/wago-org/wago/src/core/compiler/optimization"
)

// The binding inventory is checked against the amd64 Optimization Definitions
// during package initialization. Public sense is always "on = enabled".
var optimizationBindings = optimization.NewBindings("amd64",
	optimization.Bind("loop-invariant-store", &loopInvariantStoreEnabled),
	optimization.Bind("loop-invariant-store-unroll", &loopInvariantStoreUnrollEnabled),
	optimization.Bind("recursive-inline-budget", &recursiveInlineBudgetEnabled),
	optimization.Bind("simd-literal-operand", &simdLiteralOperandEnabled),
	optimization.Bind("integer-packet-input-hoist", &packetInputHoistEnabled),
	optimization.Bind("integer-packet-program", &integerPacketProgramEnabled),
	optimization.Bind("integer-packet-vector", &integerPacketVectorEnabled),
	optimization.Bind("structured-loop-range", &structuredLoopRangeEnabled),
	optimization.Bind("conditional-memory-store", &conditionalMemoryStoreEnabled),
	optimization.Bind("deferred-mem-left", &deferredMemLeftEnabled),
	optimization.Bind("owned-pinned-tee", &ownedPinnedTeeEnabled),
	optimization.Bind("forward-byte-copy", &forwardByteCopyEnabled),
	optimization.Bind("branch-address-maximum", &branchAddressMaximumEnabled),
	optimization.Bind("short-select-mul", &shortSelectMulEnabled),
	optimization.Bind("bulk-ready-args", &bulkReadyArgsEnabled),
	optimization.Bind("comparison-accumulate", &comparisonAccumulateEnabled),
	optimization.Bind("integer-select-group", &selectGroupEnabled),
	optimization.Bind("boolean-chain-depth", &booleanChainDepthEnabled),
	optimization.Bind("lea-double", &leaDoubleEnabled),
	optimization.Bind("bounded-remainder-lea", &boundedRemainderLeaEnabled),
	optimization.Bind("byte-store-run", &byteStoreRunEnabled),
	optimization.Bind("byte-store-movzx", &byteStoreMovzxEnabled),
	optimization.Bind("scaled-jump-table", &scaledJumpTableEnabled),
	optimization.Bind("select-local-sink", &selectLocalSinkEnabled),
	optimization.Bind("select-flags-sink", &selectFlagsSinkEnabled),
	optimization.Bind("signed-load-group", &signedLoadGroupEnabled),
	optimization.Bind("avoid-regional-sink-spill", &avoidRegionalSinkSpillEnabled),
	optimization.Bind("select-frame", &selectFrameEnabled),
	optimization.Bind("memory-update", &memoryUpdateEnabled),
	optimization.Bind("immediate-remainder", &immediateRemainderEnabled),
	optimization.Bind("pressure-compact", &pressureCompactEnabled),
	optimization.Bind("wrapped-load-group", &wrappedLoadGroupEnabled),
	optimization.Bind("wide-call-pin-budget", &wideCallPinBudgetEnabled),
	optimization.Bind("local-root-realize", &localRootRealizeEnabled),
	optimization.Bind("local-root-pressure", &localRootPressureEnabled),
	optimization.Bind("rounddown-div", &rounddownDivEnabled),
	optimization.Bind("zero-store-pair", &zeroStorePairEnabled),
	optimization.Bind("canonical-rounddown", &canonicalRounddownEnabled),
	optimization.Bind("wide-unsigned-mul-high", &wideUnsignedMulHighEnabled),
	optimization.Bind("short-select-guard", &shortSelectGuardEnabled),
	optimization.Bind("cheap-select-guard", &cheapSelectGuardEnabled),
	optimization.Bind("inline-leaf-wrapper", &inlineLeafWrapperEnabled),
	optimization.Bind("bmi1-andnot", &bmi1AndNotEnabled),
	optimization.Bind("linear-integer-collect", &linearIntegerTreeEnabled),
	optimization.Bind("byte-bounds-direct", &byteBoundsDirectEnabled),
	optimization.Bind("avoid-sink-spill", &avoidSinkSpillEnabled),
	optimization.Bind("pure-loop-vector", &pureReduceEnabled),
	optimization.Bind("select-pure-guard", &selectPureGuardEnabled),
	optimization.Bind("masked-truth", &maskedTruthEnabled),
	optimization.Bind("boolean-chain", &booleanChainEnabled),
	optimization.Bind("integer-region-vector", &integerRegionVectorEnabled),
	optimization.Bind("boolean-chain-operands", &booleanChainOperandsEnabled),
	optimization.Bind("bmi2-shift", &bmi2ShiftEnabled),
	optimization.Bind("integer-memory-vector", &integerMemoryVectorEnabled),
	optimization.Bind("integer-counted-vector", &integerCountedVectorEnabled),
	optimization.Bind("integer-paired-loads", &integerPairedLoadsEnabled),
	optimization.Bind("integer-store-vector", &integerStoreVectorEnabled),
	optimization.Bind("integer-map-vector", &integerMapVectorEnabled),
	optimization.Bind("integer-map-group-guard", &integerMapGroupGuardEnabled),
	optimization.Bind("extended-loop-pin-scan", &extendedLoopPinScanEnabled),
	optimization.Bind("weighted-loop-pin-exchange", &weightedLoopPinExchangeEnabled),
	optimization.Bind("atomic-wait-zero-timeout", &atomicZeroWaitEnabled),
	optimization.Bind("shared-memory-base", &sharedMemoryBaseEnabled),
	optimization.Bind("atomic-notify-empty", &atomicNotifyEmptyEnabled),
	optimization.Bind("memory-min-bounds", &memoryMinimumBoundsEnabled),
	optimization.Bind("atomic-static-bounds", &atomicStaticBoundsEnabled),
	optimization.Bind("bounds-facts", &boundsFactsEnabled),
	optimization.Bind("simd-superopt", &simdSuperoptEnabled),
	optimization.Bind("simd-load-extract", &simdLoadExtractEnabled),
	optimization.Bind("simd-store-load-address", &simdStoreLoadAddressEnabled),
	optimization.Bind("bulk-fill-vector", &bulkFillVectorEnabled),
	optimization.Bind("straight-line-block", &straightLineBlockEnabled),
	optimization.Bind("simd-reduction-borrow", &simdReductionBorrowEnabled),
	optimization.Bind("prepared-direct-entry", &preparedDirectEntryEnabled),
	optimization.Bind("prepared-bounded-entry", &preparedBoundedEntryEnabled),
	optimization.Bind("direct-go-host-import", &directGoImportEnabled),
	optimization.Bind("native-leaf-host", &nativeLeafHostEnabled),
	optimization.Bind("wide-loop-int-const", &wideLoopIntConstEnabled),
	optimization.Bind("compact-loop-align32", &compactLoopAlign32Enabled),
	optimization.Bind("counted-loop-latch", &countedLoopLatchEnabled),
	optimization.Bind("linear-sum-loop", &linearSumLoopEnabled),
	optimization.Bind("callfree-loop-cold-exit", &callFreeLoopColdExitEnabled),
	optimization.Bind("callfree-loop-reg-state", &callFreeLoopRegStateEnabled),
	optimization.Bind("interval-region-pins", &intervalRegionPinsEnabled),
	optimization.Bind("interval-control", &intervalControlEnabled),
	optimization.Bind("interval-next-use", &intervalNextUseEnabled),
	optimization.Bind("interval-scratch-lease", &intervalScratchLeaseEnabled),
	optimization.Bind("interval-r8-lease", &intervalR8LeaseEnabled),
	optimization.Bind("interval-i64-weight", &intervalI64WeightEnabled),
	optimization.Bind("memsize-regional-lease", &memSizeRegionalLeaseEnabled),
	optimization.Bind("module-global-regional-lease", &moduleGlobalRegionalLeaseEnabled),
	optimization.Bind("magic-div", &magicDivEnabled),
	optimization.Bind("shared-trap-body", &sharedTrapBodyEnabled),
	optimization.Bind("shared-adapters", &sharedAdaptersEnabled),
	optimization.Bind("st-flags", &stFlagsEnabled),
	optimization.Bind("store8-flags", &store8FlagsEnabled),
	optimization.Bind("reg-merge", &regMergeEnabled),
	optimization.Bind("tee-sink", &teeLocalSinkEnabled),
	optimization.Bind("unary-sink", &unaryLocalSinkEnabled),
	optimization.Bind("branch-fold", &branchFoldEnabled),
	optimization.Bind("entry-arg-pins", &entryArgPinsEnabled),
	optimization.Bind("ext-fp-pins", &extendedFPPinsEnabled),
	optimization.Bind("tree-order", &treeOrderEnabled),
	optimization.Bind("assoc-tree", &associativeTreeEnabled),
	optimization.Bind("bmi2-rorx", &bmi2RorxEnabled),
	optimization.Bind("avx512-ternary", &avx512TernaryEnabled),
	optimization.Bind("avx512-vrotate", &avx512VRotateEnabled),
	optimization.Bind("vex-float-mem", &vexFloatMemEnabled),
	optimization.Bind("float-frame-mem", &floatFrameMemEnabled),
	optimization.Bind("float-store-borrow", &floatStoreBorrowEnabled),
	optimization.Bind("memory-compare-immediate", &memoryCompareEnabled),
	optimization.Bind("wide-local-pins", &wideLocalPinsEnabled),
	optimization.Bind("direct-int-branch-merge", &directIntBranchMergeEnabled),
	optimization.Bind("multi-bounds-cert", &multiBoundsCertEnabled),
	optimization.Bind("addr-zext-elim", &memory32AddrZExtElimEnabled),
	optimization.Bind("canonical-i32", &canonicalI32CarriersEnabled),
	optimization.Bind("value-facts", &valueFactsEnabled),
	optimization.Bind("immutable-table", &immutableLocalTableEnabled),
	optimization.Bind("immutable-table-type", &immutableTableTypeEnabled),
	optimization.Bind("inline-callfree", &inlineCallFreeHintsEnabled),
	optimization.Bind("store-forward", &linearStoreForwardEnabled),
	optimization.Bind("frame-elide", &smallFrameElideEnabled),
	optimization.Bind("compact-i32-frame", &compactI32FrameEnabled),
	optimization.Bind("local-slot-order", &localSlotOrderEnabled),
	optimization.Bind("commute-self-update", &commuteSelfUpdateEnabled),
	optimization.Bind("commute-fixed-self-update", &commuteFixedSelfUpdateEnabled),
	optimization.Bind("i64-mask32", &i64Mask32Enabled),
	optimization.Bind("accumulator-immediate", &accumulatorImmediateEnabled),
	optimization.Bind("dead-gc-new", &deadGCNewEnabled),
	optimization.Bind("gc-native-alloc", &nativeGCStructAllocEnabled),
	optimization.Bind("v128-const-cache", &v128ConstCacheEnabled),
	optimization.Bind("v128-pins", &v128LocalPinsEnabled),
	optimization.Bind("reg-abi", &regABIEnabled),
	optimization.Bind("inline", &inlineEnabled),
	optimization.Bind("inline-parameter-pins", &inlineParameterPinsEnabled),
	optimization.Bind("local-spill-reuse", &localSpillReuseEnabled),
	optimization.BindInverted("stack-fence", &noStackFence),
	optimization.BindInverted("stack-reg", &noStackReg),
	optimization.Bind("literal-pool-align", &literalPoolAlignEnabled),
	optimization.Bind("nested-loop-range", &nestedLoopRangeEnabled),
	optimization.Bind("select-compare-pair", &selectComparePairEnabled),
	optimization.Bind("pure-countdown-unroll", &pureCountdownUnrollEnabled),
	optimization.Bind("bmi1-reset-lowest-bit", &bmi1ResetLowestBitEnabled),
	optimization.Bind("unsigned-remainder-range", &unsignedRemainderRangeEnabled),
)

var (
	optLoopInvariantStore      = optimizationBindings.Option("loop-invariant-store")
	optInvariantPrefixUnroll   = optimizationBindings.Option("loop-invariant-store-unroll")
	optBMI1AndNot              = optimizationBindings.Option("bmi1-andnot")
	optByteBoundsDirect        = optimizationBindings.Option("byte-bounds-direct")
	optLinearIntegerTree       = optimizationBindings.Option("linear-integer-collect")
	optSharedMemoryBase        = optimizationBindings.Option("shared-memory-base")
	optMemoryMinimumBounds     = optimizationBindings.Option("memory-min-bounds")
	optAtomicNotifyEmpty       = optimizationBindings.Option("atomic-notify-empty")
	optAtomicStaticBounds      = optimizationBindings.Option("atomic-static-bounds")
	optAtomicZeroWait          = optimizationBindings.Option("atomic-wait-zero-timeout")
	optBoundsFacts             = optimizationBindings.Option("bounds-facts")
	optStraightLineBlock       = optimizationBindings.Option("straight-line-block")
	optBulkFillVector          = optimizationBindings.Option("bulk-fill-vector")
	optSIMDStoreLoadAddress    = optimizationBindings.Option("simd-store-load-address")
	optSIMDLoadExtract         = optimizationBindings.Option("simd-load-extract")
	optSIMDSuperopt            = optimizationBindings.Option("simd-superopt")
	optSIMDReductionBorrow     = optimizationBindings.Option("simd-reduction-borrow")
	optPreparedDirectEntry     = optimizationBindings.Option("prepared-direct-entry")
	optPreparedBoundedEntry    = optimizationBindings.Option("prepared-bounded-entry")
	optDirectGoHostImport      = optimizationBindings.Option("direct-go-host-import")
	optNativeLeafHost          = optimizationBindings.Option("native-leaf-host")
	optWideLoopIntConst        = optimizationBindings.Option("wide-loop-int-const")
	optCompactLoopAlign32      = optimizationBindings.Option("compact-loop-align32")
	optCountedLoopLatch        = optimizationBindings.Option("counted-loop-latch")
	optLinearSumLoop           = optimizationBindings.Option("linear-sum-loop")
	optPureLoopVector          = optimizationBindings.Option("pure-loop-vector")
	optSelectPureGuard         = optimizationBindings.Option("select-pure-guard")
	optMaskedTruth             = optimizationBindings.Option("masked-truth")
	optBooleanChain            = optimizationBindings.Option("boolean-chain")
	optIntegerRegionVector     = optimizationBindings.Option("integer-region-vector")
	optBooleanChainOperands    = optimizationBindings.Option("boolean-chain-operands")
	optBMI2Shift               = optimizationBindings.Option("bmi2-shift")
	optIntegerMemoryVector     = optimizationBindings.Option("integer-memory-vector")
	optIntegerCountedVector    = optimizationBindings.Option("integer-counted-vector")
	optIntegerPairedLoads      = optimizationBindings.Option("integer-paired-loads")
	optIntegerStoreVector      = optimizationBindings.Option("integer-store-vector")
	optIntegerMapVector        = optimizationBindings.Option("integer-map-vector")
	optIntegerMapGroupGuard    = optimizationBindings.Option("integer-map-group-guard")
	optWeightedLoopPinExchange = optimizationBindings.Option("weighted-loop-pin-exchange")
	optAvoidSinkSpill          = optimizationBindings.Option("avoid-sink-spill")
	optExtendedLoopPinScan     = optimizationBindings.Option("extended-loop-pin-scan")
	optCallFreeLoopColdExit    = optimizationBindings.Option("callfree-loop-cold-exit")
	optLoopRegState            = optimizationBindings.Option("callfree-loop-reg-state")
	optIntervalRegionPins      = optimizationBindings.Option("interval-region-pins")
	optIntervalControl         = optimizationBindings.Option("interval-control")
	optIntervalNextUse         = optimizationBindings.Option("interval-next-use")
	optIntervalScratchLease    = optimizationBindings.Option("interval-scratch-lease")
	optIntervalR8Lease         = optimizationBindings.Option("interval-r8-lease")
	optIntervalI64Weight       = optimizationBindings.Option("interval-i64-weight")
	optMemSizeRegionalLease    = optimizationBindings.Option("memsize-regional-lease")
	optModuleGlobalRegionLease = optimizationBindings.Option("module-global-regional-lease")
	optMagicDiv                = optimizationBindings.Option("magic-div")
	optSharedTrapBody          = optimizationBindings.Option("shared-trap-body")
	optSharedAdapters          = optimizationBindings.Option("shared-adapters")
	optSTFlags                 = optimizationBindings.Option("st-flags")
	optStore8Flags             = optimizationBindings.Option("store8-flags")
	optRegMerge                = optimizationBindings.Option("reg-merge")
	optTeeSink                 = optimizationBindings.Option("tee-sink")
	optUnarySink               = optimizationBindings.Option("unary-sink")
	optBranchFold              = optimizationBindings.Option("branch-fold")
	optEntryArgPins            = optimizationBindings.Option("entry-arg-pins")
	optExtendedFPPins          = optimizationBindings.Option("ext-fp-pins")
	optTreeOrder               = optimizationBindings.Option("tree-order")
	optAssocTree               = optimizationBindings.Option("assoc-tree")
	optBMI2Rorx                = optimizationBindings.Option("bmi2-rorx")
	optAVX512Ternary           = optimizationBindings.Option("avx512-ternary")
	optAVX512VRotate           = optimizationBindings.Option("avx512-vrotate")
	optWideLocalPins           = optimizationBindings.Option("wide-local-pins")
	optWideCallPinBudget       = optimizationBindings.Option("wide-call-pin-budget")
	optZeroStorePair           = optimizationBindings.Option("zero-store-pair")
	optMemoryUpdate            = optimizationBindings.Option("memory-update")
	optSelectFrame             = optimizationBindings.Option("select-frame")
	optAvoidRegionalSinkSpill  = optimizationBindings.Option("avoid-regional-sink-spill")
	optSignedLoadGroup         = optimizationBindings.Option("signed-load-group")
	optSelectLocalSink         = optimizationBindings.Option("select-local-sink")
	optSelectFlagsSink         = optimizationBindings.Option("select-flags-sink")
	optScaledJumpTable         = optimizationBindings.Option("scaled-jump-table")
	optLeaDouble               = optimizationBindings.Option("lea-double")
	optComparisonAccumulate    = optimizationBindings.Option("comparison-accumulate")
	optShortSelectMul          = optimizationBindings.Option("short-select-mul")
	optBulkReadyArgs           = optimizationBindings.Option("bulk-ready-args")
	optStructuredLoopRange     = optimizationBindings.Option("structured-loop-range")
	optIntegerPacketVector     = optimizationBindings.Option("integer-packet-vector")
	optIntegerPacketProgram    = optimizationBindings.Option("integer-packet-program")
	optPacketInputHoist        = optimizationBindings.Option("integer-packet-input-hoist")
	optSIMDLiteralOperand      = optimizationBindings.Option("simd-literal-operand")
	optRecursiveInlineBudget   = optimizationBindings.Option("recursive-inline-budget")
	optConditionalMemoryStore  = optimizationBindings.Option("conditional-memory-store")
	optDeferredMemLeft         = optimizationBindings.Option("deferred-mem-left")
	optOwnedPinnedTee          = optimizationBindings.Option("owned-pinned-tee")
	optForwardByteCopy         = optimizationBindings.Option("forward-byte-copy")
	optBranchAddressMaximum    = optimizationBindings.Option("branch-address-maximum")
	optBooleanChainDepth       = optimizationBindings.Option("boolean-chain-depth")
	optSelectGroupGuard        = optimizationBindings.Option("integer-select-group")
	optBoundedRemainderLea     = optimizationBindings.Option("bounded-remainder-lea")
	optByteStoreRun            = optimizationBindings.Option("byte-store-run")
	optByteStoreMovzx          = optimizationBindings.Option("byte-store-movzx")
	optCanonicalRounddown      = optimizationBindings.Option("canonical-rounddown")
	optDirectIntBranchMerge    = optimizationBindings.Option("direct-int-branch-merge")
	optFloatFrameMem           = optimizationBindings.Option("float-frame-mem")
	optFloatStoreBorrow        = optimizationBindings.Option("float-store-borrow")
	optMemoryCompareImmediate  = optimizationBindings.Option("memory-compare-immediate")
	optVEXFloatMem             = optimizationBindings.Option("vex-float-mem")
	optMultiBoundsCert         = optimizationBindings.Option("multi-bounds-cert")
	optAddrZExtElim            = optimizationBindings.Option("addr-zext-elim")
	optCanonicalI32            = optimizationBindings.Option("canonical-i32")
	optValueFacts              = optimizationBindings.Option("value-facts")
	optImmutableTable          = optimizationBindings.Option("immutable-table")
	optImmutableTableType      = optimizationBindings.Option("immutable-table-type")
	optInlineCallFree          = optimizationBindings.Option("inline-callfree")
	optStoreForward            = optimizationBindings.Option("store-forward")
	optFrameElide              = optimizationBindings.Option("frame-elide")
	optCompactI32Frame         = optimizationBindings.Option("compact-i32-frame")
	optLocalSlotOrder          = optimizationBindings.Option("local-slot-order")
	optCommuteSelfUpdate       = optimizationBindings.Option("commute-self-update")
	optCommuteFixedSelfUpdate  = optimizationBindings.Option("commute-fixed-self-update")
	optI64Mask32               = optimizationBindings.Option("i64-mask32")
	optAccumulatorImmediate    = optimizationBindings.Option("accumulator-immediate")
	optDeadGCNew               = optimizationBindings.Option("dead-gc-new")
	optGCNativeAlloc           = optimizationBindings.Option("gc-native-alloc")
	optV128ConstCache          = optimizationBindings.Option("v128-const-cache")
	optV128Pins                = optimizationBindings.Option("v128-pins")
	optRegABI                  = optimizationBindings.Option("reg-abi")
	optInlineParameterPins     = optimizationBindings.Option("inline-parameter-pins")
	optInlineLeafWrapper       = optimizationBindings.Option("inline-leaf-wrapper")
	optCheapSelectGuard        = optimizationBindings.Option("cheap-select-guard")
	optShortSelectGuard        = optimizationBindings.Option("short-select-guard")
	optImmediateRemainder      = optimizationBindings.Option("immediate-remainder")
	optPressureCompact         = optimizationBindings.Option("pressure-compact")
	optWrappedLoadGroup        = optimizationBindings.Option("wrapped-load-group")
	optLocalRootRealize        = optimizationBindings.Option("local-root-realize")
	optLocalRootPressure       = optimizationBindings.Option("local-root-pressure")
	optRounddownDiv            = optimizationBindings.Option("rounddown-div")
	optWideUnsignedMulHigh     = optimizationBindings.Option("wide-unsigned-mul-high")
	optLocalSpillReuse         = optimizationBindings.Option("local-spill-reuse")
	optInline                  = optimizationBindings.Option("inline")
	optStackFence              = optimizationBindings.Option("stack-fence")
	optStackReg                = optimizationBindings.Option("stack-reg")
	optLiteralPoolAlign        = optimizationBindings.Option("literal-pool-align")
	optNestedLoopRange         = optimizationBindings.Option("nested-loop-range")
	optSelectComparePair       = optimizationBindings.Option("select-compare-pair")
	optPureCountdownUnroll     = optimizationBindings.Option("pure-countdown-unroll")
	optBMI1ResetLowestBit      = optimizationBindings.Option("bmi1-reset-lowest-bit")
	optUnsignedRemainderRange  = optimizationBindings.Option("unsigned-remainder-range")
)

type KnobInfo = optimization.Info
type OptimizationSnapshot = optimization.Snapshot
type CodegenPolicy = shared.CodegenPolicy

func OptKnobs() []KnobInfo { return optimizationBindings.Infos() }

func OptKnobSnapshot() ([]KnobInfo, OptimizationSnapshot) { return optimizationBindings.Snapshot() }

func CurrentOptKnobSnapshot() OptimizationSnapshot { return optimizationBindings.CurrentSnapshot() }

func SetOptKnob(name string, on bool) bool { return optimizationBindings.Set(name, on) }

func currentCodegenPolicy() CodegenPolicy {
	selection, err := optimizationBindings.ResolveSnapshot(nil, OptimizationSnapshot{}, nil)
	if err != nil {
		panic(err)
	}
	return shared.DefaultCodegenPolicy(selection)
}
