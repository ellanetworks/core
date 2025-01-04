// Copyright 2024 Ella Networks
// Copyright 2019 Communication Service/Software Laboratory, National Chiao Tung University (free5gc.org)
// SPDX-License-Identifier: Apache-2.0

package dnn

type Dnn []uint8

func (d *Dnn) MarshalBinary() (data []byte, err error) {
	data = append(data, uint8(len(*d)))
	data = append(data, (*d)...)

	return data, nil
}

func (d *Dnn) UnmarshalBinary(data []byte) error {
	(*d) = data[1:]
	return nil
}
