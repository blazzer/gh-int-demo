package obs_test

import (
	"testing"

	"github.com/blazzer/gh-int-demo/internal/obs"
)

func TestRecordGitHubDuration_Stats(t *testing.T) {
	t.Cleanup(obs.ResetGitHubDurationForTest)

	obs.RecordGitHubDuration(10)
	obs.RecordGitHubDuration(20)
	obs.RecordGitHubDuration(15)

	count, sum, max, last := obs.GitHubDurationStatsForTest()
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	if sum != 45 {
		t.Fatalf("sum = %d, want 45", sum)
	}
	if max != 20 {
		t.Fatalf("max = %d, want 20", max)
	}
	if last != 15 {
		t.Fatalf("last = %d, want 15", last)
	}
}
