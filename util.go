package gliter

import "strings"

const MAX_PAD int = 20

func pad(val string, dimension int) string {
	if len(val) == dimension {
		return val
	} else if len(val) > dimension {
		return string([]rune(val)[0:dimension-2]) + ".."
	}
	return val + strings.Repeat(" ", dimension-len(val))
}

func sep(dimension int) string {
	return strings.Repeat("-", dimension)
}
