// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package supportbundle

type SanitizedData map[string]any

type ExportOptions struct {
	IncludeAuditLogs       bool
	IncludeSubscriberUsage bool
}

type SectionStatus struct {
	Status string `json:"status"`
	Count  int    `json:"captured_count"`
	Reason string `json:"reason,omitempty"`
}

func (s SectionStatus) WithCount(count int) SectionStatus {
	s.Count = count
	if s.Status == "" {
		s.Status = "complete"
	}

	if s.Status == "failed" && count > 0 {
		s.Status = "partial"
	}

	return s
}
