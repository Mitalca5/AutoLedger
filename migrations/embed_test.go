package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"testing"
)

// Already applied on existing deployments: renaming would re-apply them.
var knownDuplicatePrefixes = map[string]bool{"000035": true}

var migrationName = regexp.MustCompile(`^(\d{6})_[a-z0-9_]+\.(up|down)\.sql$`)

func TestMigrationPrefixesAreUnique(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatal(err)
	}

	ups := map[string][]string{}
	downs := map[string]int{}
	for _, e := range entries {
		if e.Name() == "embed.go" || e.Name() == "embed_test.go" {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("%s does not match NNNNNN_name.(up|down).sql", e.Name())
			continue
		}
		if m[2] == "up" {
			ups[m[1]] = append(ups[m[1]], e.Name())
		} else {
			downs[m[1]]++
		}
	}

	prefixes := make([]string, 0, len(ups))
	for p := range ups {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes)

	for _, p := range prefixes {
		if len(ups[p]) > 1 && !knownDuplicatePrefixes[p] {
			t.Errorf("migration prefix %s is used by several files: %v", p, ups[p])
		}
		if downs[p] != len(ups[p]) {
			t.Errorf("migration prefix %s has %d up and %d down files", p, len(ups[p]), downs[p])
		}
	}
}

func TestKnownDuplicatePrefixesStillExist(t *testing.T) {
	for p := range knownDuplicatePrefixes {
		matches, _ := fs.Glob(FS, p+"_*.up.sql")
		if len(matches) < 2 {
			t.Errorf("prefix %s is no longer duplicated: remove it from knownDuplicatePrefixes", p)
		}
	}
}
