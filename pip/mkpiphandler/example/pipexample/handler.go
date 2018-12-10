// pipexample is a generated PIP server handler package. DO NOT EDIT.
package pipexample

import (
	"encoding/binary"
	"errors"
	"github.com/infobloxopen/themis/pdp"
	"github.com/infobloxopen/themis/pip/server"
	"net"
)

type Handler func(string, net.IP) (*net.IPNet, error)

const (
	reqIdSize         = 4
	reqVersionSize    = 2
	reqVersion        = uint16(1)
	reqArgs           = uint16(2)
	reqBigCounterSize = 2
	reqTypeSize       = 1
)

var (
	errFragment          = errors.New("fragment")
	errInvalidReqVersion = errors.New("invalid request version")
	errInvalidArgCount   = errors.New("invalid count of request arguments")
)

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

		n, err := pdp.MarshalInfoResponseNetwork(in[:cap(in)], r)
		if err != nil {
			panic(err)
		}

		return b[:reqIdSize+n]
	}
}

func handler(in []byte, f Handler) (*net.IPNet, error) {
	if len(in) < reqVersionSize+reqBigCounterSize {
		return nil, errFragment
	}

	if v := binary.LittleEndian.Uint16(in); v != reqVersion {
		return nil, errInvalidReqVersion
	}
	in = in[reqVersionSize:]

	if c := binary.LittleEndian.Uint16(in); c != reqArgs {
		return nil, errInvalidArgCount
	}
	in = in[reqBigCounterSize:]

	v0, in, err := pdp.GetInfoRequestStringValue(in)
	if err != nil {
		return nil, err
	}

	v1, in, err := pdp.GetInfoRequestAddressValue(in)
	if err != nil {
		return nil, err
	}

	return f(v0, v1)
}
