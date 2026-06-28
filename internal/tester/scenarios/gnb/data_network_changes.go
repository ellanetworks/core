// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/ellanetworks/core/internal/tester/testutil/procedure"
	"github.com/ellanetworks/core/internal/tester/testutil/validate"
	"github.com/free5gc/nas"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func init() {
	scenarios.Register(scenarios.Scenario{
		Name: "gnb/data-network-dns-change",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &dataNetworkChangeParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address (e.g. http://10.3.0.2:5002)")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkDNSChange(ctx, env, params.(*dataNetworkChangeParams))
		},
		Fixture: fixtureDataNetworkDNSChange,
	})

	scenarios.Register(scenarios.Scenario{
		Name: "gnb/data-network-mtu-change",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &dataNetworkChangeParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkMTUChange(ctx, env, params.(*dataNetworkChangeParams))
		},
		Fixture: fixtureDataNetworkMTUChange,
	})

	scenarios.Register(scenarios.Scenario{
		Name: "gnb/data-network-pool-change",
		BindFlags: func(fs *pflag.FlagSet) any {
			p := &dataNetworkChangeParams{}
			fs.StringVar(&p.EllaAPIAddress, "ella-api-address", "", "Ella Core API address")
			fs.StringVar(&p.EllaAPIToken, "ella-api-token", "", "Ella Core API token")

			return p
		},
		Run: func(ctx context.Context, env scenarios.Env, params any) error {
			return runDataNetworkPoolChange(ctx, env, params.(*dataNetworkChangeParams))
		},
		Fixture: fixtureDataNetworkPoolChange,
	})
}

type dataNetworkChangeParams struct {
	EllaAPIAddress string
	EllaAPIToken   string
}

func fixtureDataNetworkDNSChange(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func fixtureDataNetworkMTUChange(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

func fixtureDataNetworkPoolChange(env scenarios.Env) scenarios.FixtureSpec {
	return scenarios.FixtureSpec{
		Subscribers: []scenarios.SubscriberSpec{scenarios.DefaultSubscriber()},
	}
}

// runDataNetworkDNSChange verifies that changing the DNS server in a data
// network triggers a PDU Session Modification Command carrying the new DNS
// in Extended PCO (TS 24.501 §6.3.2), without releasing the session.
func runDataNetworkDNSChange(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info("PDU session established, proceeding to change DNS in data network",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	newDNS := "1.1.1.1"

	err = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
		Name:     scenarios.DefaultDNN,
		DNS:      newDNS,
		IPv4Pool: scenarios.DefaultUEIPv4Pool,
		IPv6Pool: scenarios.DefaultUEIPv6Pool,
		Mtu:      scenarios.DefaultMTU,
	})
	if err != nil {
		return fmt.Errorf("failed to update data network DNS: %v", err)
	}

	defer func() {
		_ = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
			Name:     scenarios.DefaultDNN,
			DNS:      scenarios.DefaultDNS,
			IPv4Pool: scenarios.DefaultUEIPv4Pool,
			IPv6Pool: scenarios.DefaultUEIPv6Pool,
			Mtu:      scenarios.DefaultMTU,
		})
	}()

	logger.Logger.Info("Data network DNS updated, waiting for session modification signalling")

	modCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionModificationCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Modification Command: %v", err)
	}

	logger.Logger.Info("UE received PDU Session Modification Command")

	err = validate.PCODNS(modCmd, &validate.ExpectedPCODNS{
		IPv4: newDNS,
	})
	if err != nil {
		return fmt.Errorf("PCO DNS validation failed: %v", err)
	}

	logger.Logger.Info("DNS change validated successfully", zap.String("New DNS", newDNS))

	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("DNS change scenario completed successfully")

	return nil
}

// runDataNetworkMTUChange verifies that changing the MTU in a data network
// triggers a PDU Session Release with cause #39 "reactivation requested"
// (TS 23.501 §5.6.10.4 NOTE 3).
func runDataNetworkMTUChange(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info("PDU session established, proceeding to change MTU in data network",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	newMTU := int32(1400)

	err = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
		Name:     scenarios.DefaultDNN,
		DNS:      scenarios.DefaultDNS,
		IPv4Pool: scenarios.DefaultUEIPv4Pool,
		IPv6Pool: scenarios.DefaultUEIPv6Pool,
		Mtu:      newMTU,
	})
	if err != nil {
		return fmt.Errorf("failed to update data network MTU: %v", err)
	}

	defer func() {
		_ = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
			Name:     scenarios.DefaultDNN,
			DNS:      scenarios.DefaultDNS,
			IPv4Pool: scenarios.DefaultUEIPv4Pool,
			IPv6Pool: scenarios.DefaultUEIPv6Pool,
			Mtu:      scenarios.DefaultMTU,
		})
	}()

	logger.Logger.Info("Data network MTU updated, waiting for session release")

	releaseCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionReleaseCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Release Command: %v", err)
	}

	logger.Logger.Info("UE received PDU Session Release Command")

	if releaseCmd.PDUSessionReleaseCommand == nil {
		return fmt.Errorf("PDUSessionReleaseCommand is nil")
	}

	cause := releaseCmd.PDUSessionReleaseCommand.GetCauseValue()
	if cause != 39 {
		return fmt.Errorf("expected cause #39 (reactivation requested), got %d", cause)
	}

	logger.Logger.Info("MTU change triggered session release with cause #39 as expected")

	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("MTU change scenario completed successfully")

	return nil
}

