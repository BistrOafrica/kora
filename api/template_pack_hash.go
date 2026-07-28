package api

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/asenawritescode/kora/doctype"
)

// setTemplatePackHash keeps the CMS-owned hash in sync with the child rows.
// The field is read-only to API clients, but must be written by the server
// whenever a Template Pack is created or updated.
func setTemplatePackHash(dt *doctype.DocType, doc *doctype.Document) {
	if dt == nil || doc == nil || dt.Name != "Template Pack" {
		return
	}

	type packFile struct {
		path    string
		content string
	}
	files := make([]packFile, 0, len(doc.GetTable("template_files")))
	for _, row := range doc.GetTable("template_files") {
		if row == nil {
			continue
		}
		files = append(files, packFile{
			path:    row.GetString("path"),
			content: row.GetString("content"),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	h := sha256.New()
	for _, file := range files {
		_, _ = h.Write([]byte(file.path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(file.content))
		_, _ = h.Write([]byte{0})
	}
	doc.Set("config_hash", hex.EncodeToString(h.Sum(nil)))
}
