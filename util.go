package gliter

import "strings"

func pad(val string, dimension int) string {
	if len(val) >= dimension {
		return val
	}
	return val + strings.Repeat(" ", dimension-len(val))
}

func sep(dimension int) string {
	return strings.Repeat("-", dimension)
}
