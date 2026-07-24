package doctype

import "testing"

func TestConfigDiffFull_NilDoctypesIsSafe(t *testing.T) {
	diff := &ConfigDiffFull{
		SectionChanges: []SectionChange{{Section: "views", Change: "modified", Name: "POS"}},
	}
	if got := diff.Summary(); got != "0 added, 0 removed, 0 changed (0 breaking), 1 section changes" {
		t.Fatalf("Summary() = %q", got)
	}
	if got := diff.BreakingChanges(); got != nil {
		t.Fatalf("BreakingChanges() = %#v, want nil", got)
	}
}
