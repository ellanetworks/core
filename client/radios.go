package client

import "context"

type GetRadioOptions struct {
	Name string `json:"name"`
}

type PlmnID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnID PlmnID `json:"plmnID"`
	Tac    string `json:"tac"`
}

type Snssai struct {
	Sst int32  `json:"sst"`
	Sd  string `json:"sd"`
}

type SupportedTAI struct {
	Tai     Tai      `json:"tai"`
	SNssais []Snssai `json:"snssais"`
}

type Radio struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

func (c *Client) GetRadio(ctx context.Context, opts *GetRadioOptions) (*Radio, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/radios/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var radioResponse Radio

	err = resp.DecodeResult(&radioResponse)
	if err != nil {
		return nil, err
	}
	return &radioResponse, nil
}

func (c *Client) ListRadios(ctx context.Context) ([]*Radio, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/radios",
	})
	if err != nil {
		return nil, err
	}
	var radios []*Radio
	err = resp.DecodeResult(&radios)
	if err != nil {
		return nil, err
	}
	return radios, nil
}
