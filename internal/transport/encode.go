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

func encodeEventPage(output io.Writer, page EventPage) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(page)
}

func encodeFindingPage(output io.Writer, page FindingPage) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(page)
}
