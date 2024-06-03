// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package logger

import (
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log         *logrus.Logger
	AppLog      *logrus.Entry
	InitLog     *logrus.Entry
	CfgLog      *logrus.Entry
	ContextLog  *logrus.Entry
	DataRepoLog *logrus.Entry
	NgapLog     *logrus.Entry
	HandlerLog  *logrus.Entry
	HttpLog     *logrus.Entry
	GmmLog      *logrus.Entry
	MtLog       *logrus.Entry
	ProducerLog *logrus.Entry
	LocationLog *logrus.Entry
	CommLog     *logrus.Entry
	CallbackLog *logrus.Entry
	UtilLog     *logrus.Entry
	NasLog      *logrus.Entry
	ConsumerLog *logrus.Entry
	EeLog       *logrus.Entry
	GinLog      *logrus.Entry
	GrpcLog     *logrus.Entry
	KafkaLog    *logrus.Entry
)

const (
	FieldRanAddr     string = "ran_addr"
	FieldRanId       string = "ran_id"
	FieldAmfUeNgapID string = "amf_ue_ngap_id"
	FieldSupi        string = "supi"
	FieldSuci        string = "suci"
)

func init() {
	log = logrus.New()
	log.SetReportCaller(false)

	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category", FieldRanAddr, FieldRanId, FieldAmfUeNgapID, FieldSupi, FieldSuci},
	}

	AppLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Init"})
	CfgLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "CFG"})
	ContextLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Context"})
	DataRepoLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "DRepo"})
	NgapLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "NGAP"})
	HandlerLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Handler"})
	HttpLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "HTTP"})
	GmmLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "GMM"})
	MtLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "MT"})
	ProducerLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Producer"})
	LocationLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "LocInfo"})
	CommLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Comm"})
	CallbackLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Callback"})
	UtilLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Util"})
	NasLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "NAS"})
	ConsumerLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Consumer"})
	EeLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "EventExposure"})
	GinLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "GIN"})
	GrpcLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "GRPC"})
	KafkaLog = log.WithFields(logrus.Fields{"component": "AMF", "category": "Kafka"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
