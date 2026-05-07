package main

import (
	"fmt"
	"html/template"
	"strings"
)

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"split": strings.Split,
	"join": func(base, part string) string {
		if base == "" {
			return part
		}
		return base + "/" + part
	},
	"parent": func(path string) string {
		if !strings.Contains(path, "/") {
			return ""
		}
		parts := strings.Split(path, "/")
		return strings.Join(parts[:len(parts)-1], "/")
	},
	"add": func(a, b int) int {
		return a + b
	},
	"max": func(a, b int) int {
		if a > b {
			return a
		}
		return b
	},
	"truncate": func(s string, l int) string {
		s = strings.Split(s, "\n")[0]
		s = strings.TrimSpace(s)
		r := []rune(s)
		if len(r) <= l {
			return s
		}
		return string(r[:l]) + "..."
	},
	"float64": func(i int) float64 {
		return float64(i)
	},
	"multiply": func(a, b float64) float64 {
		return a * b
	},
	"divide": func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"humanize": func(size int64) string {
		const unit = 1024
		if size < unit {
			return fmt.Sprintf("%d B", size)
		}
		div, exp := int64(unit), 0
		for n := size / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
	},
}).ParseFS(viewsFS, "views/*.html"))
