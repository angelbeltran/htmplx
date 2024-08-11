package htmplx

import (
	"bytes"
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
	"slices"
	"strings"
)

func NewHandler[D RequestData](dir fs.FS) *Handler[D] {
	return &Handler[D]{
		log: newLogger(),
		fs:  dir,
	}
}

func NewHandlerForDirectory[D RequestData](dir string) *Handler[D] {
	return NewHandler[D](os.DirFS(dir))
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
	log   *slog.Logger
	fs    fs.FS
	data  func(*http.Request) D
	funcs func(*http.Request) template.FuncMap
}

func (h *Handler[D]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	l := h.log.With("path", r.URL.Path)
	l.Debug("handling request")
	defer l.Debug("request served")

	out, contentType, err := h.ServeFile(r)
	if err != nil {
		l.With("error", err).
			Error("internal server error")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if out == nil {
		l.Warn("not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	io.Copy(w, out)
}

func (h *Handler[D]) ServeFile(r *http.Request) (
	out io.Reader,
	contentType string,
	err error,
) {

	urlPath := r.URL.Path

	l := h.log.With("path", urlPath)

	var pathParts []string
	if cleanPath := strings.Trim(urlPath, "/"); cleanPath != "" {
		pathParts = strings.Split(cleanPath, "/")
	}

	l = l.With("pathArray", pathParts)

	rh := requestHandler{
		fs:  h.fs,
		log: l,
	}

	// explicit filenames with file extension should result in a simple file lookup.
	if ext := path.Ext(urlPath); ext != "" {
		if ext == ".tmpl" {
			// templates are not visible
			return nil, "", nil
		}

		l.Debug("attempting to serve file")

		return rh.readFileAndContentType(strings.TrimPrefix(urlPath, "/"))
	}

	// load and compile templates

	layout := template.New("layout")

	if h.funcs != nil {
		layout = layout.Funcs(h.funcs(r))
	}

	layout, err = layout.Parse(layoutTemplateString)
	if err != nil {
		err = fmt.Errorf("failed to parse layout template: %w", err)
		l.With("error", err).
			Error("internal server error")
		return nil, "", err
	}

	l.Debug("loading templates")

	pathExpSubmatches, err := rh.loadTemplates(layout, pathParts)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		}

		l.With("error", err).
			Error("internal server error")
		return nil, "", err
	}

	var data D
	if h.data != nil {
		data = h.data(r)
		data.SetPathExpressionSubmatches(pathExpSubmatches)
	}

	var buf bytes.Buffer

	if err := layout.Execute(&buf, data); err != nil {
		l.With("error", err).
			Error("failed to execute template")
		return nil, "", fmt.Errorf("failed to execute template: %w", err)
	}

	return &buf, "text/html", nil
}

type requestHandler struct {
	fs  fs.FS
	log *slog.Logger
}

func (h requestHandler) serveFile(w http.ResponseWriter, filename string) {
	f, contentType, err := h.readFileAndContentType(filename)
	if err != nil {
		h.internalServerError(w, err)
		return
	}
	if f == nil {
		h.notFound(w)
		return
	}

	h.log = h.log.With("Content-Type", contentType)
	h.log.Debug("setting Content-Type header")
	w.Header().Set("Content-Type", contentType)
	h.log.Debug("writing file to response body")

	io.Copy(w, f)
	h.log.Debug("file served")
}

func (h requestHandler) readFileAndContentType(filename string) (
	out io.Reader,
	contentType string,
	err error,
) {
	f, err := h.fs.Open(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		} else {
			return nil, "", fmt.Errorf("failed to look up %s: %w", filename, err)
		}
		return
	}
	defer f.Close()

	h.log.Debug("sniffing content type")
	contentType = mime.TypeByExtension(path.Ext(filename))
	h.log.Debug("content type by file extension: " + contentType)

	var bytesRead []byte

	if contentType == "" {
		contentType, bytesRead, err = h.sniffContentType(f)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read file %s: %w", filename, err)
		}
	}

	var buf bytes.Buffer

	if len(bytesRead) > 0 {
		if _, err := buf.Write(bytesRead); err != nil {
			return nil, contentType, fmt.Errorf("unexpected error while writing buffer: %w", err)
		}
	}

	if _, err := io.Copy(&buf, f); err != nil {
		return nil, contentType, fmt.Errorf("unexpected error while writing buffer: %w", err)
	}

	return &buf, contentType, nil
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

