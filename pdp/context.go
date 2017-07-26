package pdp

import (
	"net"

	"github.com/infobloxopen/go-trees/domaintree"
	"github.com/infobloxopen/go-trees/iptree"
	"github.com/infobloxopen/go-trees/strtree"
)

const (
	EffectDeny = iota
	EffectPermit

	EffectNotApplicable

	EffectIndeterminate
	EffectIndeterminateD
	EffectIndeterminateP
	EffectIndeterminateDP
)

var (
	effectNames = []string{
		"Deny",
		"Permit",
		"NotApplicable",
		"Indeterminate",
		"Indeterminate{D}",
		"Indeterminate{P}",
		"Indeterminate{DP}"}

	EffectIDs = map[string]int{
		"deny":   EffectDeny,
		"permit": EffectPermit}
)

type Context struct {
	a map[string]map[int]AttributeValue
	c *strtree.Tree
}

func (c *Context) getAttribute(a Attribute) (AttributeValue, error) {
	t, ok := c.a[a.id]
	if !ok {
		return AttributeValue{}, a.newMissingError()
	}

	v, ok := t[a.t]
	if !ok {
		return AttributeValue{}, a.newMissingError()
	}

	return v, nil
}

func (c *Context) calculateBooleanExpression(e Expression) (bool, error) {
	v, err := e.calculate(c)
	if err != nil {
		return false, err
	}

	return v.boolean()
}

func (c *Context) calculateStringExpression(e Expression) (string, error) {
	v, err := e.calculate(c)
	if err != nil {
		return "", err
	}

	return v.str()
}

func (c *Context) calculateAddressExpression(e Expression) (net.IP, error) {
	v, err := e.calculate(c)
	if err != nil {
		return nil, err
	}

	return v.address()
}

func (c *Context) calculateDomainExpression(e Expression) (string, error) {
	v, err := e.calculate(c)
	if err != nil {
		return "", err
	}

	return v.domain()
}

func (c *Context) calculateNetworkExpression(e Expression) (*net.IPNet, error) {
	v, err := e.calculate(c)
	if err != nil {
		return nil, err
	}

	return v.network()
}

func (c *Context) calculateSetOfStringsExpression(e Expression) (*strtree.Tree, error) {
	v, err := e.calculate(c)
	if err != nil {
		return nil, err
	}

	return v.setOfStrings()
}

func (c *Context) calculateSetOfNetworksExpression(e Expression) (*iptree.Tree, error) {
	v, err := e.calculate(c)
	if err != nil {
		return nil, err
	}

	return v.setOfNetworks()
}

func (c *Context) calculateSetOfDomainsExpression(e Expression) (*domaintree.Node, error) {
	v, err := e.calculate(c)
	if err != nil {
		return nil, err
	}

	return v.setOfDomains()
}

type Response struct {
	Effect      int
	status      boundError
	obligations []AttributeAssignmentExpression
}

type Evaluable interface {
	GetID() (string, bool)
	Calculate(ctx *Context) Response
}
