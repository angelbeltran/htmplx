package htmplx

import "io/fs"

type RequestData interface {
	SetPathExpressionSubmatches(matches []DirEntryWithSubmatches)
}

// RequestDataMap satisfies RequestData.
type RequestDataMap map[string]any

func (d RequestDataMap) SetPathExpressionSubmatches(matches []DirEntryWithSubmatches) {
	d["pathExpressionSubmatches"] = matches

	// any named submatches are accessible by name
	for _, match := range matches {
		for _, kv := range match.Submatches {
			if kv.Key != "" {
				d[kv.Key] = kv.Value
			}
		}
	}
}

// PathExpressionSubmatches satisfies RequestData.
// An alternative to RequestDataMap.
// Embed into a struct type to satisfy RequestData and capture path expression submatches.
type PathExpressionSubmatches map[string]string

func (m PathExpressionSubmatches) SetPathExpressionSubmatches(matches []DirEntryWithSubmatches) {
	// any named submatches are accessible by name
	for _, match := range matches {
		for _, kv := range match.Submatches {
			if kv.Key != "" {
				m[kv.Key] = kv.Value
			}
		}
	}
}

type DirEntryWithSubmatches struct {
	File       fs.FileInfo
	Submatches []KeyValuePair
}

type KeyValuePair struct {
	Key   string
	Value string
}
