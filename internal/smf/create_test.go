// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"testing"

	"github.com/free5gc/nas/nasMessage"
)

func TestNegotiatePDUSessionType(t *testing.T) {
	s := &SMF{}
	ctx := context.Background()

	tests := []struct {
		name         string
		requested    uint8
		ipv4Pool     string
		ipv6Pool     string
		expectedType uint8
		expectError  bool
	}{
		// --- IPv4 requests ---
		{
			name:         "IPv4 requested with IPv4 pool",
			requested:    nasMessage.PDUSessionTypeIPv4,
			ipv4Pool:     "10.0.0.0/24",
			ipv6Pool:     "",
			expectedType: nasMessage.PDUSessionTypeIPv4,
			expectError:  false,
		},
		{
			name:        "IPv4 requested without IPv4 pool",
			requested:   nasMessage.PDUSessionTypeIPv4,
			ipv4Pool:    "",
			ipv6Pool:    "2001:db8::/32",
			expectError: true,
		},

		// --- IPv6 requests ---
		{
			name:         "IPv6 requested with IPv6 pool",
			requested:    nasMessage.PDUSessionTypeIPv6,
			ipv4Pool:     "",
			ipv6Pool:     "2001:db8::/32",
			expectedType: nasMessage.PDUSessionTypeIPv6,
			expectError:  false,
		},
		{
			name:        "IPv6 requested without IPv6 pool",
			requested:   nasMessage.PDUSessionTypeIPv6,
			ipv4Pool:    "10.0.0.0/24",
			ipv6Pool:    "",
			expectError: true,
		},

		// --- IPv4v6 (dual-stack) requests ---
		{
			name:         "IPv4v6 requested with both pools",
			requested:    nasMessage.PDUSessionTypeIPv4IPv6,
			ipv4Pool:     "10.0.0.0/24",
			ipv6Pool:     "2001:db8::/32",
			expectedType: nasMessage.PDUSessionTypeIPv4IPv6,
			expectError:  false,
		},
		{
			name:         "IPv4v6 requested with IPv4-only pool -> downgraded to IPv4",
			requested:    nasMessage.PDUSessionTypeIPv4IPv6,
			ipv4Pool:     "10.0.0.0/24",
			ipv6Pool:     "",
			expectedType: nasMessage.PDUSessionTypeIPv4,
			expectError:  false,
		},
		{
			name:         "IPv4v6 requested with IPv6-only pool -> downgraded to IPv6",
			requested:    nasMessage.PDUSessionTypeIPv4IPv6,
			ipv4Pool:     "",
			ipv6Pool:     "2001:db8::/32",
			expectedType: nasMessage.PDUSessionTypeIPv6,
			expectError:  false,
		},
		{
			name:        "IPv4v6 requested with no pools",
			requested:   nasMessage.PDUSessionTypeIPv4IPv6,
			ipv4Pool:    "",
			ipv6Pool:    "",
			expectError: true,
		},

		// --- Unknown type ---
		{
			name:        "unknown PDU session type",
			requested:   0x00,
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := &Policy{
				IPv4Pool: tc.ipv4Pool,
				IPv6Pool: tc.ipv6Pool,
			}

			result, err := s.negotiatePDUSessionType(ctx, tc.requested, policy)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for requested=%d, ipv4Pool=%q, ipv6Pool=%q", tc.requested, tc.ipv4Pool, tc.ipv6Pool)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tc.expectedType {
				t.Errorf("expected type %d, got %d", tc.expectedType, result)
			}
		})
	}
}

func TestNegotiatePDUSessionType_CauseForDowngrade(t *testing.T) {
	s := &SMF{}
	ctx := context.Background()

	// IPv4v6 requested, IPv4-only pool -> should produce IPv4-only cause
	{
		requested := nasMessage.PDUSessionTypeIPv4IPv6
		policy := &Policy{IPv4Pool: "10.0.0.0/24", IPv6Pool: ""}

		negotiated, err := s.negotiatePDUSessionType(ctx, requested, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if negotiated != nasMessage.PDUSessionTypeIPv4 {
			t.Fatalf("expected IPv4, got %d", negotiated)
		}

		var cause uint8

		if requested == nasMessage.PDUSessionTypeIPv4IPv6 && negotiated != nasMessage.PDUSessionTypeIPv4IPv6 {
			if negotiated == nasMessage.PDUSessionTypeIPv4 {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
			} else {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
			}
		}

		if cause != nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed {
			t.Errorf("expected cause %d (IPv4-only), got %d", nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed, cause)
		}
	}

	// IPv4v6 requested, IPv6-only pool -> should produce IPv6-only cause
	{
		requested := nasMessage.PDUSessionTypeIPv4IPv6
		policy := &Policy{IPv4Pool: "", IPv6Pool: "2001:db8::/32"}

		negotiated, err := s.negotiatePDUSessionType(ctx, requested, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if negotiated != nasMessage.PDUSessionTypeIPv6 {
			t.Fatalf("expected IPv6, got %d", negotiated)
		}

		var cause uint8

		if requested == nasMessage.PDUSessionTypeIPv4IPv6 && negotiated != nasMessage.PDUSessionTypeIPv4IPv6 {
			if negotiated == nasMessage.PDUSessionTypeIPv4 {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
			} else {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
			}
		}

		if cause != nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed {
			t.Errorf("expected cause %d (IPv6-only), got %d", nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed, cause)
		}
	}

	// IPv4v6 requested with both pools -> no cause (full dual-stack accepted)
	{
		requested := nasMessage.PDUSessionTypeIPv4IPv6
		policy := &Policy{IPv4Pool: "10.0.0.0/24", IPv6Pool: "2001:db8::/32"}

		negotiated, err := s.negotiatePDUSessionType(ctx, requested, policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if negotiated != nasMessage.PDUSessionTypeIPv4IPv6 {
			t.Fatalf("expected IPv4v6, got %d", negotiated)
		}

		var cause uint8

		if requested == nasMessage.PDUSessionTypeIPv4IPv6 && negotiated != nasMessage.PDUSessionTypeIPv4IPv6 {
			if negotiated == nasMessage.PDUSessionTypeIPv4 {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed
			} else {
				cause = nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed
			}
		}

		if cause != 0 {
			t.Errorf("expected no cause, got %d", cause)
		}
	}

	// IPv4 requested, IPv6-only pool -> reject (not a downgrade, a rejection)
	{
		requested := nasMessage.PDUSessionTypeIPv4
		policy := &Policy{IPv4Pool: "", IPv6Pool: "2001:db8::/32"}

		_, err := s.negotiatePDUSessionType(ctx, requested, policy)
		if err == nil {
			t.Fatal("expected error for mismatched single-stack request")
		}
	}
}
