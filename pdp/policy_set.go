package pdp

import "fmt"

type policyCombiningAlg interface {
	execute(rules []Evaluable, ctx *Context) Response
}

type PolicyCombiningAlgMaker func(policies []Evaluable, params interface{}) policyCombiningAlg

var (
	firstApplicableEffectPCAInstance = firstApplicableEffectPCA{}
	denyOverridesPCAInstance         = denyOverridesPCA{}

	PolicyCombiningAlgs = map[string]PolicyCombiningAlgMaker{
		"firstapplicableeffect": makeFirstApplicableEffectPCA,
		"denyoverrides":         makeDenyOverridesPCA}

	PolicyCombiningParamAlgs = map[string]PolicyCombiningAlgMaker{
		"mapper": makeMapperPCA}
)

type PolicySet struct {
	id          string
	hidden      bool
	target      Target
	policies    []Evaluable
	obligations []AttributeAssignmentExpression
	algorithm   policyCombiningAlg
}

func NewPolicySet(ID string, hidden bool, target Target, policies []Evaluable, makePCA PolicyCombiningAlgMaker, params interface{}, obligations []AttributeAssignmentExpression) *PolicySet {
	return &PolicySet{
		id:          ID,
		hidden:      hidden,
		target:      target,
		policies:    policies,
		obligations: obligations,
		algorithm:   makePCA(policies, params)}
}

func (p *PolicySet) describe() string {
	if pid, ok := p.GetID(); ok {
		return fmt.Sprintf("policy set %q", pid)
	}

	return "hidden policy set"
}

func (p *PolicySet) GetID() (string, bool) {
	return p.id, !p.hidden
}

func (p *PolicySet) Calculate(ctx *Context) Response {
	match, err := p.target.calculate(ctx)
	if err != nil {
		r := combineEffectAndStatus(err, p.algorithm.execute(p.policies, ctx))
		if r.status != nil {
			r.status = bindError(err, p.describe())
		}
		return r
	}

	if !match {
		return Response{EffectNotApplicable, nil, nil}
	}

	r := p.algorithm.execute(p.policies, ctx)
	if r.Effect == EffectDeny || r.Effect == EffectPermit {
		r.obligations = append(r.obligations, p.obligations...)
	}

	if r.status != nil {
		r.status = bindError(r.status, p.describe())
	}

	return r
}

type firstApplicableEffectPCA struct {
}

func makeFirstApplicableEffectPCA(policies []Evaluable, params interface{}) policyCombiningAlg {
	return firstApplicableEffectPCAInstance
}

func (a firstApplicableEffectPCA) execute(policies []Evaluable, ctx *Context) Response {
	for _, p := range policies {
		r := p.Calculate(ctx)
		if r.Effect != EffectNotApplicable {
			return r
		}
	}

	return Response{EffectNotApplicable, nil, nil}
}

type denyOverridesPCA struct {
}

func makeDenyOverridesPCA(policies []Evaluable, params interface{}) policyCombiningAlg {
	return denyOverridesPCAInstance
}

func (a denyOverridesPCA) describe() string {
	return "deny overrides"
}

func (a denyOverridesPCA) execute(policies []Evaluable, ctx *Context) Response {
	errs := []error{}
	obligations := make([]AttributeAssignmentExpression, 0)

	indetD := 0
	indetP := 0
	indetDP := 0

	permits := 0

	for _, p := range policies {
		r := p.Calculate(ctx)
		if r.Effect == EffectDeny {
			return r
		}

		if r.Effect == EffectPermit {
			permits++
			obligations = append(obligations, r.obligations...)
			continue
		}

		if r.Effect == EffectNotApplicable {
			continue
		}

		if r.Effect == EffectIndeterminateD {
			indetD++
		} else {
			if r.Effect == EffectIndeterminateP {
				indetP++
			} else {
				indetDP++
			}

		}

		errs = append(errs, r.status)
	}

	var err boundError
	if len(errs) > 1 {
		err = bindError(newMultiError(errs), a.describe())
	} else if len(errs) > 0 {
		err = bindError(errs[0], a.describe())
	}

	if indetDP > 0 || (indetD > 0 && (indetP > 0 || permits > 0)) {
		return Response{EffectIndeterminateDP, err, nil}
	}

	if indetD > 0 {
		return Response{EffectIndeterminateD, err, nil}
	}

	if permits > 0 {
		return Response{EffectPermit, nil, obligations}
	}

	if indetP > 0 {
		return Response{EffectIndeterminateP, err, nil}
	}

	return Response{EffectNotApplicable, nil, nil}
}
