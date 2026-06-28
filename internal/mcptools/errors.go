package mcptools

// ToolErrorCode classifies MCP tool failures for agents and structured logs.
type ToolErrorCode string

const (
	CodeUnauthorized ToolErrorCode = "UNAUTHORIZED"
	CodeRateLimited  ToolErrorCode = "RATE_LIMITED"
	CodeUpstream     ToolErrorCode = "UPSTREAM"
	CodeInternal     ToolErrorCode = "INTERNAL"
)

// ToolError is the structured error payload returned to MCP clients.
type ToolError struct {
	Code      ToolErrorCode `json:"code"`
	Message   string        `json:"message"`
	Retryable bool          `json:"retryable"`
}
