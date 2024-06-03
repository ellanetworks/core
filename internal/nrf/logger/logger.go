// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"time"

	formatter "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var (
	log            *logrus.Logger
	AppLog         *logrus.Entry
	InitLog        *logrus.Entry
	CfgLog         *logrus.Entry
	HandlerLog     *logrus.Entry
	ManagementLog  *logrus.Entry
	AccessTokenLog *logrus.Entry
	DiscoveryLog   *logrus.Entry
	GinLog         *logrus.Entry
	GrpcLog        *logrus.Entry
	UtilLog        *logrus.Entry
)

func init() {
	log = logrus.New()
	log.SetReportCaller(false)

	log.Formatter = &formatter.Formatter{
		TimestampFormat: time.RFC3339,
		TrimMessages:    true,
		NoFieldsSpace:   true,
		HideKeys:        true,
		FieldsOrder:     []string{"component", "category"},
	}

	AppLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "App"})
	InitLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "Init"})
	CfgLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "CFG"})
	HandlerLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "HDLR"})
	ManagementLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "MGMT"})
	AccessTokenLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "Token"})
	DiscoveryLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "DSCV"})
	GinLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "GIN"})
	GrpcLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "GRPC"})
	UtilLog = log.WithFields(logrus.Fields{"component": "NRF", "category": "Util"})
}

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetReportCaller(set bool) {
	log.SetReportCaller(set)
}
