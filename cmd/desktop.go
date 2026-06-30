package cmd

import "strings"

type desktopApp struct {
	Name string
	Exec string
}

func stripExecCodes(exec string) string {
	var b strings.Builder
	for i := 0; i < len(exec); i++ {
		if exec[i] == '%' && i+1 < len(exec) {
			i++
			continue
		}
		b.WriteByte(exec[i])
	}
	return strings.TrimSpace(b.String())
}
