package pdusession

import "context"

type EllaSmfSbi struct{}

func (s EllaSmfSbi) ActivateSmContext(smContextRef string) ([]byte, error) {
	return ActivateSmContext(smContextRef)
}

func (s EllaSmfSbi) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	return ReleaseSmContext(ctx, smContextRef)
}
