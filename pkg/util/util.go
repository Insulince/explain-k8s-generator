package util

import (
	"strings"
)

func RemoveBlankLines(data string) string {
	var lines []string
	for _, line := range strings.Split(data, "\n") {
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
