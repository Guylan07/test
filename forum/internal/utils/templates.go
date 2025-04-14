package utils

import (
	"html/template"
	"path/filepath"
)

func ParseTemplate(baseTemplate string, templates ...string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"subtract": func(a, b int) int {
			return a - b
		},
		"sequence": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
	}
	
	allTemplates := append([]string{baseTemplate}, templates...)
	
	return template.New(filepath.Base(baseTemplate)).Funcs(funcMap).ParseFiles(allTemplates...)
}