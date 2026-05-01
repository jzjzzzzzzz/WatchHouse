package transport

import (
	"encoding/json"
	"io"
)

func encodeResponse(output io.Writer, response BatchResponse) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(response)
}
