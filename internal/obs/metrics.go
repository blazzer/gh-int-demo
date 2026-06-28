package obs

import (
	"expvar"
	"fmt"
	"sync"
)

var (
	mcpRequestsTotal      = expvar.NewMap("mcp_requests_total")
	githubRequests        = expvar.NewMap("github_requests_total")
	githubDurationMS      = expvar.NewMap("github_request_duration_ms")
	githubDurationMSCount = new(expvar.Int)
	githubDurationMSSum   = new(expvar.Int)
	githubDurationMSMax   = new(expvar.Int)
	githubDurationMSLast  = new(expvar.Int)
)

func init() {
	githubDurationMS.Set("count", githubDurationMSCount)
	githubDurationMS.Set("sum", githubDurationMSSum)
	githubDurationMS.Set("max", githubDurationMSMax)
	githubDurationMS.Set("last", githubDurationMSLast)
}

var githubDurationMu sync.Mutex

// IncMCPRequest increments the counter for an MCP method.
func IncMCPRequest(method string) {
	mcpRequestsTotal.Add(method, 1)
}

// IncGitHubRequest increments the counter for a GitHub response status class.
func IncGitHubRequest(statusCode int) {
	class := fmt.Sprintf("%dxx", statusCode/100)
	githubRequests.Add(class, 1)
}

// RecordGitHubDuration records request duration in milliseconds (count, sum, max, last).
func RecordGitHubDuration(ms int64) {
	githubDurationMu.Lock()
	defer githubDurationMu.Unlock()

	githubDurationMSCount.Add(1)
	githubDurationMSSum.Add(ms)
	githubDurationMSLast.Set(ms)
	if ms > githubDurationMSMax.Value() {
		githubDurationMSMax.Set(ms)
	}
}

// GitHubDurationStatsForTest returns recorded GitHub latency stats (tests only).
func GitHubDurationStatsForTest() (count, sum, max, last int64) {
	githubDurationMu.Lock()
	defer githubDurationMu.Unlock()
	return githubDurationMSCount.Value(), githubDurationMSSum.Value(), githubDurationMSMax.Value(), githubDurationMSLast.Value()
}

// ResetGitHubDurationForTest clears GitHub latency stats (tests only).
func ResetGitHubDurationForTest() {
	githubDurationMu.Lock()
	defer githubDurationMu.Unlock()
	githubDurationMSCount.Set(0)
	githubDurationMSSum.Set(0)
	githubDurationMSMax.Set(0)
	githubDurationMSLast.Set(0)
}
