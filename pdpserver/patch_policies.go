package main

import (
	"fmt"

	"github.com/infobloxopen/themis/pdp"

	"gopkg.in/yaml.v2"
)

const (
	PatchOpAdd    = "ADD"
	PatchOpAppend = "APPEND"
	PatchOpDelete = "DELETE"
)

type PatchItem struct {
	Op     string
	Path   []string
	Entity interface{}

	pathIndex int
}

func (pi *PatchItem) getID() string {
	return pi.Path[pi.pathIndex]
}

func (pi *PatchItem) nextID() bool {
	if len(pi.Path) == pi.pathIndex+1 {
		return false
	}

	pi.pathIndex++
	return true
}

type policiesPatchCtx struct {
	// Current item in the original tree.
	ocur interface{}
	// Current item in the patched tree.
	cur interface{}
	// Parent (previous) item in the patched tree.
	prev interface{}

	pi *PatchItem
}

func copyPoliciesItem(item interface{}, parent interface{}, i int) interface{} {
	switch item := item.(type) {
	case *pdp.PolicySetType:
		cpy := *item
		cpy.Policies = make([]pdp.EvaluableType, len(item.Policies), len(item.Policies))
		copy(cpy.Policies, item.Policies)
		if parent != nil {
			ppset := parent.(*pdp.PolicySetType)
			ppset.Policies[i] = &cpy
		}
		return &cpy
	case *pdp.PolicyType:
		cpy := *item
		cpy.Rules = make([]*pdp.RuleType, len(item.Rules), len(item.Rules))
		copy(cpy.Rules, item.Rules)
		if parent != nil {
			ppset := parent.(*pdp.PolicySetType)
			ppset.Policies[i] = &cpy
		}
		return &cpy
	case *pdp.RuleType:
		cpy := *item
		if parent != nil {
			pp := parent.(*pdp.PolicyType)
			pp.Rules[i] = &cpy
		}
		return &cpy
	default:
		panic(fmt.Sprintf("Unsupported policies item type '%T'", item))
	}
}

func findPoliciesItem(id string, parent interface{}) (bool, int, interface{}) {
	switch parent := parent.(type) {
	case *pdp.PolicySetType:
		for i, e := range parent.Policies {
			if e.GetID() == id {
				return true, i, e
			}
		}
	case *pdp.PolicyType:
		for i, r := range parent.Rules {
			if r.ID == id {
				return true, i, r
			}
		}
	default:
		panic(fmt.Sprintf("Unsupported policies item type '%T'", parent))
	}

	return false, -1, nil
}

func (s *Server) applyPoliciesPatchItem(ctx *policiesPatchCtx) error {
	pi := ctx.pi
	id := pi.getID()

	if pi.nextID() {
		var (
			onext, next interface{}
			ok          bool
			i           int
		)

		nid := pi.getID()

		if ok, _, onext = findPoliciesItem(nid, ctx.ocur); !ok {
			return fmt.Errorf("Cannot find '%v' next item in original content", pi.Path[:pi.pathIndex])
		}

		if ok, i, next = findPoliciesItem(nid, ctx.cur); !ok {
			return fmt.Errorf("Cannot find '%v' next item in patched content", pi.Path[:pi.pathIndex])
		}

		if onext == next {
			next = copyPoliciesItem(next, ctx.cur, i)
		}

		ctx.ocur = onext
		ctx.prev = ctx.cur
		ctx.cur = next

		return s.applyPoliciesPatchItem(ctx)
	}

	switch pi.Op {
	case PatchOpAppend:
		switch item := ctx.cur.(type) {
		case *pdp.PolicySetType:
			e, err := s.ctx.UnmarshalEvaluable(pi.Entity)
			if err != nil {
				return err
			}

			item.Policies = append(item.Policies, e)
		case *pdp.PolicyType:
			r, err := s.ctx.UnmarshalRule(pi.Entity)
			if err != nil {
				return err
			}

			item.Rules = append(item.Rules, r)
		default:
			return fmt.Errorf("Operation '%s' is unsupported for type '%T'", pi.Op, item)
		}
	case PatchOpDelete:
		switch item := ctx.cur.(type) {
		case *pdp.PolicySetType:
			ppset := ctx.prev.(*pdp.PolicySetType)

			ok, i, _ := findPoliciesItem(id, ppset)
			if !ok {
				return fmt.Errorf("Cannot delete '%v' policy set from patched policy set. Policy set does not exist", pi.Path)
			}

			ppset.Policies = append(ppset.Policies[:i], ppset.Policies[i+1:]...)
		case *pdp.PolicyType:
			ppset := ctx.prev.(*pdp.PolicySetType)

			ok, i, _ := findPoliciesItem(id, ppset)
			if !ok {
				return fmt.Errorf("Cannot delete '%v' policy from patched policy set. Policy does not exist", pi.Path)
			}

			ppset.Policies = append(ppset.Policies[:i], ppset.Policies[i+1:]...)
		case *pdp.RuleType:
			pp := ctx.prev.(*pdp.PolicyType)

			ok, i, _ := findPoliciesItem(id, pp)
			if !ok {
				return fmt.Errorf("Cannot delete '%v' rule from patched policy. Rule does not exist", pi.Path)
			}

			pp.Rules = append(pp.Rules[:i], pp.Rules[i+1:]...)
		default:
			return fmt.Errorf("Operation '%s' is unsupported for type '%T'", pi.Op, item)
		}
	default:
		return fmt.Errorf("Unsupported '%s' patch operation for policies", pi.Op)
	}

	return nil
}

func (s *Server) copyAndPatchPolicies(data []byte, content map[string]interface{}) (pdp.EvaluableType, error) {
	var patches []PatchItem
	if err := yaml.Unmarshal(data, &patches); err != nil {
		return nil, err
	}

	cpyPolicy := copyPoliciesItem(s.Policy, nil, 0)
	for _, pi := range patches {
		ctx := &policiesPatchCtx{
			ocur: s.Policy,
			cur:  cpyPolicy,
			prev: nil,
			pi:   &pi,
		}

		if err := s.applyPoliciesPatchItem(ctx); err != nil {
			return nil, err
		}
	}

	return cpyPolicy.(pdp.EvaluableType), nil
}
