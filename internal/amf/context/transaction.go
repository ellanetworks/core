// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

type EventChannel struct {
	Message     chan interface{}
	Event       chan string
	AmfUe       *AmfUe
	NasHandler  func(*AmfUe, NasMsg)
	NgapHandler func(*AmfUe, NgapMsg)
	SbiHandler  func(s1, s2 string, msg interface{}) (interface{}, string, interface{}, interface{})
}

func (tx *EventChannel) UpdateNgapHandler(handler func(*AmfUe, NgapMsg)) {
	tx.AmfUe.TxLog.Infof("updated ngaphandler")
	tx.NgapHandler = handler
}

func (tx *EventChannel) UpdateNasHandler(handler func(*AmfUe, NasMsg)) {
	tx.NasHandler = handler
}

func (tx *EventChannel) UpdateSbiHandler(handler func(s1, s2 string, msg interface{}) (interface{}, string, interface{}, interface{})) {
	tx.AmfUe.TxLog.Infof("updated sbihandler")
	tx.SbiHandler = handler
}

func (tx *EventChannel) Start() {
	for {
		select {
		case msg := <-tx.Message:
			switch msg := msg.(type) {
			case NasMsg:
				tx.NasHandler(tx.AmfUe, msg)
			case NgapMsg:
				tx.NgapHandler(tx.AmfUe, msg)
			case SbiMsg:
				p1, p2, p3, p4 := tx.SbiHandler(msg.UeContextID, msg.ReqURI, msg.Msg)
				res := SbiResponseMsg{
					RespData:       p1,
					LocationHeader: p2,
					ProblemDetails: p3,
					TransferErr:    p4,
				}
				msg.Result <- res
			}
		case event := <-tx.Event:
			if event == "quit" {
				tx.AmfUe.TxLog.Infof("closed ue goroutine")
				return
			}
		}
	}
}

func (tx *EventChannel) SubmitMessage(msg interface{}) {
	tx.Message <- msg
}
