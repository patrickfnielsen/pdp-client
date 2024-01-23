package pdp

import (
	"errors"
	"sync"

	"github.com/open-policy-agent/opa/rego"
)

type queryCache struct {
	sync.Mutex
	cache map[string]*rego.PreparedEvalQuery
}

func newQueryCache() *queryCache {
	return &queryCache{cache: map[string]*rego.PreparedEvalQuery{}}
}

func (qc *queryCache) Get(key string, orElse func(string) (*rego.PreparedEvalQuery, error)) (*rego.PreparedEvalQuery, error) {
	qc.Lock()
	defer qc.Unlock()

	result, ok := qc.cache[key]
	if ok {
		return result, nil
	}

	result, err := orElse(key)
	if err == nil {
		qc.cache[key] = result
		return result, nil
	}

	return nil, errors.New("failed to get prepared query from cache")
}

func (qc *queryCache) Set(key string, pq *rego.PreparedEvalQuery) {
	qc.Lock()
	defer qc.Unlock()

	qc.cache[key] = pq
}

func (qc *queryCache) Clear() {
	qc.Lock()
	defer qc.Unlock()

	qc.cache = make(map[string]*rego.PreparedEvalQuery)
}
