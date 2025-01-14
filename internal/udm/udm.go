// Copyright 2024 Ella Networks

package udm

const (
	UDM_HNP_PRIVATE_KEY = "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a"
)

func Start() error {
	udmContext.SuciProfiles = []SuciProfile{
		{
			ProtectionScheme: "1", // Standard defined value for Protection Scheme A (TS 33.501 Annex C)
			PrivateKey:       UDM_HNP_PRIVATE_KEY,
		},
	}
	return nil
}
