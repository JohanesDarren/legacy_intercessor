// Package migrations embeds the SQL migration files so the API binary is
// self-contained and needs no migration tool at runtime.
package migrations

import (
	"embed"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Up returns every up migration as (version, sql) pairs, sorted by version.
func Up() ([]Migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	out := make([]Migration, 0, len(names))
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out = append(out, Migration{
			Version: strings.TrimSuffix(name, ".up.sql"),
			SQL:     string(body),
		})
	}
	return out, nil
}

// Migration is one versioned SQL file.
type Migration struct {
	Version string
	SQL     string
}
