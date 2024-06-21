package trace

import (
	"strings"
)

const fixedStringSuffix = "...stringLengthTooLong"
const defaultMaxStringLength = 32766

var maxStringLength = defaultMaxStringLength

// SetMaxStringLength sets the maximum length of a string attribute value.
func SetMaxStringLength(limit int) {
	if limit > defaultMaxStringLength {
		return
	}
	maxStringLength = limit
}

// isStringTooLong
func isStringTooLong(s string) bool {
	return len(s) > maxStringLength
}

// fixStringTooLong
// Document contains at least one immense term in field=\"logs.fields.value\"
// (whose UTF8 encoding is longer than the max length 32766)
func fixStringTooLong(s string) (result string) {
	if isStringTooLong(s) {
		return strings.ToValidUTF8(s[:maxStringLength-len(fixedStringSuffix)]+fixedStringSuffix, "")
	}
	return s
}
