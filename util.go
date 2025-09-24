package main

import "strings"

func escapeString(s string) string {
	return strings.ReplaceAll(s, "/", "-")
}
