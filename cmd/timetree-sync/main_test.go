package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/timetreesync"
)

func TestFormatSummaryReportsOnlyActualWrites(t *testing.T) {
	s := timetreesync.Summary{Results: []timetreesync.GroupResult{
		{Group: "ribonclock", Fetched: 12, Changed: 3, Written: true, Conflicts: []string{"ambiguous"}},
		{Group: "dollykiss", Fetched: 8, Changed: 2, Error: "write failed"},
	}}
	got := formatSummary(s, errors.New("failed"))
	for _, want := range []string{"一部失敗", "リボンクロック: 取得12件・更新3件・保留1件", "Dolly Kiss: 取得8件・更新0件・保留0件・未反映2件・失敗"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}
