// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"reflect"
)

type BPManager struct {
	// Need these variable conducting Add additional PSA (TS23.502 4.3.5.4)
	// There value will change from time to time
	ULCL                  *UPF
	ActivatingPath        *DataPath
	UpdatedBranchingPoint map[*UPF]int
	PendingUPF            PendingUPF
	ActivatedPaths        []*DataPath
	BPStatus              BPStatus
	AddingPSAState        AddingPSAState
}

type BPStatus int

const (
	UnInitialized BPStatus = iota
	AddingPSA
	AddPSASuccess
	InitializedSuccess
	InitializedFail
)

type AddingPSAState int

const (
	ActivatingDataPath AddingPSAState = iota
	EstablishingNewPSA
	EstablishingULCL
	UpdatingPSA2DownLink
	UpdatingRANAndIUPFUpLink
	Finished
)

type PendingUPF map[string]bool

func (bpMGR *BPManager) SelectPSA2(smContext *SMContext) {
	hasSelectPSA2 := false
	bpMGR.ActivatedPaths = []*DataPath{}
	for _, dataPath := range smContext.Tunnel.DataPathPool {
		if dataPath.Activated {
			bpMGR.ActivatedPaths = append(bpMGR.ActivatedPaths, dataPath)
		} else {
			if !hasSelectPSA2 {
				bpMGR.ActivatingPath = dataPath
				hasSelectPSA2 = true
			}
		}
	}
}

func (bpMGR *BPManager) FindULCL(smContext *SMContext) error {
	bpMGR.UpdatedBranchingPoint = make(map[*UPF]int)
	activatingPath := bpMGR.ActivatingPath
	for _, psa1Path := range bpMGR.ActivatedPaths {
		depth := 0
		psa1CurDPNode := psa1Path.FirstDPNode
		for psa2CurDPNode := activatingPath.FirstDPNode; psa2CurDPNode != nil; psa2CurDPNode = psa2CurDPNode.Next() {
			if reflect.DeepEqual(psa2CurDPNode.UPF.NodeID, psa1CurDPNode.UPF.NodeID) {
				psa1CurDPNode = psa1CurDPNode.Next()
				depth++

				if _, exist := bpMGR.UpdatedBranchingPoint[psa2CurDPNode.UPF]; !exist {
					bpMGR.UpdatedBranchingPoint[psa2CurDPNode.UPF] = depth
				}
			} else {
				break
			}
		}
	}

	maxDepth := 0
	for upf, depth := range bpMGR.UpdatedBranchingPoint {
		if depth > maxDepth {
			bpMGR.ULCL = upf
			maxDepth = depth
		}
	}
	return nil
}

func (pendingUPF PendingUPF) IsEmpty() bool {
	if len(pendingUPF) == 0 {
		return true
	} else {
		return false
	}
}
