// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"math"
	"net"

	"github.com/ellanetworks/core/internal/util/idgenerator"
)

type UPF struct {
	N3Interface net.IP

	pdrIDGenerator *idgenerator.IDGenerator
	farIDGenerator *idgenerator.IDGenerator
	barIDGenerator *idgenerator.IDGenerator
	qerIDGenerator *idgenerator.IDGenerator

	NodeID net.IP
}

func NewUPF(nodeID net.IP) *UPF {
	upf := new(UPF)
	upf.NodeID = nodeID
	upf.pdrIDGenerator = idgenerator.NewGenerator(1, math.MaxUint16)
	upf.farIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)
	upf.barIDGenerator = idgenerator.NewGenerator(1, math.MaxUint8)
	upf.qerIDGenerator = idgenerator.NewGenerator(1, math.MaxUint32)
	upf.N3Interface = nil

	return upf
}

func (upf *UPF) pdrID() (uint16, error) {
	pdrID, err := upf.pdrIDGenerator.Allocate()
	if err != nil {
		return 0, fmt.Errorf("could not allocate PDR ID: %v", err)
	}

	return uint16(pdrID), nil
}

func (upf *UPF) farID() (uint32, error) {
	tmpID, err := upf.farIDGenerator.Allocate()
	if err != nil {
		return 0, err
	}

	return uint32(tmpID), nil
}

func (upf *UPF) qerID() (uint32, error) {
	tmpID, err := upf.qerIDGenerator.Allocate()
	if err != nil {
		return 0, err
	}

	return uint32(tmpID), nil
}

func (upf *UPF) AddPDR() (*PDR, error) {
	pdrID, err := upf.pdrID()
	if err != nil {
		return nil, err
	}

	pdr := new(PDR)
	pdr.PDRID = pdrID

	newFAR, err := upf.AddFAR()
	if err != nil {
		return nil, err
	}

	pdr.FAR = newFAR

	return pdr, nil
}

func (upf *UPF) AddFAR() (*FAR, error) {
	farID, err := upf.farID()
	if err != nil {
		return nil, err
	}

	far := new(FAR)
	far.ApplyAction.Drop = true
	far.FARID = farID

	return far, nil
}

func (upf *UPF) AddQER() (*QER, error) {
	qerID, err := upf.qerID()
	if err != nil {
		return nil, err
	}

	qer := new(QER)
	qer.QERID = qerID

	return qer, nil
}

func (upf *UPF) RemovePDR(pdr *PDR) {
	upf.pdrIDGenerator.FreeID(int64(pdr.PDRID))
}

func (upf *UPF) RemoveFAR(far *FAR) {
	upf.farIDGenerator.FreeID(int64(far.FARID))
}

func (upf *UPF) RemoveBAR(bar *BAR) {
	upf.barIDGenerator.FreeID(int64(bar.BARID))
}

func (upf *UPF) RemoveQER(qer *QER) {
	upf.qerIDGenerator.FreeID(int64(qer.QERID))
}
