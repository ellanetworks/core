// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

func (p *RawPDU) MMEUES1APID() (MMEUES1APID, bool, error) {
	var v MMEUES1APID

	ok, err := p.Get(IDMMEUES1APID, &v)

	return v, ok, err
}

func (p *RawPDU) SetMMEUES1APID(v MMEUES1APID) error { return p.Set(IDMMEUES1APID, v) }

func (p *RawPDU) ENBUES1APID() (ENBUES1APID, bool, error) {
	var v ENBUES1APID

	ok, err := p.Get(IDENBUES1APID, &v)

	return v, ok, err
}

func (p *RawPDU) SetENBUES1APID(v ENBUES1APID) error { return p.Set(IDENBUES1APID, v) }

func (p *RawPDU) SourceMMEUES1APID() (MMEUES1APID, bool, error) {
	var v MMEUES1APID

	ok, err := p.Get(IDSourceMMEUES1APID, &v)

	return v, ok, err
}

func (p *RawPDU) SetSourceMMEUES1APID(v MMEUES1APID) error { return p.Set(IDSourceMMEUES1APID, v) }

func (p *RawPDU) UES1APIDs() (UES1APIDs, bool, error) {
	var v UES1APIDs

	ok, err := p.Get(IDUES1APIDs, &v)

	return v, ok, err
}

func (p *RawPDU) SetUES1APIDs(v UES1APIDs) error { return p.Set(IDUES1APIDs, v) }
