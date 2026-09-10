package engine

import (
	"school_district_reading/internal/policy"
	"school_district_reading/internal/retrieve"
	"school_district_reading/internal/store"
)

// WithStore returns a new Engine that ranks against st while copying explainer,
// version, and LLM flags. Audit is left nil so session snapshots never write
// the optional disk log (CLI engine.New still wires Audit from the environment).
func (e *Engine) WithStore(st *store.Store) *Engine {
	if e == nil {
		return New(st)
	}
	return &Engine{
		Store:    st,
		Retrieve: retrieve.New(st),
		Policy:   policy.Filter{Store: st},
		Explain:  e.Explain,
		Audit:    nil,
		LLMOn:    e.LLMOn,
		Version:  e.Version,
	}
}
