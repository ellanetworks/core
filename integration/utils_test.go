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
	Subscribers []SubscriberConfig
	Networking  NetworkingConfig
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

func createSubs(ctx context.Context, cl *client.Client, subs []SubscriberConfig) error {
	for _, sub := range subs {
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
