package pkg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchemaMakeSingleHandler(t *testing.T) {
	p := &Endpoint{
		Args: []string{
			pipTypeBoolean,
			pipTypeInteger,
			pipTypeNetwork,
		},
		goArgs: []string{
			goTypeBool,
			goTypeInt64,
			goTypeNetIPNet,
		},
		goArgPkgs: goPkgNetMask,

		Result:       pipTypeFloat,
		goResult:     goTypeFloat64,
		goResultZero: "0",
		goResultPkg:  0,
	}
	s := &Schema{
		Package: "test",
		Endpoints: map[string]*Endpoint{
			"*": p,
		},
	}

	h, err := s.makeSingleHandler(p)
	assert.NoError(t, err)

	assert.Equal(t, "test", h.Package)
	assert.Contains(t, h.Imports, goPkgNetName)
	assert.Equal(t, h.ArgCount, 3)

	assert.Contains(t, h.Types, goTypeBool)
	assert.Contains(t, h.Types, goTypeInt64)
	assert.Contains(t, h.Types, goTypeNetIPNet)

	assert.Equal(t, "v0, v1, v2", h.Args)
	assert.NotZero(t, h.ArgParsers)
	assert.Equal(t, goTypeFloat64, h.ResultType)
	assert.Equal(t, "0", h.ResultZero)
}

func TestSchemaMakeSingleHandlerWithUnknownTypes(t *testing.T) {
	p := &Endpoint{
		Args:   []string{"unknown"},
		Result: "unknown",
	}
	s := &Schema{
		Package: "test",
		Endpoints: map[string]*Endpoint{
			"*": p,
		},
	}

	h, err := s.makeSingleHandler(p)
	assert.EqualError(t, err, "argument 0: unknown type \"unknown\"", "handler: %#v", h)

	p.Args[0] = "boolean"
	h, err = s.makeSingleHandler(p)
	assert.EqualError(t, err, "result: unknown type \"unknown\"", "handler: %#v", h)
}

func TestSingleHandlerExecute(t *testing.T) {
	h := singleHandler{
		Package:    "test",
		Imports:    strings.Join(singleHandlerImports, "\n\t"),
		ArgCount:   0,
		Types:      "",
		Args:       "",
		ArgParsers: "",
		ResultType: goTypeBool,
		ResultZero: "false",
		Marshaller: pdpMarshallerBoolean,
	}

	b := new(bytes.Buffer)
	err := h.execute(b)
	assert.NoError(t, err)
	assert.Equal(t, testSingleHandlerSource, b.String())
}

const testSingleHandlerSource = `// Package test is a generated PIP server handler package. DO NOT EDIT.
package test

import (
	"encoding/binary"
	"errors"
	"github.com/infobloxopen/themis/pdp"
	"github.com/infobloxopen/themis/pip/server"
)

// Handler is a customized PIP handler for given input and output.
type Handler func() (bool, error)

const (
	reqIdSize         = 4
	reqVersionSize    = 2
	reqVersion        = uint16(1)
	reqArgs           = uint16(0)
	reqBigCounterSize = 2
	reqTypeSize       = 1
)

var (
	errFragment          = errors.New("fragment")
	errInvalidReqVersion = errors.New("invalid request version")
	errInvalidArgCount   = errors.New("invalid count of request arguments")
)

// WrapHandler converts custom Handler to generic PIP ServiceHandler.
func WrapHandler(f Handler) server.ServiceHandler {
	return func(b []byte) []byte {
		if len(b) < reqIdSize {
			panic("missing request id")
		}
		in := b[reqIdSize:]

		r, err := handler(in, f)
		if err != nil {
			n, err := pdp.MarshalInfoError(in[:cap(in)], err)
			if err != nil {
				panic(err)
			}

			return b[:reqIdSize+n]
		}

		n, err := pdp.MarshalInfoResponseBoolean(in[:cap(in)], r)
		if err != nil {
			panic(err)
		}

		return b[:reqIdSize+n]
	}
}

func handler(in []byte, f Handler) (bool, error) {
	if len(in) < reqVersionSize+reqBigCounterSize {
		return false, errFragment
	}

	if v := binary.LittleEndian.Uint16(in); v != reqVersion {
		return false, errInvalidReqVersion
	}
	in = in[reqVersionSize:]

	if c := binary.LittleEndian.Uint16(in); c != reqArgs {
		return false, errInvalidArgCount
	}
	in = in[reqBigCounterSize:]

	return f()
}
`
