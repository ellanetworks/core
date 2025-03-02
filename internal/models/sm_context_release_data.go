package models

type SmContextReleaseData struct {
	Cause             Cause
	NgApCause         *NgApCause
	Var5gMmCauseValue int32
	UeTimeZone        string
	N2SmInfo          *RefToBinaryData
	N2SmInfoType      N2SmInfoType
}
