package context

// Supported Feature-1
const UpFunctionFeatures1Ueip uint16 = 1 << 2

type UPFunctionFeatures struct {
	SupportedFeatures  uint16
	SupportedFeatures1 uint16
	SupportedFeatures2 uint16
}
