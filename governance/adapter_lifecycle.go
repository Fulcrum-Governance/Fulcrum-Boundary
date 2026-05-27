package governance

// AdapterLifecycleStep names one stage in the governed action lifecycle.
type AdapterLifecycleStep string

const (
	AdapterStepParse       AdapterLifecycleStep = "parse"
	AdapterStepIdentify    AdapterLifecycleStep = "identify"
	AdapterStepEvaluate    AdapterLifecycleStep = "evaluate"
	AdapterStepDeny        AdapterLifecycleStep = "deny"
	AdapterStepForward     AdapterLifecycleStep = "forward"
	AdapterStepInspect     AdapterLifecycleStep = "inspect"
	AdapterStepMetadata    AdapterLifecycleStep = "metadata"
	AdapterStepRecord      AdapterLifecycleStep = "record"
	AdapterStepBypassProof AdapterLifecycleStep = "bypass_proof"
	AdapterStepFailClosed  AdapterLifecycleStep = "fail_closed"
)

// AdapterLifecycleSteps is the complete production-readiness checklist for an adapter.
var AdapterLifecycleSteps = []AdapterLifecycleStep{
	AdapterStepParse,
	AdapterStepIdentify,
	AdapterStepEvaluate,
	AdapterStepDeny,
	AdapterStepForward,
	AdapterStepInspect,
	AdapterStepMetadata,
	AdapterStepRecord,
	AdapterStepBypassProof,
	AdapterStepFailClosed,
}

// AdapterStepImplementation describes how a lifecycle step is satisfied today.
type AdapterStepImplementation string

const (
	AdapterStepImplemented   AdapterStepImplementation = "implemented"
	AdapterStepDelegated     AdapterStepImplementation = "delegated"
	AdapterStepNotApplicable AdapterStepImplementation = "not_applicable"
	AdapterStepStub          AdapterStepImplementation = "stub"
)

// AdapterMaturity describes how the adapter may be represented publicly.
type AdapterMaturity string

const (
	AdapterMaturityExperimental AdapterMaturity = "experimental"
	AdapterMaturityPreview      AdapterMaturity = "preview"
	AdapterMaturityProduction   AdapterMaturity = "production"
)
