package scale

import (
	"fmt"
	"strconv"
	"strings"
)

// A&D serial command constants (per HV-60KCWP-K specification).
const (
	CmdWeigh = "Q\r\n"
	CmdTare  = "T\r\n"
	CmdZero  = "R\r\n"
)

// ResponseHeader identifies the type of A&D weighing response.
type ResponseHeader string

const (
	HeaderST ResponseHeader = "ST" // Stable weight
	HeaderUS ResponseHeader = "US" // Unstable weight
	HeaderOL ResponseHeader = "OL" // Overload
	HeaderQT ResponseHeader = "QT" // Tare done
	HeaderTA ResponseHeader = "TA" // Tare done (alternate)
	HeaderZR ResponseHeader = "ZR" // Zero done (from R command)
)

// WeighResult holds a parsed weighing response.
type WeighResult struct {
	Header ResponseHeader
	Value  float64
	Unit   string
	Stable bool
}

// ParseWeighResponse parses an A&D weighing response line.
//
// A&D format: "HD,+NNNNN.NN UU\r\n" or "HD,  +NNNNN.NN UU\r\n"
// Examples:
//
//	"ST,+00012.34  kg\r\n" → stable, 12.34 kg
//	"US,+00012.30  kg\r\n" → unstable, 12.30 kg
//	"OL\r\n"               → overload
//	"ST,-00001.00  kg\r\n" → stable, -1.00 kg
func ParseWeighResponse(raw string) (WeighResult, error) {
	line := strings.TrimRight(raw, "\r\n")
	if line == "" {
		return WeighResult{}, fmt.Errorf("empty response")
	}

	// Overload has no comma
	if line == "OL" {
		return WeighResult{Header: HeaderOL}, nil
	}

	parts := strings.SplitN(line, ",", 2)
	if len(parts) != 2 {
		return WeighResult{}, fmt.Errorf("invalid response format: %q", raw)
	}

	header := ResponseHeader(strings.TrimSpace(parts[0]))
	body := strings.TrimSpace(parts[1])

	switch header {
	case HeaderST, HeaderUS:
		value, unit, err := parseValueUnit(body)
		if err != nil {
			return WeighResult{}, fmt.Errorf("parse value: %w (raw: %q)", err, raw)
		}
		return WeighResult{
			Header: header,
			Value:  value,
			Unit:   unit,
			Stable: header == HeaderST,
		}, nil
	default:
		return WeighResult{Header: header}, nil
	}
}

// parseValueUnit extracts the numeric value and unit from the body portion.
// Input examples: "+00012.34  kg", "-00001.00  kg"
func parseValueUnit(body string) (float64, string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0, "", fmt.Errorf("empty body")
	}

	lastAlpha := -1
	for i := len(body) - 1; i >= 0; i-- {
		if body[i] >= 'a' && body[i] <= 'z' || body[i] >= 'A' && body[i] <= 'Z' {
			lastAlpha = i
		} else if lastAlpha >= 0 {
			break
		}
	}

	var numStr, unit string
	if lastAlpha < 0 {
		numStr = body
	} else {
		numStr = strings.TrimSpace(body[:lastAlpha])
		unit = strings.TrimSpace(body[lastAlpha:])
	}

	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse float %q: %w", numStr, err)
	}

	return value, unit, nil
}

// ParseAckResponse checks if a response is a simple acknowledgement (QT, TA, ZR).
func ParseAckResponse(raw string) (ResponseHeader, bool) {
	line := strings.TrimRight(raw, "\r\n")
	line = strings.TrimSpace(line)

	switch ResponseHeader(line) {
	case HeaderQT:
		return HeaderQT, true
	case HeaderTA:
		return HeaderTA, true
	case HeaderZR:
		return HeaderZR, true
	}

	parts := strings.SplitN(line, ",", 2)
	header := ResponseHeader(strings.TrimSpace(parts[0]))
	switch header {
	case HeaderQT, HeaderTA, HeaderZR:
		return header, true
	}

	return "", false
}
