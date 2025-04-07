package client

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// GetMetrics retrieves the metrics from the server and returns a map
// where keys are metric names and values are their corresponding float values.
func (c *Client) GetMetrics() (map[string]float64, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   RawRequest,
		Method: "GET",
		Path:   "api/v1/metrics",
	})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Read the entire metrics output.
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parsePrometheusMetrics(string(data))
}

// parsePrometheusMetrics parses a Prometheus metrics text output and returns
// a map of metric names to their float values.
func parsePrometheusMetrics(data string) (map[string]float64, error) {
	metrics := make(map[string]float64)
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		// Trim spaces and skip empty lines or comments.
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Each metric line should contain the metric name and its value.
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metric %s: %w", parts[0], err)
		}
		metrics[parts[0]] = value
	}
	return metrics, nil
}
