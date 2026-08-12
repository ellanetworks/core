// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

// buildMobileIdentity renders a decoded 5GS mobile identity (TS 24.501 §9.11.3.4)
// into its typed JSON form: SUCI, 5G-GUTI, IMEI, 5G-S-TMSI, or IMEISV.
func buildMobileIdentity(id fgs.MobileIdentity) MobileIdentity {
	toi := uint8(id.Type())

	switch {
	case id.SUCI != nil:
		suci := id.SUCI.String()
		mid := MobileIdentity{Identity: utils.MakeEnum(toi, "SUCI", false), SUCI: &suci}

		// A NAI-format SUPI (TS 24.501 §9.11.3.4) carries no PLMN.
		if id.SUCI.Format == fgs.SUPIFormatIMSI {
			plmnIDModel := PLMNID{Mcc: id.SUCI.PLMN.MCC, Mnc: id.SUCI.PLMN.MNC}
			mid.PLMNID = &plmnIDModel
		}

		return mid
	case id.GUTI != nil:
		guti := gutiToString(*id.GUTI)
		return MobileIdentity{GUTI: &guti, Identity: utils.MakeEnum(toi, "5G-GUTI", false)}
	case id.STMSI != nil:
		sTmsi := stmsiToString(*id.STMSI)
		return MobileIdentity{STMSI: &sTmsi, Identity: utils.MakeEnum(toi, "5G-S-TMSI", false)}
	case id.PEI != nil && id.Type() == fgs.IdentityIMEI:
		imei := id.PEI.String()
		return MobileIdentity{Identity: utils.MakeEnum(toi, "IMEI", false), IMEI: &imei}
	case id.PEI != nil:
		imeisv := id.PEI.String()
		return MobileIdentity{Identity: utils.MakeEnum(toi, "IMEISV", false), IMEISV: &imeisv}
	case id.Type() == fgs.IdentityNoIdentity:
		return MobileIdentity{Identity: utils.MakeEnum(toi, "No Identity", false)}
	default:
		return MobileIdentity{Identity: utils.MakeEnum(toi, "", true)}
	}
}

// gutiToString renders a 5G-GUTI as PLMN identity, AMF identifier and 5G-TMSI
// concatenated in hex (TS 24.501 §9.11.3.4).
func gutiToString(g fgs.GUTI) string {
	amf, err := fgs.AMFIdentifier{RegionID: g.AMFRegionID, SetID: g.AMFSetID, Pointer: g.AMFPointer}.MarshalBinary()
	if err != nil {
		return ""
	}

	return g.PLMN.MCC + g.PLMN.MNC + hex.EncodeToString(amf) + hex.EncodeToString(g.TMSI[:])
}

// stmsiToString renders a 5G-S-TMSI as the AMF Set ID, AMF Pointer and 5G-TMSI
// concatenated in hex (TS 24.501 §9.11.3.4).
func stmsiToString(s fgs.STMSI) string {
	raw, err := s.MarshalBinary()
	if err != nil {
		return ""
	}

	return hex.EncodeToString(raw[1:])
}

func buildTypeOfIdentityEnum(toi uint8) utils.EnumField {
	return utils.NamedEnum(toi, fgs.MobileIdentityType(toi).Name())
}

// decodePDUSessionStatus renders a PDU session status IE value as one entry per PDU
// session identity (0-15).
func decodePDUSessionStatus(bitmap *fgs.PSIBitmap) []PDUSessionStatusPDU {
	if bitmap == nil {
		return nil
	}

	psi := bitmap.PSI
	out := []PDUSessionStatusPDU{}

	for i := range 16 {
		out = append(out, PDUSessionStatusPDU{PDUSessionID: i, Active: psi[i]})
	}

	return out
}
