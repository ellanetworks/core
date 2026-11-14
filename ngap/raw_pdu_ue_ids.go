// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

func (p *RawPDU) AMFUENGAPID() (AMFUENGAPID, bool, error) {
	var v AMFUENGAPID

	ok, err := p.Get(IDAMFUENGAPID, &v)

	return v, ok, err
}

func (p *RawPDU) SetAMFUENGAPID(v AMFUENGAPID) error { return p.Set(IDAMFUENGAPID, v) }

func (p *RawPDU) RANUENGAPID() (RANUENGAPID, bool, error) {
	var v RANUENGAPID

	ok, err := p.Get(IDRANUENGAPID, &v)

	return v, ok, err
}

func (p *RawPDU) SetRANUENGAPID(v RANUENGAPID) error { return p.Set(IDRANUENGAPID, v) }

func (p *RawPDU) SourceAMFUENGAPID() (AMFUENGAPID, bool, error) {
	var v AMFUENGAPID

	ok, err := p.Get(IDSourceAMFUENGAPID, &v)

	return v, ok, err
}

func (p *RawPDU) SetSourceAMFUENGAPID(v AMFUENGAPID) error { return p.Set(IDSourceAMFUENGAPID, v) }

func (p *RawPDU) NewAMFUENGAPID() (AMFUENGAPID, bool, error) {
	var v AMFUENGAPID

	ok, err := p.Get(IDNewAMFUENGAPID, &v)

	return v, ok, err
}

func (p *RawPDU) SetNewAMFUENGAPID(v AMFUENGAPID) error { return p.Set(IDNewAMFUENGAPID, v) }

func (p *RawPDU) UENGAPIDs() (UENGAPIDs, bool, error) {
	var v UENGAPIDs

	ok, err := p.Get(IDUENGAPIDs, &v)

	return v, ok, err
}

func (p *RawPDU) SetUENGAPIDs(v UENGAPIDs) error { return p.Set(IDUENGAPIDs, v) }
