package models

type AuthenticationVector struct {
	AvType   AvType
	Rand     string
	Xres     string
	Autn     string
	CkPrime  string
	IkPrime  string
	XresStar string
	Kausf    string
}
