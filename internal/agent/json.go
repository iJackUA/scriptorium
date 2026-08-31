package agent

import "encoding/json"

// unmarshalResponse is kept separate from the public Response shape so the
// Go API can use the concise Cost name while matching Claude's wire key.
func unmarshalResponse(body []byte, response *Response) error {
	var wire struct {
		Result       string  `json:"result"`
		IsError      bool    `json:"is_error"`
		TotalCostUSD float64 `json:"total_cost_usd"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return err
	}
	response.Result = wire.Result
	response.IsError = wire.IsError
	response.Cost = wire.TotalCostUSD
	return nil
}
