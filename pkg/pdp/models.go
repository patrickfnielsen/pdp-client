package pdp

import (
	"context"
	"time"

	"log/slog"
)

type DecisionResult struct {
	ID          string      `json:"decisionId"`  // a unique identifier for this decision (which is included in the decision log.)
	Result      interface{} `json:"result"`      // the output of query evaluation.
	Path        string      `json:"path"`        // the path of query evaluation.
	Input       interface{} `json:"input"`       // the path of query evaluation.
	RequestedBy string      `json:"requestedBy"` // the client remote ip address
	Timestamp   time.Time   `json:"timestamp"`   // timestamp of decision
}

func (n DecisionResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("decision_id", n.ID),
		slog.Any("result", n.Result),
		slog.String("path", n.Path),
		slog.Any("input", n.Input),
		slog.String("requested_by", n.RequestedBy),
		slog.Time("timestamp", n.Timestamp))
}

type DecisionOptions struct {
	RemoteAddr string      // specifies client remote ip address
	Path       string      // specifies name of policy decision to evaluate (e.g., example/allow)
	Input      interface{} // specifies value of the input document to evaluate policy with
}

type DecisionUser struct {
	Key        string                 `validate:"required" json:"key"`
	Attributes map[string]interface{} `json:"attributes"`
}

type DecisionRequest struct {
	User       DecisionUser `validate:"required" json:"user"`
	Action     string       `validate:"required" json:"action"`
	Permission string       `validate:"required" json:"permission"`
	Path       string       `json:"path"`
}

type PolicyProjectUpdate struct {
	Available bool
	OldHash   string
	NewHash   string
}

type PolicyProject struct {
	Url           string
	Branch        string
	SSHKey        []byte
	Hash          string
	PolicyBundles []PolicyBundle
}

type PolicyBundle struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type PolicyUpdater struct {
	eventHandlerFunc func(context.Context, []PolicyBundle)
	project          PolicyProject
}
