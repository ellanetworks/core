// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package redact

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

func ApplyYAML(b []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, errors.New("cannot parse YAML configuration safely")
	}

	if err := maskYAMLSecrets(&doc, make(map[*yaml.Node]bool)); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(&doc); err != nil {
		return nil, errors.New("cannot encode sanitized YAML configuration")
	}

	if err := enc.Close(); err != nil {
		return nil, errors.New("cannot finalize sanitized YAML configuration")
	}

	return buf.Bytes(), nil
}

func maskYAMLSecrets(n *yaml.Node, visited map[*yaml.Node]bool) error {
	if n == nil || visited[n] {
		return nil
	}

	visited[n] = true
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			var key string
			if err := n.Content[i].Decode(&key); err != nil {
				return errors.New("cannot safely decode YAML configuration key")
			}

			if key != "join-token" {
				continue
			}

			v := n.Content[i+1]

			target := v
			for depth := 0; target != nil && target.Kind == yaml.AliasNode && depth < 32; depth++ {
				target = target.Alias
			}

			if target == nil || target.Kind != yaml.ScalarNode {
				return errors.New("cannot safely mask YAML join token")
			}

			target.Value, target.Tag, target.Style = "*", "!!str", yaml.DoubleQuotedStyle
			if v.Kind == yaml.AliasNode {
				n.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "*", Style: yaml.DoubleQuotedStyle}
			}
		}
	}

	if err := maskYAMLSecrets(n.Alias, visited); err != nil {
		return err
	}

	for _, child := range n.Content {
		if err := maskYAMLSecrets(child, visited); err != nil {
			return err
		}
	}

	return nil
}
