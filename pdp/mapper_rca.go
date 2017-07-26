package pdp

import (
	"fmt"

	"github.com/infobloxopen/go-trees/strtree"
)

type mapperRCA struct {
	argument  Expression
	rules     *strtree.Tree
	def       *Rule
	err       *Rule
	algorithm ruleCombiningAlg
}

type MapperRCAParams struct {
	Argument  Expression
	DefOk     bool
	Def       string
	ErrOk     bool
	Err       string
	Algorithm ruleCombiningAlg
}

func getSetOfIDs(v AttributeValue) ([]string, error) {
	ID, err := v.str()
	if err == nil {
		return []string{ID}, nil
	}

	setIDs, err := v.setOfStrings()
	if err == nil {
		return sortSetOfStrings(setIDs), nil
	}

	listIDs, err := v.listOfStrings()
	if err == nil {
		return listIDs, nil
	}

	return nil, newMapperArgumentTypeError(v.t)
}

func collectSubRules(IDs []string, m *strtree.Tree) []*Rule {
	rules := []*Rule{}
	for _, ID := range IDs {
		rule, ok := m.Get(ID)
		if ok {
			rules = append(rules, rule.(*Rule))
		}
	}

	return rules
}

func makeMapperRCA(rules []*Rule, params interface{}) ruleCombiningAlg {
	mapperParams, ok := params.(MapperRCAParams)
	if !ok {
		panic(fmt.Errorf("Mapper rule combining algorithm maker expected MapperRCAParams structure as params "+
			"but got %T", params))
	}

	var (
		m   *strtree.Tree
		def *Rule
		err *Rule
	)

	if rules != nil {
		m = strtree.NewTree()
		count := 0
		for _, r := range rules {
			if !r.hidden {
				m.InplaceInsert(r.id, r)
				count++
			}
		}

		if count > 0 {
			if mapperParams.DefOk {
				if v, ok := m.Get(mapperParams.Def); ok {
					def = v.(*Rule)
				}
			}

			if mapperParams.ErrOk {
				if v, ok := m.Get(mapperParams.Err); ok {
					err = v.(*Rule)
				}
			}
		} else {
			m = nil
		}
	}

	return mapperRCA{
		argument:  mapperParams.Argument,
		rules:     m,
		def:       def,
		err:       err,
		algorithm: mapperParams.Algorithm}
}

func (a mapperRCA) describe() string {
	return "mapper"
}

func (a mapperRCA) calculateErrorRule(ctx *Context, err error) Response {
	if a.err != nil {
		return a.err.calculate(ctx)
	}

	return Response{EffectIndeterminate, bindError(err, a.describe()), nil}
}

func (a mapperRCA) getRulesMap(rules []*Rule) *strtree.Tree {
	if a.rules != nil {
		return a.rules
	}

	m := strtree.NewTree()
	count := 0
	for _, rule := range rules {
		if !rule.hidden {
			m.InplaceInsert(rule.id, rule)
			count++
		}
	}

	if count > 0 {
		return m
	}

	return nil
}

func (a mapperRCA) execute(rules []*Rule, ctx *Context) Response {
	v, err := a.argument.calculate(ctx)
	if err != nil {
		switch err.(type) {
		case *missingValueError:
			if a.def != nil {
				return a.def.calculate(ctx)
			}
		}

		return a.calculateErrorRule(ctx, err)
	}

	if a.algorithm != nil {
		IDs, err := getSetOfIDs(v)
		if err != nil {
			return a.calculateErrorRule(ctx, err)
		}

		r := a.algorithm.execute(collectSubRules(IDs, a.getRulesMap(rules)), ctx)
		if r.Effect == EffectNotApplicable && a.def != nil {
			return a.def.calculate(ctx)
		}

		return r
	}

	ID, err := v.str()
	if err != nil {
		return a.calculateErrorRule(ctx, err)
	}

	if a.rules != nil {
		rule, ok := a.rules.Get(ID)
		if ok {
			return rule.(*Rule).calculate(ctx)
		}
	} else {
		for _, rule := range rules {
			if rule.id == ID {
				return rule.calculate(ctx)
			}
		}
	}

	if a.def != nil {
		return a.def.calculate(ctx)
	}

	return Response{EffectNotApplicable, nil, nil}
}
