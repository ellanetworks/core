// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"strconv"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

func bit(bits []bool, i int) bool {
	return i < len(bits) && bits[i]
}

func ecidMeasurements(bits []bool) *models.ECIDMeasurements {
	return &models.ECIDMeasurements{
		RSRP:   bit(bits, lpptype.ECIDMeasSupportedRSRPSup),
		RSRQ:   bit(bits, lpptype.ECIDMeasSupportedRSRQSup),
		UERxTx: bit(bits, lpptype.ECIDMeasSupportedUERxTxSup),
	}
}

func nrECIDMeasurements(bits []bool) *models.NRECIDMeasurements {
	return &models.NRECIDMeasurements{
		SSRSRP:  bit(bits, lpptype.NRECIDMeasSupportedSSRSRPSup),
		SSRSRQ:  bit(bits, lpptype.NRECIDMeasSupportedSSRSRQSup),
		CSIRSRP: bit(bits, lpptype.NRECIDMeasSupportedCSIRSRPSup),
		CSIRSRQ: bit(bits, lpptype.NRECIDMeasSupportedCSIRSRQSup),
	}
}

func decodeECIDLocationInformation(r9 *lpptype.ProvideLocationInformationR9IEs, out *models.ProvideLocationInformation) {
	cells := lmfmodels.NewCellCollector(lmfmodels.MeasurementSourceUE)

	if e := r9.ECIDProvideLocationInformation; e != nil {
		if info := e.ECIDSignalMeasurementInformation; info != nil {
			if info.PrimaryCellMeasuredResults != nil {
				eutraCell(cells, info.PrimaryCellMeasuredResults).Serving = true
			}

			primaryOnly := info.PrimaryCellMeasuredResults == nil && len(info.MeasuredResultsList.List) == 1

			for i := range info.MeasuredResultsList.List {
				e := &info.MeasuredResultsList.List[i]
				cell := eutraCell(cells, e)
				cell.Serving = cell.Serving || primaryOnly || e.UERxTxTimeDiff != nil
			}
		}

		if e.ECIDError != nil && e.ECIDError.TargetDeviceErrorCauses != nil {
			cause := int64(e.ECIDError.TargetDeviceErrorCauses.Cause)
			out.ECIDErrorCause = &cause
		}
	}

	if r9.R16Additions != nil {
		if e := r9.R16Additions.NRECIDProvideLocationInformation; e != nil {
			if info := e.NRECIDSignalMeasurementInformation; info != nil {
				nrCell(cells, &info.NRPrimaryCellMeasuredResults).Serving = true

				for i := range info.NRMeasuredResultsList {
					nrCell(cells, &info.NRMeasuredResultsList[i])
				}
			}

			if e.NRECIDError != nil && e.NRECIDError.TargetDeviceErrorCauses != nil {
				cause := int64(e.NRECIDError.TargetDeviceErrorCauses.Cause)
				out.NRECIDErrorCause = &cause
			}
		}
	}

	out.Measurements = cells.Cells()
}

func eutraCell(cells *lmfmodels.CellCollector, e *lpptype.MeasuredResultsElement) *lmfmodels.CellMeasurement {
	arfcn := e.ARFCNEUTRA
	if e.V9a0Additions != nil && e.V9a0Additions.ARFCNEUTRA != nil {
		arfcn = *e.V9a0Additions.ARFCNEUTRA
	}

	cell := cells.Cell(lmfmodels.RATEUTRA, e.PhysCellID, arfcn)

	if cgi := e.CellGlobalID; cgi != nil && cgi.CellIdentity.EUTRA != nil {
		cell.ECGI = &coremodels.Ecgi{
			PlmnID:      plmnID(cgi.PLMNIdentity.MCC, cgi.PLMNIdentity.MNC),
			EutraCellID: cellIdentity(*cgi.CellIdentity.EUTRA, 7),
		}
	}

	ext := e.V1470Additions
	if ext == nil {
		ext = &lpptype.MeasuredResultsElementV1470Additions{}
	}

	switch {
	case ext.RSRPResult != nil:
		cell.RSRP = reportValue(ext.RSRPResult, lmfmodels.EUTRARSRPExtendedDBm)
	case e.RSRPResult != nil:
		cell.RSRP = reportValue(e.RSRPResult, lmfmodels.EUTRARSRPDBm)
	}

	switch {
	case ext.RSRQResult != nil:
		cell.RSRQ = reportValue(ext.RSRQResult, lmfmodels.EUTRARSRQExtendedDB)
	case e.RSRQResult != nil:
		cell.RSRQ = reportValue(e.RSRQResult, lmfmodels.EUTRARSRQDB)
	}

	cell.UERxTxTimeDiff = e.UERxTxTimeDiff

	return cell
}

