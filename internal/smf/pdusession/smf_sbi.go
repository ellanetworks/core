package pdusession

import "context"

type EllaSmfSbi struct{}

func (s *EllaSmfSbi) ActivateSmContext(smContextRef string) ([]byte, error) {
	return ActivateSmContext(smContextRef)
}

func (s *EllaSmfSbi) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	return ReleaseSmContext(ctx, smContextRef)
}

func (s *EllaSmfSbi) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	return UpdateSmContextXnHandoverPathSwitchReq(ctx, smContextRef, n2Data)
}

func (s *EllaSmfSbi) UpdateSmContextHandoverFailed(smContextRef string, n2Data []byte) error {
	return UpdateSmContextHandoverFailed(smContextRef, n2Data)
}
