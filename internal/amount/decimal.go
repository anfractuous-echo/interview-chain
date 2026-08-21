package amount

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

const (
	DecimalPlaces        = 6
	SmallestUnitsPerUnit = int64(1_000_000)
)

var ErrInvalidDecimal = errors.New("invalid decimal amount")

func ParseSmallestUnits(value string) (int64, error) {
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return 0, ErrInvalidDecimal
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidDecimal
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > math.MaxInt64/SmallestUnitsPerUnit {
		return 0, ErrInvalidDecimal
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > DecimalPlaces {
		return 0, ErrInvalidDecimal
	}
	for len(fraction) < DecimalPlaces {
		fraction += "0"
	}
	fractionUnits := int64(0)
	if fraction != "" {
		fractionUnits, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, ErrInvalidDecimal
		}
	}
	units := whole * SmallestUnitsPerUnit
	if fractionUnits > math.MaxInt64-units {
		return 0, ErrInvalidDecimal
	}
	units += fractionUnits
	if units <= 0 {
		return 0, ErrInvalidDecimal
	}
	return units, nil
}
