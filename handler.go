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
	"regexp"
	"strings"
)

func NewHandler[D RequestData](dir fs.FS) *Handler[D] {
	return &Handler[D]{
		fs: dir,
	}
}

func NewHandlerForDirectory[D RequestData](dir string) *Handler[D] {
	return &Handler[D]{
		fs: os.DirFS(dir),
	}
}

func (h *Handler[D]) WithData(data func(*http.Request) D) *Handler[D] {
	h.data = data
	return h
}

func (h *Handler[D]) WithFuncs(funcs func(*http.Request) template.FuncMap) *Handler[D] {
	h.funcs = funcs
	return h
}

type Handler[D RequestData] struct {
	fs    fs.FS
	data  func(*http.Request) D
	funcs func(*http.Request) template.FuncMap
}

func (h *Handler[D]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
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

	// load and compile templates

	layout := template.New("layout")

	if h.funcs != nil {
		layout.Funcs(h.funcs(r))
	}

	layout, err := layout.Parse(layoutTemplateString)
	if err != nil {
		l.With("error", fmt.Errorf("failed to parse layout template: %w", err)).
			Error("internal server error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.Debug("loading templates")

	pathExpSubmatches, err := rh.loadTemplatesOnPath(layout, pathParts)
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

	var data D
	if h.data != nil {
		data = h.data(r)
		data.SetPathExpressionSubmatches(pathExpSubmatches)
	}

	if err := layout.Execute(w, data); err != nil {
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

func (h requestHandler) loadTemplatesOnPath(layout *template.Template, path []string) (pathExpSubmatches []DirEntryWithSubmatches, err error) {
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
		if bodyFound, pathExpSubmatches, err = h.loadLayoutTemplatesAlongPath(layout, h.fs, path); err != nil {
			return nil, err
		}
	}

	if !bodyFound {
		return nil, fmt.Errorf("%w: no body defined", fs.ErrNotExist)
	}

	return pathExpSubmatches, nil
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

func (h requestHandler) loadLayoutTemplatesAlongPath(layout *template.Template, fsys fs.FS, path []string) (bodyFound bool, pathExpSubmatches []DirEntryWithSubmatches, err error) {
	if len(path) == 0 {
		return false, nil, nil
	}

	dir := path[0]
	h.log = h.log.With("directory", dir)

	// immediate fail urls with regex path parts so as to not expose regex paths directly
	if isRegexPathPart(dir) {
		h.log.Debug("path includes regex: " + dir)
		return false, nil, fmt.Errorf("%w: path includes regex: %s", fs.ErrNotExist, dir)
	}

	// find a directory by exact name or one that is a regex matching
	var dirExpSubmatches DirEntryWithSubmatches

	if info, err := h.stat(fsys, dir); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, nil, fmt.Errorf("failed to check directory %s: %w", dir, err)
		}

		h.log.Debug("looking up matching regex directories")
		matchingDirs, err := h.findMatchingRegexDirs(fsys, dir)
		if err != nil {
			return false, nil, err
		}
		if len(matchingDirs) == 0 {
			return false, nil, fmt.Errorf("directory not found: %s: %w", dir, fs.ErrNotExist)
		}

		for _, d := range matchingDirs {
			if len(d.Submatches) > len(dirExpSubmatches.Submatches) {
				dirExpSubmatches = d
			}
		}

		h.log.Debug("matching regex directory found: " + dirExpSubmatches.File.Name())
	} else if !info.IsDir() {
		return false, nil, fmt.Errorf("%s is not a directory: %w", dir, fs.ErrNotExist)
	} else {
		dirExpSubmatches = DirEntryWithSubmatches{
			File: info,
		}
	}

	dir = dirExpSubmatches.File.Name()
	h.log = h.log.With("directory", dir)

	// gather and compile all template files in the directory

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
		return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("failed to look up entries in %s: %w", dir, err)
	}

	rawTemplatesByName := make(map[string][]byte, len(templateFilesFound))
	for _, filename := range templateFilesFound {
		templateName := strings.TrimSuffix(filename, htmlTmplExt)
		if templateName == "" {
			return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("template file found without name: %s", htmlTmplExt)
		}

		relativeFilename := dir + "/" + filename

		f, err := fsys.Open(relativeFilename)
		if err != nil {
			return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("failed to open %s: %w", relativeFilename, err)
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("failed to read %s: %w", relativeFilename, err)
		}

		rawTemplatesByName[templateName] = b
	}

	h.log.Debug("overwriting templates with templates in child directories")
	for name, b := range rawTemplatesByName {
		if _, err := layout.New(name).Parse(string(b)); err != nil {
			return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("failed to parse template %s: %w", name, err)
		}
	}

	// continue gather templates in the remaining subpath.

	subFsys, err := fs.Sub(fsys, dir)
	if err != nil {
		return false, []DirEntryWithSubmatches{dirExpSubmatches}, err
	}

	subpath := path[1:]

	h.log = h.log.With("subpath", subpath)
	h.log.Debug("loading templates under subpath")

	var subpathExpSubmatches []DirEntryWithSubmatches
	bodyFound, subpathExpSubmatches, err = h.loadLayoutTemplatesAlongPath(layout, subFsys, subpath)
	pathExpSubmatches = append([]DirEntryWithSubmatches{dirExpSubmatches}, subpathExpSubmatches...)
	if err != nil {
		return bodyFound, pathExpSubmatches, err
	}

	if !bodyFound {
		_, bodyFound = rawTemplatesByName["body"]
	}

	return bodyFound, pathExpSubmatches, nil
}

func isRegexPathPart(part string) bool {
	return len(part) >= 3 && strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}

func trimRegexPathPart(part string) string {
	return part[1 : len(part)-1]
}

func (h requestHandler) stat(fsys fs.FS, dir string) (fs.FileInfo, error) {
	f, err := fsys.Open(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			h.log.Debug("directory not found")
		}

		return nil, fmt.Errorf("failed to open directory: %w", err)
	}
	defer f.Close()

	return f.Stat()
}