// runDataNetworkPoolChange verifies that changing the IP pool in a data network
// triggers a PDU Session Release with cause #39 "reactivation requested".
func runDataNetworkPoolChange(ctx context.Context, env scenarios.Env, p *dataNetworkChangeParams) error {
	if p.EllaAPIAddress == "" {
		return fmt.Errorf("--ella-api-address is required")
	}

	if p.EllaAPIToken == "" {
		return fmt.Errorf("--ella-api-token is required")
	}

	cl, err := client.New(&client.Config{BaseURL: p.EllaAPIAddress})
	if err != nil {
		return fmt.Errorf("failed to create Ella client: %v", err)
	}

	cl.SetToken(p.EllaAPIToken)

	gNodeB, err := startGNB(env)
	if err != nil {
		return err
	}

	defer gNodeB.Close()

	ranUENGAPID := int64(scenarios.DefaultRANUENGAPID)

	sub := subscriber{
		IMSI:           scenarios.DefaultIMSI,
		Key:            scenarios.DefaultKey,
		OPc:            scenarios.DefaultOPC,
		SequenceNumber: scenarios.DefaultSequenceNumber,
	}

	newUE, err := newDefaultUE(gNodeB, sub.IMSI[5:], sub.Key, sub.OPc, sub.SequenceNumber, env.PDUSessionType())
	if err != nil {
		return fmt.Errorf("could not create UE: %v", err)
	}

	gNodeB.AddUE(ranUENGAPID, newUE)

	_, err = procedure.InitialRegistration(&procedure.InitialRegistrationOpts{
		RANUENGAPID:  ranUENGAPID,
		PDUSessionID: scenarios.DefaultPDUSessionID,
		UE:           newUE,
	})
	if err != nil {
		return fmt.Errorf("initial registration failed: %v", err)
	}

	logger.Logger.Info("PDU session established, proceeding to change IP pool in data network",
		zap.String("IMSI", sub.IMSI),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	newIPv4Pool := "10.46.0.0/16"

	err = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
		Name:     scenarios.DefaultDNN,
		DNS:      scenarios.DefaultDNS,
		IPv4Pool: newIPv4Pool,
		IPv6Pool: scenarios.DefaultUEIPv6Pool,
		Mtu:      scenarios.DefaultMTU,
	})
	if err != nil {
		return fmt.Errorf("failed to update data network IP pool: %v", err)
	}

	defer func() {
		_ = cl.UpdateDataNetwork(ctx, &client.UpdateDataNetworkOptions{
			Name:     scenarios.DefaultDNN,
			DNS:      scenarios.DefaultDNS,
			IPv4Pool: scenarios.DefaultUEIPv4Pool,
			IPv6Pool: scenarios.DefaultUEIPv6Pool,
			Mtu:      scenarios.DefaultMTU,
		})
	}()

	logger.Logger.Info("Data network IP pool updated, waiting for session release")

	releaseCmd, err := newUE.WaitForNASGSMMessage(nas.MsgTypePDUSessionReleaseCommand, 15*time.Second)
	if err != nil {
		return fmt.Errorf("UE did not receive PDU Session Release Command: %v", err)
	}

	logger.Logger.Info("UE received PDU Session Release Command")

	if releaseCmd.PDUSessionReleaseCommand == nil {
		return fmt.Errorf("PDUSessionReleaseCommand is nil")
	}

	cause := releaseCmd.PDUSessionReleaseCommand.GetCauseValue()
	if cause != 39 {
		return fmt.Errorf("expected cause #39 (reactivation requested), got %d", cause)
	}

	logger.Logger.Info("IP pool change triggered session release with cause #39 as expected")

	pduSessionIDs := [16]bool{}
	pduSessionIDs[scenarios.DefaultPDUSessionID] = true

	err = procedure.UEContextRelease(&procedure.UEContextReleaseOpts{
		AMFUENGAPID:   gNodeB.GetAMFUENGAPID(ranUENGAPID),
		RANUENGAPID:   ranUENGAPID,
		GnodeB:        gNodeB,
		UE:            newUE,
		PDUSessionIDs: pduSessionIDs,
	})
	if err != nil {
		return fmt.Errorf("UE context release failed: %v", err)
	}

	logger.Logger.Info("IP pool change scenario completed successfully")

	return nil
}