func nrCell(cells *lmfmodels.CellCollector, e *lpptype.NRMeasuredResultsElement) *lmfmodels.CellMeasurement {
	var (
		arfcn     int64
		arfcnType lmfmodels.ARFCNType
	)

	switch {
	case e.NRARFCN.SSBARFCN != nil:
		arfcn, arfcnType = *e.NRARFCN.SSBARFCN, lmfmodels.ARFCNTypeSSB
	case e.NRARFCN.CSIRSPointA != nil:
		arfcn, arfcnType = *e.NRARFCN.CSIRSPointA, lmfmodels.ARFCNTypeCSIRSPointA
	}

	cell := cells.Cell(lmfmodels.RATNR, e.NRPhysCellID, arfcn)
	cell.ARFCNType = arfcnType

	if cgi := e.NRCellGlobalID; cgi != nil {
		cell.NCGI = &coremodels.Ncgi{
			PlmnID:   plmnID(cgi.MCC, cgi.MNC),
			NrCellID: cellIdentity(cgi.NRCellIdentity, 9),
		}
	}

	if r := e.ResultsSSBCell; r != nil {
		cell.SSRSRP = reportValue(r.NRRSRP, lmfmodels.NRRSRPDBm)
		cell.SSRSRQ = reportValue(r.NRRSRQ, lmfmodels.NRRSRQDB)
	}

	if r := e.ResultsCSIRSCell; r != nil {
		cell.CSIRSRP = reportValue(r.NRRSRP, lmfmodels.NRRSRPDBm)
		cell.CSIRSRQ = reportValue(r.NRRSRQ, lmfmodels.NRRSRQDB)
	}

	for _, r := range e.ResultsSSBIndexes {
		beam := lmfmodels.Beam(&cell.SSBBeams, r.SSBIndex)
		beam.RSRP = reportValue(r.SSBResults.NRRSRP, lmfmodels.NRRSRPDBm)
		beam.RSRQ = reportValue(r.SSBResults.NRRSRQ, lmfmodels.NRRSRQDB)
	}

	for _, r := range e.ResultsCSIRSIndexes {
		beam := lmfmodels.Beam(&cell.CSIRSBeams, r.CSIRSIndex)
		beam.RSRP = reportValue(r.CSIRSResults.NRRSRP, lmfmodels.NRRSRPDBm)
		beam.RSRQ = reportValue(r.CSIRSResults.NRRSRQ, lmfmodels.NRRSRQDB)
	}

	return cell
}

func reportValue(v *int64, convert func(int64) float64) *float64 {
	if v == nil {
		return nil
	}

	out := convert(*v)

	return &out
}

func plmnID(mcc, mnc []lpptype.MCCMNCDigit) *coremodels.PlmnID {
	return &coremodels.PlmnID{Mcc: digits(mcc), Mnc: digits(mnc)}
}

func digits(d []lpptype.MCCMNCDigit) string {
	out := make([]byte, 0, len(d))
	for _, x := range d {
		out = strconv.AppendInt(out, x.Value, 10)
	}

	return string(out)
}

func cellIdentity(bits []bool, width int) string {
	var v uint64
	for _, b := range bits {
		v <<= 1
		if b {
			v |= 1
		}
	}

	s := strconv.FormatUint(v, 16)
	for len(s) < width {
		s = "0" + s
	}

	return s
}
