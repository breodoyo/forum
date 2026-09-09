package render

import (
	"html/template"
	"net/http"
	"path/filepath"
)

var templates map[string]*template.Template

var partials = []string{"base.html", "header.html", "footer.html", "sidebar.html"}

// Load compiles all template pages together with the shared partials at
// startup. A page that fails to parse aborts startup so template errors
// surface immediately instead of rendering empty pages at request time.
func Load(templateDir string) error {
	templates = make(map[string]*template.Template)

	pages, err := filepath.Glob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Build path list including partials
		files := []string{}
		for _, p := range partials {
			files = append(files, filepath.Join(templateDir, p))
		}
		files = append(files, page)

		tmpl, err := template.ParseFiles(files...)
		if err != nil {
			return err
		}
		templates[name] = tmpl
	}

	return nil
}

// Page renders a parsed template with data.
func Page(w http.ResponseWriter, name string, data any) {
	tmpl, exists := templates[name]
	if !exists {
		// Fallback: parse the requested page together with the shared
		// partials (e.g. for templates added after startup).
		files := make([]string, 0, len(partials)+1)
		for _, p := range partials {
			files = append(files, filepath.Join("web/templates", p))
		}
		files = append(files, filepath.Join("web/templates", name))

		var err error
		tmpl, err = template.ParseFiles(files...)
		if err != nil {
			http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
