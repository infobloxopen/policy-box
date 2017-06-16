package pdp

import (
	"fmt"
	"strings"
)

func (ctx *YastCtx) unmarshalSelectorPathValueElement(v interface{}) (ExpressionType, error) {
	a, err := ctx.unmarshalValue(v)
	if err != nil {
		return nil, err
	}

	if a.DataType != DataTypeString {
		return nil, ctx.errorf("Expected only %s but got %s value",
			DataTypeNames[DataTypeString], DataTypeNames[a.DataType])
	}

	return a, nil
}

func (ctx *YastCtx) unmarshalSelectorPathAttributeElement(v interface{}) (ExpressionType, error) {
	ID, err := ctx.validateString(v, "attribute ID")
	if err != nil {
		return nil, err
	}

	a, ok := ctx.attrs[ID]
	if !ok {
		return nil, ctx.errorf("Unknown attribute ID %s", ID)
	}

	if a.DataType != DataTypeString &&
		a.DataType != DataTypeDomain &&
		a.DataType != DataTypeAddress &&
		a.DataType != DataTypeNetwork {
		return nil, ctx.errorf("Expected only %s or %s but got %s attribute %s",
			DataTypeNames[DataTypeString], DataTypeNames[DataTypeDomain], DataTypeNames[a.DataType], ID)
	}

	return AttributeDesignatorType{a}, nil
}

func (ctx *YastCtx) unmarshalSelectorPathSelectorElement(v interface{}) (ExpressionType, error) {
	s, err := ctx.unmarshalSelector(v)
	if err != nil {
		return nil, err
	}

	if s.DataType != DataTypeString &&
		s.DataType != DataTypeDomain &&
		s.DataType != DataTypeAddress &&
		s.DataType != DataTypeNetwork {
		return nil, ctx.errorf("Expected only %s or %s but got %s selector",
			DataTypeNames[DataTypeString], DataTypeNames[DataTypeDomain], DataTypeNames[s.DataType])
	}

	return s, nil
}

func (ctx *YastCtx) unmarshalSelectorPathStructuredElement(m map[interface{}]interface{}) (ExpressionType, error) {
	k, v, err := ctx.getSingleMapPair(m, "value or attribute map")
	if err != nil {
		return nil, err
	}

	s, err := ctx.validateString(k, "specificator")
	if err != nil {
		return nil, err
	}

	switch s {
	case yastTagValue:
		return ctx.unmarshalSelectorPathValueElement(v)

	case yastTagAttribute:
		return ctx.unmarshalSelectorPathAttributeElement(v)

	case yastTagSelector:
		return ctx.unmarshalSelectorPathSelectorElement(v)
	}

	return nil, ctx.errorf("Expected value, attribute or selector specificator but got %s", s)
}

func (ctx *YastCtx) unmarshalSelectorPathElement(v interface{}, i int) (string, ExpressionType, error) {
	ctx.pushNodeSpec("%d", i+1)
	defer ctx.popNodeSpec()

	s, err := ctx.validateString(v, "string, value, attribute or selector")
	if err != nil {
		m, err := ctx.validateMap(v, "string, value, attribute or selector")
		if err != nil {
			return "", nil, err
		}

		e, err := ctx.unmarshalSelectorPathStructuredElement(m)

		desc := ""
		if e != nil {
			desc = e.describe()
		}

		return desc, e, err
	}

	return fmt.Sprintf("%q", s), AttributeValueType{DataTypeString, s}, nil
}

func (ctx *YastCtx) unmarshalSelectorPath(m map[interface{}]interface{}) ([]string, []ExpressionType, error) {
	v, err := ctx.extractList(m, yastTagPath, "selector path")
	if err != nil {
		return nil, nil, err
	}

	ctx.pushNodeSpec(yastTagPath)
	defer ctx.popNodeSpec()

	path := make([]string, len(v))
	p := make([]ExpressionType, len(v))
	for i, item := range v {
		s, a, err := ctx.unmarshalSelectorPathElement(item, i)
		if err != nil {
			return nil, nil, err
		}

		path[i] = s
		p[i] = a
	}

	return path, p, nil
}

func (ctx *YastCtx) unmarshalSelectorContent(m map[interface{}]interface{}) (string, interface{}, error) {
	ID, err := ctx.extractString(m, yastTagContent, "selector content id")
	if err != nil {
		return "", nil, err
	}

	c, ok := ctx.includes[ID]
	if !ok {
		return ID, nil, ctx.errorf("No content with id %s", ID)
	}

	return ID, c, nil
}

func (ctx *YastCtx) contentIndexKey(cid string, path []string) string {
	if len(path) <= 1 {
		return fmt.Sprintf("%q", cid)
	} else {
		path = path[:len(path)-1]
		path = append([]string{fmt.Sprintf("%q", cid)}, path...)
	}
	return strings.Join(path, "/")
}

func (ctx *YastCtx) unmarshalSelector(v interface{}) (*SelectorType, error) {
	ctx.pushNodeSpec(yastTagSelector)
	defer ctx.popNodeSpec()

	m, err := ctx.validateMap(v, "selector attributes")
	if err != nil {
		return nil, err
	}

	strT, err := ctx.extractString(m, yastTagType, "type")
	if err != nil {
		return nil, err
	}

	t, ok := DataTypeIDs[strings.ToLower(strT)]
	if !ok {
		return nil, ctx.errorf("Unknown value type %#v", strT)
	}

	strPath, rawPath, err := ctx.unmarshalSelectorPath(m)
	if err != nil {
		return nil, err
	}

	selKey := strings.Join(strPath, "/")

	ID, rawCtx, err := ctx.unmarshalSelectorContent(m)
	if err != nil {
		return nil, err
	}

	if ctx.selectors == nil {
		ctx.selectors = make(map[string]map[string]*SelectorType)
	}

	pathMap, ok := ctx.selectors[ID]
	if !ok {
		pathMap = make(map[string]*SelectorType)
		ctx.selectors[ID] = pathMap
	}

	ckey := ctx.contentIndexKey(ID, strPath)
	ctx.addPolicyToContentIndex(ckey, m)

	sel, ok := pathMap[selKey]
	if ok {
		return sel, nil
	}

	c, p, dp := prepareSelectorContent(rawCtx, rawPath, t)

	sel = &SelectorType{t, p, c, ID, dp}
	pathMap[selKey] = sel

	return sel, nil
}
