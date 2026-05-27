package firewall

import (
	"bytes"
	"encoding/json"
)

func RenderInventoryNDJSON(inventory Inventory) ([]byte, error) {
	records := BuildInventoryRecords(inventory)
	var out bytes.Buffer
	for i, record := range records {
		if i > 0 {
			out.WriteByte('\n')
		}
		body, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		out.Write(body)
	}
	return out.Bytes(), nil
}
