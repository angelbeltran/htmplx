package htmplx

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func NewHandler[Data any](
	dir fs.FS,
	dataFn func(*http.Request) Data,
	funcMapFn func(*http.Request) template.FuncMap,
) *Handler[Data] {
	return &Handler[Data]{
		fs:      dir,
		data:    dataFn,
		funcMap: funcMapFn,
	}
}

func NewHandlerForDirectory[Data any](
	dir string,
	dataFn func(*http.Request) Data,
	funcMapFn func(*http.Request) template.FuncMap,
) *Handler[Data] {
	return NewHandler[Data](os.DirFS(dir), dataFn, funcMapFn)
}

type Handler[Data any] struct {
	fs      fs.FS
	data    func(*http.Request) Data
	funcMap func(*http.Request) template.FuncMap
}

func (h *Handler[Data]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	var pathParts []string
	if cleanPath := strings.Trim(r.URL.Path, "/"); cleanPath != "" {
		pathParts = strings.Split(cleanPath, "/")
	}

	l := slog.With("path", r.URL.Path)
	l.Debug("handling request")
	defer l.Debug("request served")

	l.Debug("loading templates")

	t, err := requestHandler{
		fs:  h.fs,
		log: l,
	}.loadTemplates(pathParts)
	if err != nil {
		if errors.Is(err, errNotFound) {
			l.Debug("not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		l.With("error", err).
			Error("internal server error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	var data Data
	if h.data != nil {
		data = h.data(r)
	}

	if err := t.Execute(w, data); err != nil {
		l.With("error", err).
			Error("failed to execute template")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type requestHandler struct {
	fs  fs.FS
	log *slog.Logger
}

func (h requestHandler) loadTemplates(path []string) (*template.Template, error) {
	layout, err := layoutTemplate.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone layout template: %w", err)
	}

	var bodyFound bool

	h.log.Debug("loading head.html.tmpl")
	if _, err := loadTemplate(layout, h.fs, "head", "head.html.tmpl"); err != nil {
		if !errors.Is(err, errNotFound) {
			return nil, err
		}
		h.log.Debug("head.html.tmpl not found at root")
	}

	h.log.Debug("loading body.html.tmpl")
	if _, err := loadTemplate(layout, h.fs, "body", "body.html.tmpl"); err != nil {
		if !errors.Is(err, errNotFound) {
			return nil, err
		}
		h.log.Debug("body.html.tmpl not found at root")
	} else {
		bodyFound = true
	}

	if len(path) > 0 {
		h.log.Debug("loading templates under path")
		var err error
		if bodyFound, err = h.loadLayoutTemplatesAlongPath(layout, h.fs, path); err != nil {
			return nil, err
		}
	}

	if !bodyFound {
		return nil, fmt.Errorf("%w: no body defined", errNotFound)
	}

	return layout, nil
}

var errNotFound = fmt.Errorf("not found")

func loadTemplate(t *template.Template, fsys fs.FS, name, path string) (*template.Template, error) {
	f, err := fsys.Open(path)
	if err != nil {
		var perr *fs.PathError
		if !errors.As(err, &perr) || !errors.Is(perr.Err, fs.ErrNotExist) {
			return nil, fmt.Errorf("failed to look up %s: %w", path, err)
		}

		return nil, errNotFound
	}

	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", path, err)
	}

	return t.New(name).Parse(string(b))
}

func (h requestHandler) loadLayoutTemplatesAlongPath(layout *template.Template, fsys fs.FS, path []string) (bodyFound bool, err error) {
	if len(path) == 0 {
		return false, nil
	}

	dir := path[0]
	h.log = h.log.With("directory", dir)

	f, err := fsys.Open(dir)
	if err != nil {
		var perr *fs.PathError
		if !errors.As(err, &perr) || !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("failed to open directory %s: %w", dir, err)
		}

		h.log.Debug("directory not found")
		return false, errNotFound
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("failed to get info on %s: %w", dir, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory: %w", dir, err)
	}

	const htmlTmplExt = ".html.tmpl"

	h.log.Debug("walking directory " + dir)

	var templateFilesFound []string
	if err := fs.WalkDir(fsys, dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.Name() == dir {
			return nil
		}
		if e.IsDir() {
			return fs.SkipDir
		}

		name := e.Name()
		h.log.Debug("template file found: " + name)
		if strings.HasSuffix(name, htmlTmplExt) {
			templateFilesFound = append(templateFilesFound, name)
		}

		return nil
	}); err != nil {
		return false, fmt.Errorf("failed to look up entries in %s: %w", dir, err)
	}

	rawTemplatesByName := make(map[string][]byte, len(templateFilesFound))
	for _, filename := range templateFilesFound {
		templateName := strings.TrimSuffix(filename, htmlTmplExt)
		if templateName == "" {
			return false, fmt.Errorf("template file found without name: %s", htmlTmplExt)
		}

		relativeFilename := dir + "/" + filename

		f, err := fsys.Open(relativeFilename)
		if err != nil {
			return false, fmt.Errorf("failed to open %s: %w", relativeFilename, err)
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", relativeFilename, err)
		}

		rawTemplatesByName[templateName] = b
	}

	h.log.Debug("overwriting templates with templates in child directories")
	for name, b := range rawTemplatesByName {
		if _, err := layout.New(name).Parse(string(b)); err != nil {
			return false, fmt.Errorf("failed to parse template %s: %w", name, err)
		}
	}

	subFsys, err := fs.Sub(fsys, dir)
	if err != nil {
		return false, err
	}
	h.log = h.log.With("subpath", path[1:])
	h.log.Debug("loading templates under subpath")
	if bodyFound, err = h.loadLayoutTemplatesAlongPath(layout, subFsys, path[1:]); err != nil {
		return bodyFound, err
	}

	if !bodyFound {
		_, bodyFound = rawTemplatesByName["body"]
	}

	return bodyFound, nil
}
