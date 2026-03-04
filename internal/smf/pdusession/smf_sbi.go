package pdusession

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
)

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

func (s *EllaSmfSbi) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*models.UpdateSmContextResponse, error) {
	return UpdateSmContextN1Msg(ctx, smContextRef, n1Msg)
}

func (s *EllaSmfSbi) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	return CreateSmContext(ctx, supi, pduSessionID, dnn, snssai, n1Msg)
}

func (s *EllaSmfSbi) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	return UpdateSmContextCauseDuplicatePDUSessionID(ctx, smContextRef)
}
