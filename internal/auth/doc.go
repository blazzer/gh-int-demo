// Package auth provides a minimal MCP-layer identity sketch for gh-int-demo.
//
// Production would replace this with OAuth 2.1 resource-server validation and
// per-session token binding. Today the server accepts either:
//   - Authorization: Bearer <user-token> (per-request identity), or
//   - GITHUB_TOKEN env fallback (shared demo identity on Fly).
package auth
