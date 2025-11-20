package integration_test

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

type SubscriberConfig struct {
	Imsi           string
	Key            string
	SequenceNumber string
	OPc            string
	PolicyName     string
}

type PolicyConfig struct {
	Name            string
	BitrateUplink   string
	BitrateDownlink string
	Var5qi          int32
	Arp             int32
	DataNetworkName string
}

type DataNetworkConfig struct {
	Name   string
	IPPool string
	DNS    string
	Mtu    int32
}

type OperatorID struct {
	MCC string
	MNC string
}

type OperatorSlice struct {
	SST int32
	SD  string
}

type OperatorTracking struct {
	SupportedTACs []string
}

type OperatorConfig struct {
	ID       OperatorID
	Slice    OperatorSlice
	Tracking OperatorTracking
}

type RouteConfig struct {
	Destination string
	Gateway     string
	Interface   string
	Metric      int
}

type NetworkingConfig struct {
	NAT    bool
	Routes []RouteConfig
}

type EllaCoreConfig struct {
	Operator     OperatorConfig
	DataNetworks []DataNetworkConfig
	Policies     []PolicyConfig
	Subscribers  []SubscriberConfig
	Networking   NetworkingConfig
}

type logWriter struct{ t *testing.T }

func (w logWriter) Write(p []byte) (int, error) {
	// log line-by-line to keep test output readable
	s := string(p)
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		w.t.Log(sc.Text())
	}
	return len(p), nil
}

func configureEllaCore(ctx context.Context, cl *client.Client, c EllaCoreConfig) error {
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
		Enabled: c.Networking.NAT,
	})
	if err != nil {
		return fmt.Errorf("failed to configure NAT: %v", err)
	}

	err = createRoutes(ctx, cl, c.Networking.Routes)
	if err != nil {
		return fmt.Errorf("failed to create routes: %v", err)
	}

	err = updateOperator(ctx, cl, c.Operator)
	if err != nil {
		return fmt.Errorf("failed to update operator config: %v", err)
	}

	err = createDataNetworks(ctx, cl, c.DataNetworks)
	if err != nil {
		return fmt.Errorf("failed to create data networks: %v", err)
	}

	err = createPolicies(ctx, cl, c.Policies)
	if err != nil {
		return fmt.Errorf("could not create policies: %v", err)
	}

	err = createSubs(ctx, cl, c.Subscribers)
	if err != nil {
		return fmt.Errorf("could not create subscribers: %v", err)
	}

	return nil
}

func createRoutes(ctx context.Context, cl *client.Client, routes []RouteConfig) error {
	for _, r := range routes {
		createRouteOpts := &client.CreateRouteOptions{
			Destination: r.Destination,
			Gateway:     r.Gateway,
			Interface:   r.Interface,
			Metric:      r.Metric,
		}
		err := cl.CreateRoute(ctx, createRouteOpts)
		if err != nil {
			return fmt.Errorf("failed to create n6 route: %v", err)
		}
	}

	return nil
}

func updateOperator(ctx context.Context, cl *client.Client, c OperatorConfig) error {
	opConfig, err := cl.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("failed to get operator: %v", err)
	}

	if opConfig.ID.Mcc != c.ID.MCC || opConfig.ID.Mnc != c.ID.MNC {
		err := cl.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{
			Mcc: c.ID.MCC,
			Mnc: c.ID.MNC,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator ID: %v", err)
		}
	}

	if opConfig.Slice.Sst != int(c.Slice.SST) || opConfig.Slice.Sd != c.Slice.SD {
		err := cl.UpdateOperatorSlice(ctx, &client.UpdateOperatorSliceOptions{
			Sst: int(c.Slice.SST),
			Sd:  c.Slice.SD,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator slice: %v", err)
		}
	}

	currentTACsMap := make(map[string]bool)
	for _, tac := range opConfig.Tracking.SupportedTacs {
		currentTACsMap[tac] = true
	}

	needUpdate := false

	for _, tac := range c.Tracking.SupportedTACs {
		if !currentTACsMap[tac] {
			needUpdate = true
			break
		}
	}

	if needUpdate {
		err := cl.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{
			SupportedTacs: c.Tracking.SupportedTACs,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator tracking: %v", err)
		}
	}

	return nil
}

func createDataNetworks(ctx context.Context, cl *client.Client, dn []DataNetworkConfig) error {
	for _, dnn := range dn {
		existingDNN, _ := cl.GetDataNetwork(ctx, &client.GetDataNetworkOptions{
			Name: dnn.Name,
		})

		if existingDNN == nil {
			err := cl.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
				Name:   dnn.Name,
				IPPool: dnn.IPPool,
				DNS:    dnn.DNS,
				Mtu:    dnn.Mtu,
			})
			if err != nil {
				return fmt.Errorf("failed to create data network: %v", err)
			}
		}
	}

	return nil
}

func createPolicies(ctx context.Context, cl *client.Client, policies []PolicyConfig) error {
	for _, policy := range policies {
		existingPolicy, _ := cl.GetPolicy(ctx, &client.GetPolicyOptions{
			Name: policy.Name,
		})

		if existingPolicy == nil {
			err := cl.CreatePolicy(ctx, &client.CreatePolicyOptions{
				Name:            policy.Name,
				BitrateUplink:   policy.BitrateUplink,
				BitrateDownlink: policy.BitrateDownlink,
				Var5qi:          policy.Var5qi,
				Arp:             policy.Arp,
				DataNetworkName: policy.DataNetworkName,
			})
			if err != nil {
				return fmt.Errorf("failed to create policy: %v", err)
			}
		}
	}

	return nil
}

func createSubs(ctx context.Context, cl *client.Client, subs []SubscriberConfig) error {
	for _, sub := range subs {
		existingSub, _ := cl.GetSubscriber(ctx, &client.GetSubscriberOptions{
			ID: sub.Imsi,
		})

		if existingSub == nil {
			err := cl.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
				Imsi:           sub.Imsi,
				Key:            sub.Key,
				SequenceNumber: sub.SequenceNumber,
				PolicyName:     sub.PolicyName,
				OPc:            sub.OPc,
			})
			if err != nil {
				return fmt.Errorf("failed to create subscriber: %v", err)
			}
		}
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

func waitForPatternInContainer(
	parent context.Context,
	dc *DockerClient,
	container, logPath, regex string,
	timeout, interval time.Duration,
) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	re, err := regexp.Compile(regex)
	if err != nil {
		return fmt.Errorf("bad regex: %w", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			tail, _ := dc.Exec(context.Background(), container,
				[]string{"sh", "-lc", fmt.Sprintf("tail -n 50 %s || true", logPath)},
				false, 3*time.Second, nil)
			return fmt.Errorf("timeout waiting for %q. last lines:\n%s", regex, tail)
		case <-ticker.C:
			out, err := dc.Exec(ctx, container,
				[]string{"sh", "-lc", fmt.Sprintf(`test -f %q && cat %q || true`, logPath, logPath)},
				false, 3*time.Second, nil)
			if err != nil {
				continue // transient; try again until timeout
			}
			if re.MatchString(out) {
				return nil
			}
		}
	}
}