func (h requestHandler) findMatchingRegexDirs(fsys fs.FS, exp string) ([]DirEntryWithSubmatches, error) {
	h.log.Debug("listing directory entries")
	entries, err := h.listDirEntries(fsys)
	if err != nil {
		return nil, fmt.Errorf("failed to look up directory entries: %w", err)
	}

	h.log = h.log.With("entries", entries)
	h.log.Debug("directory entries found")

	var matchingEntries []DirEntryWithSubmatches

	for _, e := range entries {
		name := e.Name()

		if !isRegexPathPart(name) || !e.IsDir() {
			continue
		}

		l := h.log.With("entry", e.Name())
		l.Debug("checking regex directory entry")

		rawRegex := "^" + trimRegexPathPart(name) + "$"
		l = l.With("expression", rawRegex)
		l.Debug("compiling expression")

		re, err := regexp.Compile(rawRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex directory name: %w", err)
		}

		matches := re.FindStringSubmatch(exp)
		if matches != nil {
			l = l.With("matches", matches)
			l.Debug("matches found")

			info, err := e.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to look up file info on file %s: %w", e.Name(), err)
			}

			subexpNames := re.SubexpNames()[1:]
			submatches := matches[1:]

			kvs := make([]KeyValuePair, len(submatches))
			for i, m := range submatches {
				kvs[i] = KeyValuePair{
					Key:   subexpNames[i],
					Value: m,
				}
			}

			matchingEntries = append(matchingEntries, DirEntryWithSubmatches{
				File:       info,
				Submatches: kvs,
			})
		}
	}

	return matchingEntries, nil
}

func (h requestHandler) listDirEntries(fsys fs.FS) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry

	h.log.Debug("walking directory")
	err := fs.WalkDir(fsys, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		h.log.Debug("entry: " + e.Name())

		if e.Name() == "." {
			return nil
		}

		entries = append(entries, e)

		if e.IsDir() {
			return fs.SkipDir
		}

		return nil
	})

	return entries, err
}

func (h requestHandler) notFound(w http.ResponseWriter) {
	h.log.Debug("not found")
	w.WriteHeader(http.StatusNotFound)
}

func (h requestHandler) internalServerError(w http.ResponseWriter, err error) {
	h.log.With("error", err).Error("internal server error")
	w.WriteHeader(http.StatusInternalServerError)
}
