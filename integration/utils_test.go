package integration_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ellanetworks/core/client"
)

const (
	testPolicyName               = "default"
	numIMSIS                     = 5
	testStartIMSI                = "001010100000001"
	testUERANSIMIMSI             = "001019756139935"
	testSubscriberKey            = "0eefb0893e6f1c2855a3a244c6db1277"
	testSubscriberCustomOPc      = "98da19bbc55e2a5b53857d10557b1d26"
	testSubscriberSequenceNumber = "000000000022"
	numProfiles                  = 5
)

func computeIMSI(baseIMSI string, increment int) (string, error) {
	intBaseImsi, err := strconv.Atoi(baseIMSI)
	if err != nil {
		return "", fmt.Errorf("failed to convert base IMSI to int: %v", err)
	}
	newIMSI := intBaseImsi + increment
	return fmt.Sprintf("%015d", newIMSI), nil
}

func configureEllaCore(ctx context.Context, cl *client.Client, nat bool) error {
	initializeOpts := &client.InitializeOptions{
		Email:    "admin@ellanetworks.com",
		Password: "admin",
	}

	err := cl.Initialize(ctx, initializeOpts)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	err = cl.Refresh(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	createAPITokenOpts := &client.CreateAPITokenOptions{
		Name:   "integration-test-token",
		Expiry: "",
	}
	resp, err := cl.CreateMyAPIToken(ctx, createAPITokenOpts)
	if err != nil {
		return fmt.Errorf("failed to create API token: %v", err)
	}

	cl.SetToken(resp.Token)

	err = cl.UpdateNATInfo(ctx, &client.UpdateNATInfoOptions{
		Enabled: nat,
	})
	if err != nil {
		return fmt.Errorf("failed to configure NAT: %v", err)
	}

	for i := range numIMSIS {
		imsi, err := computeIMSI(testStartIMSI, i)
		if err != nil {
			return fmt.Errorf("failed to compute IMSI: %v", err)
		}

		createSubscriberOpts := &client.CreateSubscriberOptions{
			Imsi:           imsi,
			Key:            testSubscriberKey,
			SequenceNumber: testSubscriberSequenceNumber,
			PolicyName:     testPolicyName,
			OPc:            testSubscriberCustomOPc,
		}
		err = cl.CreateSubscriber(ctx, createSubscriberOpts)
		if err != nil {
			return fmt.Errorf("failed to create subscriber: %v", err)
		}
	}

	createUEransimSubscriberOpts := &client.CreateSubscriberOptions{
		Imsi:           testUERANSIMIMSI,
		Key:            testSubscriberKey,
		SequenceNumber: testSubscriberSequenceNumber,
		PolicyName:     testPolicyName,
		OPc:            testSubscriberCustomOPc,
	}
	err = cl.CreateSubscriber(ctx, createUEransimSubscriberOpts)
	if err != nil {
		return fmt.Errorf("failed to create UERANSIM subscriber: %v", err)
	}

	return nil
}

func waitForEllaCoreReady(ctx context.Context, cl *client.Client) error {
	timer := time.After(2 * time.Minute)

	for {
		select {
		case <-timer:
			return fmt.Errorf("timeout waiting for ella core to be ready")
		default:
			_, err := cl.GetStatus(ctx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil
		}
	}
}
