package calendarcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These functions deliberately walk month buckets starting on day 1. Counts are exact:
// an extra call in an already approved function still needs a review.
var firstOfMonthSites = map[string]struct {
	count  int
	reason string
}{
	"internal/database/repository_fleet.go:GetFleetSummary":                              {2, "startOfMonth is constructed on day 1; startOfHistory derives from it"},
	"internal/services/charging_place_stats.go:bounds":                                   {1, "time.Date explicitly sets the day to 1"},
	"internal/services/energy_stats.go:previousMonths":                                   {1, "parsing YYYY-MM starts on day 1"},
	"internal/services/smoothing.go:allocateMissingKmByMonth":                            {1, "cur is constructed on day 1 and advances whole month buckets"},
	"internal/services/tco_maintenance_amortization.go:calculateMaintenanceAmortization": {1, "first and last are parsed YYYY-MM month buckets"},
	"internal/services/tco_monthly.go:fillMonths":                                        {1, "first is parsed from YYYY-MM"},
	"internal/services/tco_tire_amortization.go:computeMonthlyTireAmortization":          {1, "startMonth is parsed from YYYY-MM"},
	"internal/services/tire_forecast.go:coveredMonths":                                   {1, "d is constructed on day 1"},
	"internal/services/tire_forecast.go:monthlyKmProfile":                                {2, "thisMonth starts on day 1; start derives from it"},
}

func monthAdds(fset *token.FileSet, filename string, source any) (map[string]int, error) {
	f, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return nil, err
	}
	found := map[string]int{}
	for _, declaration := range f.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		name := "<package>"
		var node ast.Node = declaration
		if ok {
			if fn.Body == nil {
				continue
			}
			name, node = fn.Name.Name, fn.Body
		}
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "AddDate" {
				return true
			}
			month := call.Args[1]
			for {
				p, ok := month.(*ast.ParenExpr)
				if !ok {
					break
				}
				month = p.X
			}
			if value, ok := month.(*ast.BasicLit); ok && value.Kind == token.INT && value.Value == "0" {
				return true
			}
			found[name]++
			return true
		})
	}
	return found, nil
}

func TestMonthAdditionsRequireCalendarClampOrReviewedMonthBucket(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "../.."))
	seen := map[string]int{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			calls, err := monthAdds(token.NewFileSet(), path, nil)
			if err != nil {
				return err
			}
			for fn, count := range calls {
				key := filepath.ToSlash(rel) + ":" + fn
				seen[key] = count
				if _, allowed := firstOfMonthSites[key]; !allowed {
					t.Errorf("%s uses AddDate with months: use models.AddCalendarMonths or document why its date always starts on day 1", key)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for key, allowed := range firstOfMonthSites {
		if allowed.reason == "" || seen[key] != allowed.count {
			t.Errorf("review month-bucket exception %s: got %d calls, expected %d (%s)", key, seen[key], allowed.count, allowed.reason)
		}
	}
}

func TestMonthAdditionScanner(t *testing.T) {
	const source = `package sample
var next = t.AddDate(0, 1, 0)
func calendar(t Clock, n int) { t.AddDate(0, n, 0); t.AddDate(1, 2, 3); t.AddDate(0, (1), 0) }
func days(t Clock) { t.AddDate(0, (0), -30); t.AddDate(1, 0, 0) }
func clamp(t Clock) { AddCalendarMonths(t, 1) }
`
	found, err := monthAdds(token.NewFileSet(), "sample.go", source)
	if err != nil {
		t.Fatal(err)
	}
	if found["calendar"] != 3 || found["days"] != 0 || found["clamp"] != 0 || found["<package>"] != 1 {
		t.Fatalf("unexpected month-addition calls: %v", found)
	}
}
