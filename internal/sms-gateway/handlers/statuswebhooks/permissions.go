package statuswebhooks

// Distinct from webhooks:list/write/delete (the existing device-dispatched
// webhooks), since these are a different resource with different semantics:
// server-dispatched, account-owned, message-status events rather than
// device-originated SMS lifecycle events.
const (
	ScopeList   = "status-webhooks:list"
	ScopeWrite  = "status-webhooks:write"
	ScopeDelete = "status-webhooks:delete"
)
