// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package pcf

// var tracer = otel.Tracer("ella-core/pcf")

// var pcfCtx *PCFContext

// type PCFContext struct {
// 	DBInstance *db.Database
// }

// func Start(dbInstance *db.Database) error {
// 	pcfCtx = &PCFContext{
// 		DBInstance: dbInstance,
// 	}
// 	return nil
// }

// func CreateSMPolicy(ctx context.Context, request models.SmPolicyContextData) (*models.SmPolicyDecision, error) {
// 	ctx, span := tracer.Start(ctx, "PCF Create SMPolicy")
// 	span.SetAttributes(
// 		attribute.String("ue.supi", request.Supi),
// 	)

// 	if request.Supi == "" || request.SliceInfo == nil || request.Dnn == "" {
// 		return nil, fmt.Errorf("Errorneous/Missing Mandotory IE")
// 	}

// 	subscriberPolicy, err := GetSubscriberPolicy(ctx, request.Supi, request.SliceInfo.Sst, request.SliceInfo.Sd, request.Dnn)
// 	if err != nil {
// 		return nil, fmt.Errorf("can't find subscriber policy for subscriber %s: %s", request.Supi, err)
// 	}

// 	return subscriberPolicy, nil
// }

// func GetSubscriberPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*models.SmPolicyDecision, error) {
// 	subscriber, err := pcfCtx.DBInstance.GetSubscriber(ctx, imsi)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get subscriber %s: %w", imsi, err)
// 	}

// 	policy, err := pcfCtx.DBInstance.GetPolicyByID(ctx, subscriber.PolicyID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get policy %d: %w", subscriber.PolicyID, err)
// 	}

// 	dataNetwork, err := pcfCtx.DBInstance.GetDataNetworkByID(ctx, policy.DataNetworkID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get data network %d: %w", policy.DataNetworkID, err)
// 	}

// 	if dataNetwork.Name != dnn {
// 		return nil, fmt.Errorf("subscriber %s has no policy for dnn %s", imsi, dnn)
// 	}

// 	operator, err := pcfCtx.DBInstance.GetOperator(ctx)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get operator: %w", err)
// 	}

// 	if operator.Sst != sst || operator.GetHexSd() != sd {
// 		return nil, fmt.Errorf("subscriber %s has no policy for slice sst: %d sd: %s", imsi, sst, sd)
// 	}

// 	subscriberPolicy := &models.SmPolicyDecision{
// 		SessRule: &models.SessionRule{
// 			AuthDefQos: &models.AuthorizedDefaultQos{
// 				Var5qi: policy.Var5qi,
// 				Arp:    &models.Arp{PriorityLevel: policy.Arp},
// 			},
// 			AuthSessAmbr: &models.Ambr{
// 				Uplink:   policy.BitrateUplink,
// 				Downlink: policy.BitrateDownlink,
// 			},
// 		},
// 		QosDecs: &models.QosData{
// 			Var5qi:               policy.Var5qi,
// 			Arp:                  &models.Arp{PriorityLevel: policy.Arp},
// 			DefQosFlowIndication: true,
// 		},
// 	}

// 	return subscriberPolicy, nil
// }
