package htmplx

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
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

	l := slog.With("path", r.URL.Path)
	l.Debug("handling request")
	defer l.Debug("request served")

	var pathParts []string
	if cleanPath := strings.Trim(r.URL.Path, "/"); cleanPath != "" {
		pathParts = strings.Split(cleanPath, "/")
	}

	rh := requestHandler{
		fs:  h.fs,
		log: l,
	}

	// explicit filenames with file extension should result in a simple file lookup.
	if ext := path.Ext(r.URL.Path); ext != "" {
		if ext == ".tmpl" {
			// templates are not visible
			rh.notFound(w)
			return
		}

		l.Debug("attempting to serve file")
		rh.serveFile(w, strings.TrimPrefix(r.URL.Path, "/"))
		return
	}

	l.Debug("loading templates")

	t, err := rh.loadTemplates(pathParts)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			rh.notFound(w)
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

func (h requestHandler) serveFile(w http.ResponseWriter, filename string) {
	f, err := h.fs.Open(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			h.notFound(w)
		} else {
			h.internalServerError(w, fmt.Errorf("failed to look up %s: %w", filename, err))
		}
		return
	}
	defer f.Close()

	h.log.Debug("sniffing content type")
	contentType := mime.TypeByExtension(path.Ext(filename))
	h.log.Debug("content type by file extension: " + contentType)

	var bytesRead []byte

	if contentType == "" {
		contentType, bytesRead, err = h.sniffContentType(f)
		if err != nil {
			h.internalServerError(w, fmt.Errorf("failed to read file %s: %w", filename, err))
			return
		}
	}

	h.log = h.log.With("Content-Type", contentType)
	h.log.Debug("setting Content-Type header")
	w.Header().Set("Content-Type", contentType)
	h.log.Debug("writing file to response body")
	if len(bytesRead) > 0 {
		w.Write(bytesRead)
	}

	io.Copy(w, f)
	h.log.Debug("file served")
}

func (h requestHandler) sniffContentType(f fs.File) (contentType string, bytesRead []byte, err error) {
	p := make([]byte, 512)
	numBytesRead := 0

	for {
		h.log.Debug("reading bytes up to " + fmt.Sprint(512-numBytesRead) + " bytes")
		var n int
		n, err = f.Read(p[numBytesRead:])
		numBytesRead += n
		h.log.Debug(fmt.Sprint(n) + " bytes read")
		h.log.Debug(fmt.Sprint(numBytesRead) + " total bytes read")

		if err != nil || n == 0 || numBytesRead >= len(p) {
			break
		}
	}

	bytesRead = p[:numBytesRead]

	if err == nil || errors.Is(err, io.EOF) {
		contentType = http.DetectContentType(bytesRead)
		err = nil
	}

	return contentType, bytesRead, err
}

func (h requestHandler) loadTemplates(path []string) (*template.Template, error) {
	layout, err := layoutTemplate.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone layout template: %w", err)
	}

	var bodyFound bool

	h.log.Debug("loading head.html.tmpl")
	if _, err := h.loadTemplate(layout, "head", "head.html.tmpl"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		h.log.Debug("head.html.tmpl not found at root")
	}

	h.log.Debug("loading body.html.tmpl")
	if _, err := h.loadTemplate(layout, "body", "body.html.tmpl"); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
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
		return nil, fmt.Errorf("%w: no body defined", fs.ErrNotExist)
	}

	return layout, nil
}

func (h requestHandler) loadTemplate(t *template.Template, name, path string) (*template.Template, error) {
	b, err := h.readFile(path)
	if err != nil {
		return nil, err
	}

	return t.New(name).Parse(string(b))
}

func (h requestHandler) readFile(path string) ([]byte, error) {
	f, err := h.fs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", path, err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to look up %s: %w", path, err)
	}

	return b, nil
}

func (h requestHandler) loadLayoutTemplatesAlongPath(layout *template.Template, fsys fs.FS, path []string) (bodyFound bool, err error) {
	if len(path) == 0 {
		return false, nil
	}

	dir := path[0]
	h.log = h.log.With("directory", dir)

	f, err := fsys.Open(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			h.log.Debug("directory not found")
		}

		return false, fmt.Errorf("failed to open directory %s: %w", dir, err)
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

func (h requestHandler) notFound(w http.ResponseWriter) {
	h.log.Debug("not found")
	w.WriteHeader(http.StatusNotFound)
}

func (h requestHandler) internalServerError(w http.ResponseWriter, err error) {
	h.log.With("error", err).Error("internal server error")
	w.WriteHeader(http.StatusInternalServerError)
}