func (h requestHandler) loadTemplates(layout *template.Template, path []string) (pathExpSubmatches []DirEntryWithSubmatches, err error) {
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
		if bodyFound, pathExpSubmatches, err = h.loadTemplatesAlongPath(layout, path); err != nil {
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

func (h requestHandler) loadTemplatesAlongPath(
	layout *template.Template,
	path []string,
) (
	bodyFound bool,
	pathExpSubmatches []DirEntryWithSubmatches,
	err error,
) {
	return h.loadTemplatesAlongPathStartingAtIndex(layout, path, 0)
}

func (h requestHandler) loadTemplatesAlongPathStartingAtIndex(
	layout *template.Template,
	path []string,
	pathIndex int,
) (
	bodyFound bool,
	pathExpSubmatches []DirEntryWithSubmatches,
	err error,
) {
	path = slices.Clone(path)
	h.log = h.log.With("pathIndex", pathIndex)

	if pathIndex >= len(path) {
		// at the last directory in the path.
		// handle special cases:
		// - 404 file means return a 404 Not Found response

		h.log.Debug("checking for 404 file")
		exists, err := h.does404FileExist(path)
		if err == nil && exists {
			err = fmt.Errorf("404 file found: %w", fs.ErrNotExist)
		}

		return false, nil, err
	}

	currentDir := slices.Clone(path[:pathIndex])

	dir := path[pathIndex]

	// immediate fail urls with regex path parts so as to not expose regex paths directly
	if isRegexPathPart(dir) {
		h.log.Debug("path includes regex: " + dir)
		return false, nil, fmt.Errorf("%w: path includes regex: %s", fs.ErrNotExist, dir)
	}

	// find a directory by exact name or one that is a regex matching
	var dirExpSubmatches DirEntryWithSubmatches

	if info, err := fs.Stat(h.fs, strings.Join(append(currentDir, dir), "/")); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false, nil, fmt.Errorf("failed to check directory %s: %w", dir, err)
		}

		h.log.Debug("looking up matching regex directories")
		matchingDirs, err := h.findMatchingRegexDirs(strings.Join(currentDir, "/"), dir)
		if err != nil {
			return false, nil, err
		}
		if len(matchingDirs) == 0 {
			return false, nil, fmt.Errorf("directory not found: %s: %w", dir, fs.ErrNotExist)
		}

		dirExpSubmatches = matchingDirs[0]
		for _, d := range matchingDirs[1:] {
			if len(d.Submatches) > len(dirExpSubmatches.Submatches) {
				dirExpSubmatches = d
			}
		}

		h.log.Debug("matching regex directory found: " + dirExpSubmatches.File.Name())

		dir = dirExpSubmatches.File.Name()
		path = slices.Clone(path)
		path[pathIndex] = dir

		h.log = h.log.With("pathRegexMatch", dir)
	} else if !info.IsDir() {
		return false, nil, fmt.Errorf("%s is not a directory: %w", dir, fs.ErrNotExist)
	} else {
		dirExpSubmatches = DirEntryWithSubmatches{
			File: info,
		}
	}

	// gather and compile all template files in the directory

	const htmlTmplExt = ".html.tmpl"

	h.log.Debug("walking directory " + dir)

	var templateFilesFound []string

	fullDirName := strings.Join(append(currentDir, dir), "/")
	if err := fs.WalkDir(h.fs, fullDirName, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		h.log.Debug("file found: " + path)
		if path == fullDirName {
			return nil
		}
		if e.IsDir() {
			return fs.SkipDir
		}

		name := e.Name()
		if strings.HasSuffix(name, htmlTmplExt) {
			h.log.Debug("template file found: " + name)
			templateFilesFound = append(templateFilesFound, name)
		}

		return nil
	}); err != nil {
		return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("failed to look up entries in %s: %w", dir, err)
	}

	h.log.Debug("templates found: [" + strings.Join(templateFilesFound, ", ") + "]")

	rawTemplatesByName := make(map[string][]byte, len(templateFilesFound))
	for _, filename := range templateFilesFound {
		templateName := strings.TrimSuffix(filename, htmlTmplExt)
		if templateName == "" {
			return false, []DirEntryWithSubmatches{dirExpSubmatches}, fmt.Errorf("template file found without name: %s", htmlTmplExt)
		}

		relativeFilename := strings.Join(append(currentDir, dir, filename), "/")

		f, err := h.fs.Open(relativeFilename)
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

	// continue gather templates in the remaining path.

	var subpathExpSubmatches []DirEntryWithSubmatches
	bodyFound, subpathExpSubmatches, err = h.loadTemplatesAlongPathStartingAtIndex(layout, path, pathIndex+1)
	pathExpSubmatches = append([]DirEntryWithSubmatches{dirExpSubmatches}, subpathExpSubmatches...)
	if err != nil {
		return bodyFound, pathExpSubmatches, err
	}

	if !bodyFound {
		_, bodyFound = rawTemplatesByName["body"]
	}

	return bodyFound, pathExpSubmatches, nil
}

func (h requestHandler) does404FileExist(dir []string) (bool, error) {
	_, err := fs.Stat(h.fs, strings.Join(append(dir, "404"), "/"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("failed to look up 404 file: %w", err)
}

func isRegexPathPart(part string) bool {
	return len(part) >= 3 && strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
}

func trimRegexPathPart(part string) string {
	return part[1 : len(part)-1]
}

func (h requestHandler) findMatchingRegexDirs(parentDir, exp string) ([]DirEntryWithSubmatches, error) {
	h.log.Debug("listing directory entries under " + parentDir)
	entries, err := h.listDirEntries(parentDir)
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

func (h requestHandler) listDirEntries(dirName string) ([]fs.DirEntry, error) {
	if dirName == "" {
		dirName = "."
	}

	var entries []fs.DirEntry

	h.log.Debug("walking directory " + dirName)
	err := fs.WalkDir(h.fs, dirName, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dirName {
			return nil
		}

		h.log.Debug("entry: " + path)

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
