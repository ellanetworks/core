// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

func (c *UeConn) Serialize(f func()) {
	if c == nil {
		f()
		return
	}

	c.lane.Run(f)
}

func (c *UeConn) Await(job, resume func()) {
	c.lane.Await(job, resume)
}
