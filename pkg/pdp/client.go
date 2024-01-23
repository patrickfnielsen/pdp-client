package pdp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

type PermitClient struct {
	logger         *decisionLogger
	queryCache     *queryCache
	store          storage.Store
	policiesLoaded bool
}

type PermitConfig struct {
	Logger DecisionLogConfig
}

func New(config *PermitConfig) (*PermitClient, error) {
	logger, err := newLogger(&config.Logger)
	if err != nil {
		return nil, err
	}

	permit := &PermitClient{
		logger:         logger,
		store:          inmem.New(),
		queryCache:     newQueryCache(),
		policiesLoaded: false,
	}

	permit.logger.Start()
	return permit, nil
}

func (p *PermitClient) Close(ctx context.Context) error {
	return p.logger.Stop(ctx)
}

func (p *PermitClient) Decision(ctx context.Context, options DecisionOptions) (*DecisionResult, error) {
	result, err := newDecisionResult()
	if err != nil {
		return nil, err
	}

	r, err := parseDataPath(options.Path)
	if err != nil {
		return nil, err
	}

	pq, err := p.queryCache.Get(r.String(), func(s string) (*rego.PreparedEvalQuery, error) {
		pq, err := rego.New(
			rego.Query(s),
			rego.Store(p.store),
		).PrepareForEval(ctx)
		if err != nil {
			return nil, err
		}

		return &pq, nil
	})
	if err != nil {
		return nil, err
	}

	ts := time.Now().UTC()
	rs, err := pq.Eval(
		ctx,
		rego.EvalTime(ts),
		rego.EvalInput(options.Input),
	)

	if err != nil {
		return nil, err
	} else if len(rs) == 0 {
		return nil, errors.New(fmt.Sprintf("%v decision was undefined", options.Path))
	}

	result.Timestamp = ts
	result.Result = rs[0].Expressions[0].Value
	result.Input = options.Input
	result.Path = options.Path
	result.RequestedBy = options.RemoteAddr

	err = errors.Join(err, p.logger.Log(*result))
	return result, err
}

func (p *PermitClient) Activate(ctx context.Context, path string, policyData string) error {
	txn, err := p.store.NewTransaction(ctx, storage.TransactionParams{Write: true})
	if err != nil {
		return err
	}

	err = p.store.UpsertPolicy(ctx, txn, path, []byte(policyData))
	if err != nil {
		return err
	}

	err = p.store.Commit(ctx, txn)
	if err != nil {
		return err
	}

	p.queryCache.Clear()
	p.policiesLoaded = true
	return err
}

func (p *PermitClient) Ready() bool {
	return p.policiesLoaded
}

func newDecisionResult() (*DecisionResult, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	result := &DecisionResult{ID: id.String()}
	return result, nil
}

func parseDataPath(s string) (ast.Ref, error) {
	s = "/" + strings.TrimPrefix(s, "/")

	path, ok := storage.ParsePath(s)
	if !ok {
		return nil, errors.New(fmt.Sprintf("invalid path: %s", s))
	}

	return path.Ref(ast.DefaultRootDocument), nil
}
