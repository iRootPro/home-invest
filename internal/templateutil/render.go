package templateutil

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	templateFS  fs.FS
	funcMap     template.FuncMap
	cache       map[string]*template.Template
	cacheMu     sync.RWMutex
	partials    []string
)

func Init(fsys fs.FS) error {
	templateFS = fsys
	funcMap = template.FuncMap{
		"formatDate": func(s string) string {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				t, err = time.Parse(time.RFC3339, s)
			}
			if err != nil {
				return s
			}
			return t.Format("02.01.2006")
		},
		"formatAmount": func(f float64) string {
			return formatHuman(f)
		},
		"pct": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b * 100
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"asvColor": func(amount float64) string {
			ratio := amount / 1400000
			if ratio >= 1.0 {
				return "red"
			}
			if ratio >= 0.8 {
				return "orange"
			}
			return "green"
		},
		"seq": func(items ...any) []any {
			return items
		},
	}
	cache = make(map[string]*template.Template)

	// Collect partial templates
	partials = []string{"templates/base.html"}
	entries, err := fs.ReadDir(fsys, "templates/partials")
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				partials = append(partials, "templates/partials/"+e.Name())
			}
		}
	}

	return nil
}

func getTemplate(name string) (*template.Template, error) {
	cacheMu.RLock()
	t, ok := cache[name]
	cacheMu.RUnlock()
	if ok {
		return t, nil
	}

	files := make([]string, len(partials))
	copy(files, partials)
	files = append(files, "templates/"+name)

	// Scan template content for {{template "xxx"}} references and include them
	content, err := fs.ReadFile(templateFS, "templates/"+name)
	if err == nil {
		refs := findTemplateRefs(string(content))
		for _, ref := range refs {
			path := "templates/" + ref
			if _, err := fs.Stat(templateFS, path); err == nil {
				files = append(files, path)
			}
		}
	}

	t, err = template.New("").Funcs(funcMap).ParseFS(templateFS, files...)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	cacheMu.Lock()
	cache[name] = t
	cacheMu.Unlock()

	return t, nil
}

// findTemplateRefs extracts template names from {{template "name" ...}} calls
func findTemplateRefs(content string) []string {
	var refs []string
	for _, prefix := range []string{`{{template "`, `{{ template "`} {
		s := content
		for {
			idx := strings.Index(s, prefix)
			if idx == -1 {
				break
			}
			s = s[idx+len(prefix):]
			end := strings.Index(s, `"`)
			if end == -1 {
				break
			}
			ref := s[:end]
			if strings.Contains(ref, "/") && strings.HasSuffix(ref, ".html") {
				refs = append(refs, ref)
			}
			s = s[end:]
		}
	}
	return refs
}

func Render(w http.ResponseWriter, name string, data any) {
	t, err := getTemplate(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func RenderPartial(w http.ResponseWriter, name string, data any) {
	t, err := getTemplate(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func formatHuman(f float64) string {
	sign := ""
	if f < 0 {
		sign = "-"
		f = -f
	}
	switch {
	case f >= 1_000_000:
		v := f / 1_000_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%s%d млн", sign, int(v))
		}
		return fmt.Sprintf("%s%.1f млн", sign, v)
	case f >= 1_000:
		v := f / 1_000
		if v == float64(int(v)) {
			return fmt.Sprintf("%s%d тыс.", sign, int(v))
		}
		return fmt.Sprintf("%s%.1f тыс.", sign, v)
	default:
		if f == float64(int(f)) {
			return fmt.Sprintf("%s%d", sign, int(f))
		}
		return fmt.Sprintf("%s%.2f", sign, f)
	}
}

func addThousandsSep(s string) string {
	parts := strings.Split(s, ".")
	intPart := parts[0]
	negative := false
	if len(intPart) > 0 && intPart[0] == '-' {
		negative = true
		intPart = intPart[1:]
	}
	var result []byte
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ' ')
		}
		result = append(result, byte(c))
	}
	res := string(result)
	if negative {
		res = "-" + res
	}
	if len(parts) > 1 {
		return res + "." + parts[1]
	}
	return res
}
