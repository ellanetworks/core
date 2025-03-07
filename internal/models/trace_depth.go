package models

type TraceDepth string

const (
	TraceDepthMinimum                  TraceDepth = "MINIMUM"
	TraceDepthMedium                   TraceDepth = "MEDIUM"
	TraceDepthMaximum                  TraceDepth = "MAXIMUM"
	TraceDepthMinimumWoVendorExtension TraceDepth = "MINIMUM_WO_VENDOR_EXTENSION"
	TraceDepthMediumWoVendorExtension  TraceDepth = "MEDIUM_WO_VENDOR_EXTENSION"
	TraceDepthMaximumWoVendorExtension TraceDepth = "MAXIMUM_WO_VENDOR_EXTENSION"
)
