package models

type TraceDepth string

const (
	TraceDepth_MINIMUM                     TraceDepth = "MINIMUM"
	TraceDepth_MEDIUM                      TraceDepth = "MEDIUM"
	TraceDepth_MAXIMUM                     TraceDepth = "MAXIMUM"
	TraceDepth_MINIMUM_WO_VENDOR_EXTENSION TraceDepth = "MINIMUM_WO_VENDOR_EXTENSION"
	TraceDepth_MEDIUM_WO_VENDOR_EXTENSION  TraceDepth = "MEDIUM_WO_VENDOR_EXTENSION"
	TraceDepth_MAXIMUM_WO_VENDOR_EXTENSION TraceDepth = "MAXIMUM_WO_VENDOR_EXTENSION"
)
