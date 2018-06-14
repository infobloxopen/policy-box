package pdp

import (
	"encoding/binary"
	"math"
	"net"
	"reflect"

	"github.com/infobloxopen/go-trees/domain"
	"github.com/infobloxopen/go-trees/domaintree"
	"github.com/infobloxopen/go-trees/strtree"
)

const requestVersion = uint16(1)

const (
	requestWireTypeBooleanFalse = iota
	requestWireTypeBooleanTrue
	requestWireTypeString
	requestWireTypeInteger
	requestWireTypeFloat
	requestWireTypeIPv4Address
	requestWireTypeIPv6Address
	requestWireTypeIPv4Network
	requestWireTypeIPv6Network
	requestWireTypeDomain
	requestWireTypeSetOfStrings
	requestWireTypeSetOfDomains
	requestWireTypeSetOfNetworks
	requestWireTypeListOfStrings

	requestWireTypesTotal
)

var (
	requestWireTypeNames = []string{
		"boolean true",
		"boolean false",
		"string",
		"integer",
		"float",
		"IPv4 address",
		"IPv6 address",
		"IPv4 network",
		"IPv6 network",
		"domain",
		"set of strings",
		"set of domains",
		"set of networks",
		"list of strings",
	}

	builtinTypeByWire = []Type{
		TypeBoolean,
		TypeBoolean,
		TypeString,
		TypeInteger,
		TypeFloat,
		TypeAddress,
		TypeAddress,
		TypeNetwork,
		TypeNetwork,
		TypeDomain,
		TypeSetOfStrings,
		TypeSetOfDomains,
	}
)

// MarshalRequestAssignments marshals list of assignments to sequence of bytes.
// It requires each assignment to have immediate value as an expression (which
// can be created with MakeStringValue or similar functions). Caller should
// provide large enough buffer. Function fills the buffer and returns
// number of bytes written.
func MarshalRequestAssignments(b []byte, in []AttributeAssignment) (int, error) {
	off, err := putRequestVersion(b)
	if err != nil {
		return off, err
	}

	n, err := putAssignmentExpressions(b[off:], in)
	if err != nil {
		return off, err
	}

	return off + n, nil
}

// MarshalRequestReflection marshals set of attributes wrapped with
// reflect.Value to sequence of bytes. Caller should provide large enough
// buffer. Also caller put attribute count to marshal. For each attribute
// MarshalRequestReflection calls f function with index of the attribute.
// It expects the function to return attribute id, type and value.
// For TypeBoolean MarshalRequestReflection expects bool value, for TypeString
// - string, for TypeInteger - intX, uintX (internally converting to int64),
// TypeFloat - float32 or float64, TypeAddress - net.IP, TypeNetwork - net.IPNet
// or *net.IPNet, TypeDomain - string or domain.Name from
// github.com/infobloxopen/go-trees/domain package, TypeSetOfStrings -
// *strtree.Tree from github.com/infobloxopen/go-trees/strtree package.
// The function fills given buffer and returns number of bytes written,
// TypeSetOfDomains - *domaintree.Nodea from
// github.com/infobloxopen/go-trees/domaintree.
func MarshalRequestReflection(b []byte, c int, f func(i int) (string, Type, reflect.Value, error)) (int, error) {
	off, err := putRequestVersion(b)
	if err != nil {
		return off, err
	}

	n, err := putAttributesFromReflection(b[off:], c, f)
	if err != nil {
		return off, err
	}

	return off + n, nil
}

// UnmarshalRequestAssignments parses given sequence of bytes as a list of
// assignments. Caller should provide large enough out slice. The function
// returns number of assignments written.
func UnmarshalRequestAssignments(b []byte, out []AttributeAssignment) (int, error) {
	n, err := checkRequestVersion(b)
	if err != nil {
		return 0, err
	}

	return getAssignmentExpressions(b[n:], out)
}

// UnmarshalRequestReflection parses given sequence of bytes to set of reflected
// values. It calls f function for each attribute extracted from buffer with
// attribute id and type. The f function should return value to set.
// If it returns error UnmarshalRequestReflection stops parsing and exits with
// the error.
func UnmarshalRequestReflection(b []byte, f func(string, Type) (reflect.Value, error)) error {
	n, err := checkRequestVersion(b)
	if err != nil {
		return err
	}
	b = b[n:]

	return getAttributesToReflection(b, f)
}

func putRequestVersion(b []byte) (int, error) {
	if len(b) < 2 {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b, requestVersion)
	return 2, nil
}

func checkRequestVersion(b []byte) (int, error) {
	if len(b) < 2 {
		return 0, newRequestBufferUnderflowError()
	}

	if v := binary.LittleEndian.Uint16(b); v != requestVersion {
		return 0, newRequestVersionError(v, requestVersion)
	}

	return 2, nil
}

func putRequestAttributeCount(b []byte, n int) (int, error) {
	if n < 0 {
		return 0, newRequestInvalidAttributeCountError(n)
	}

	if n > math.MaxUint16 {
		return 0, newRequestTooManyAttributesError(n)
	}

	if len(b) < 2 {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b, uint16(n))
	return 2, nil
}

func getRequestAttributeCount(b []byte) (int, int, error) {
	if len(b) < 2 {
		return 0, 0, newRequestBufferUnderflowError()
	}

	return int(binary.LittleEndian.Uint16(b)), 2, nil
}

func putRequestAttribute(b []byte, name string, value AttributeValue) (int, error) {
	t := value.GetResultType()

	switch t {
	case TypeBoolean:
		v, _ := value.boolean()
		return putRequestAttributeBoolean(b, name, v)

	case TypeString:
		v, _ := value.str()
		return putRequestAttributeString(b, name, v)

	case TypeInteger:
		v, _ := value.integer()
		return putRequestAttributeInteger(b, name, v)

	case TypeFloat:
		v, _ := value.float()
		return putRequestAttributeFloat(b, name, v)

	case TypeAddress:
		v, _ := value.address()
		return putRequestAttributeAddress(b, name, v)

	case TypeNetwork:
		v, _ := value.network()
		return putRequestAttributeNetwork(b, name, v)

	case TypeDomain:
		v, _ := value.domain()
		return putRequestAttributeDomain(b, name, v)

	case TypeSetOfStrings:
		v, _ := value.setOfStrings()
		return putRequestAttributeSetOfStrings(b, name, v)

	case TypeSetOfDomains:
		v, _ := value.setOfDomains()
		return putRequestAttributeSetOfDomains(b, name, v)
	}

	return 0, newRequestAttributeMarshallingNotImplemented(t)
}

func getRequestAttribute(b []byte) (string, AttributeValue, int, error) {
	name, off, err := getRequestAttributeName(b)
	if err != nil {
		return "", UndefinedValue, 0, bindError(err, "name")
	}

	t, n, err := getRequestAttributeType(b[off:])
	if err != nil {
		return "", UndefinedValue, 0, bindError(bindError(err, "type"), name)
	}

	off += n

	switch t {
	case requestWireTypeSetOfNetworks, requestWireTypeListOfStrings:
		return "", UndefinedValue, 0, bindError(newRequestAttributeUnmarshallingNotImplemented(t), name)

	case requestWireTypeBooleanFalse:
		return name, MakeBooleanValue(false), off, nil

	case requestWireTypeBooleanTrue:
		return name, MakeBooleanValue(true), off, nil

	case requestWireTypeString:
		s, n, err := getRequestStringValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeStringValue(s), off + n, nil

	case requestWireTypeInteger:
		i, n, err := getRequestIntegerValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeIntegerValue(i), off + n, nil

	case requestWireTypeFloat:
		f, n, err := getRequestFloatValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeFloatValue(f), off + n, nil

	case requestWireTypeIPv4Address:
		a, n, err := getRequestIPv4AddressValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeAddressValue(a), off + n, nil

	case requestWireTypeIPv6Address:
		a, n, err := getRequestIPv6AddressValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeAddressValue(a), off + n, nil

	case requestWireTypeIPv4Network:
		a, n, err := getRequestIPv4NetworkValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeNetworkValue(a), off + n, nil

	case requestWireTypeIPv6Network:
		a, n, err := getRequestIPv6NetworkValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeNetworkValue(a), off + n, nil

	case requestWireTypeDomain:
		d, n, err := getRequestDomainValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeDomainValue(d), off + n, nil

	case requestWireTypeSetOfStrings:
		ss, n, err := getRequestSetOfStringsValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeSetOfStringsValue(ss), off + n, nil

	case requestWireTypeSetOfDomains:
		sd, n, err := getRequestSetOfDomainsValue(b[off:])
		if err != nil {
			return "", UndefinedValue, 0, bindError(bindError(err, "value"), name)
		}

		return name, MakeSetOfDomainsValue(sd), off + n, nil
	}

	return "", UndefinedValue, 0, bindError(newRequestAttributeUnmarshallingTypeError(t), name)
}

func putRequestAttributeName(b []byte, name string) (int, error) {
	if len(name) > math.MaxUint8 {
		return 0, newRequestTooLongAttributeNameError(name)
	}

	n := len(name) + 1
	if len(b) < n {
		return 0, newRequestBufferOverflowError()
	}

	b[0] = byte(len(name))
	copy(b[1:], []byte(name))

	return n, nil
}

func getRequestAttributeName(b []byte) (string, int, error) {
	if len(b) < 1 {
		return "", 0, newRequestBufferUnderflowError()
	}

	off := int(b[0]) + 1
	if len(b) < off {
		return "", 0, newRequestBufferUnderflowError()
	}

	return string(b[1:off]), off, nil
}

func putRequestAttributeType(b []byte, t int) (int, error) {
	if t < 0 || t >= requestWireTypesTotal {
		return 0, newRequestAttributeMarshallingTypeError(t)
	}

	if len(b) < 1 {
		return 0, newRequestBufferOverflowError()
	}

	b[0] = byte(t)
	return 1, nil
}

func getRequestAttributeType(b []byte) (int, int, error) {
	if len(b) < 1 {
		return 0, 0, newRequestBufferUnderflowError()
	}

	return int(b[0]), 1, nil
}

func putRequestAttributeBoolean(b []byte, name string, value bool) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestBooleanValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestBooleanValue(b []byte, value bool) (int, error) {
	if value {
		return putRequestAttributeType(b, requestWireTypeBooleanTrue)
	}

	return putRequestAttributeType(b, requestWireTypeBooleanFalse)
}

func putRequestAttributeString(b []byte, name string, value string) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestStringValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestStringValue(b []byte, value string) (int, error) {
	off, err := putRequestAttributeType(b, requestWireTypeString)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	if len(value) > math.MaxUint16 {
		return 0, newRequestTooLongStringValueError(value)
	}

	n := len(value) + 2
	if len(b) < n {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b, uint16(len(value)))
	copy(b[2:], value)

	return off + n, nil
}

func getRequestStringValue(b []byte) (string, int, error) {
	if len(b) < 2 {
		return "", 0, newRequestBufferUnderflowError()
	}

	off := int(binary.LittleEndian.Uint16(b)) + 2
	if len(b) < off {
		return "", 0, newRequestBufferUnderflowError()
	}

	return string(b[2:off]), off, nil
}

func putRequestAttributeInteger(b []byte, name string, value int64) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestIntegerValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestIntegerValue(b []byte, value int64) (int, error) {
	off, err := putRequestAttributeType(b, requestWireTypeInteger)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	if len(b) < 8 {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint64(b, uint64(value))
	return off + 8, nil
}

func getRequestIntegerValue(b []byte) (int64, int, error) {
	if len(b) < 8 {
		return 0, 0, newRequestBufferUnderflowError()
	}

	return int64(binary.LittleEndian.Uint64(b)), 8, nil
}

func putRequestAttributeFloat(b []byte, name string, value float64) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestFloatValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestFloatValue(b []byte, value float64) (int, error) {
	off, err := putRequestAttributeType(b, requestWireTypeFloat)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	if len(b) < 8 {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint64(b, math.Float64bits(value))
	return off + 8, nil
}

func getRequestFloatValue(b []byte) (float64, int, error) {
	if len(b) < 8 {
		return 0, 0, newRequestBufferUnderflowError()
	}

	return math.Float64frombits(binary.LittleEndian.Uint64(b)), 8, nil
}

func putRequestAttributeAddress(b []byte, name string, value net.IP) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestAddressValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestAddressValue(b []byte, value net.IP) (int, error) {
	t := requestWireTypeIPv4Address

	ip := value.To4()
	if ip == nil {
		t = requestWireTypeIPv6Address

		ip = value.To16()
		if ip == nil {
			return 0, newRequestAddressValueError(value)
		}
	}

	off, err := putRequestAttributeType(b, t)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	if len(b) < len(ip) {
		return 0, newRequestBufferOverflowError()
	}

	copy(b, ip)
	return off + len(ip), nil
}

func getRequestIPv4AddressValue(b []byte) (net.IP, int, error) {
	if len(b) < 4 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	return net.IPv4(b[0], b[1], b[2], b[3]), 4, nil
}

func getRequestIPv6AddressValue(b []byte) (net.IP, int, error) {
	if len(b) < 16 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	ip := net.IP(make([]byte, 16))
	copy(ip, b)
	return ip, 16, nil
}

func putRequestAttributeNetwork(b []byte, name string, value *net.IPNet) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestNetworkValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestNetworkValue(b []byte, value *net.IPNet) (int, error) {
	if value == nil {
		return 0, newRequestInvalidNetworkValueError(value)
	}

	ip := value.IP
	if len(ip) != 4 && len(ip) != 16 {
		return 0, newRequestInvalidNetworkValueError(value)
	}

	t := requestWireTypeIPv4Network
	ones, bits := value.Mask.Size()
	if bits != 32 {
		t = requestWireTypeIPv6Network
		if bits != 128 {
			return 0, newRequestInvalidNetworkValueError(value)
		}
	}

	off, err := putRequestAttributeType(b, t)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	if len(b) < len(ip)+1 {
		return 0, newRequestBufferOverflowError()
	}

	b[0] = byte(ones)

	copy(b[1:], ip)
	return off + len(ip) + 1, nil
}

func getRequestIPv4NetworkValue(b []byte) (*net.IPNet, int, error) {
	if len(b) < 5 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	mask := net.CIDRMask(int(b[0]), 32)
	if mask == nil {
		return nil, 0, newRequestIPv4InvalidMaskError(b[0])
	}

	return &net.IPNet{
		IP:   net.IPv4(b[1], b[2], b[3], b[4]).Mask(mask),
		Mask: mask,
	}, 5, nil
}

func getRequestIPv6NetworkValue(b []byte) (*net.IPNet, int, error) {
	if len(b) < 17 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	mask := net.CIDRMask(int(b[0]), 128)
	if mask == nil {
		return nil, 0, newRequestIPv6InvalidMaskError(b[0])
	}

	ip := net.IP(make([]byte, 16))
	copy(ip, b[1:])

	return &net.IPNet{
		IP:   ip.Mask(mask),
		Mask: mask,
	}, 17, nil
}

func putRequestAttributeDomain(b []byte, name string, value domain.Name) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestDomainValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestDomainValue(b []byte, value domain.Name) (int, error) {
	s := value.String()

	off, err := putRequestAttributeType(b, requestWireTypeDomain)
	if err != nil {
		return 0, err
	}

	b = b[off:]

	n := len(s) + 2
	if len(b) < n {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b, uint16(len(s)))
	copy(b[2:], []byte(s))

	return off + n, nil
}

func getRequestDomainValue(b []byte) (domain.Name, int, error) {
	s, n, err := getRequestStringValue(b)
	if err != nil {
		return domain.Name{}, 0, err
	}

	d, err := domain.MakeNameFromString(s)
	if err != nil {
		return domain.Name{}, 0, err
	}

	return d, n, nil
}

func putRequestAttributeSetOfStrings(b []byte, name string, value *strtree.Tree) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestSetOfStringsValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestSetOfStringsValue(b []byte, value *strtree.Tree) (int, error) {
	off, err := putRequestAttributeType(b, requestWireTypeSetOfStrings)
	if err != nil {
		return 0, err
	}

	ss := SortSetOfStrings(value)

	if len(ss) > math.MaxUint16 {
		return 0, newRequestTooLongSetOfStringsValueError(len(ss))
	}

	total := 2 * (len(ss) + 1)
	for i, s := range ss {
		if len(s) > math.MaxUint16 {
			return 0, bindErrorf(newRequestTooLongStringValueError(s), "%d", i+1)
		}

		total += len(s)
	}

	if len(b[off:]) < total {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b[off:], uint16(len(ss)))
	off += 2

	for _, s := range ss {
		binary.LittleEndian.PutUint16(b[off:], uint16(len(s)))
		off += 2

		copy(b[off:], s)
		off += len(s)
	}

	return off, nil
}

func getRequestSetOfStringsValue(b []byte) (*strtree.Tree, int, error) {
	if len(b) < 2 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	ss := strtree.NewTree()

	count := int(binary.LittleEndian.Uint16(b))
	off := 2

	for i := 0; i < count; i++ {
		s, n, err := getRequestStringValue(b[off:])
		if err != nil {
			return nil, 0, bindErrorf(err, "%d", i+1)
		}

		off += n

		ss.InplaceInsert(s, i)
	}

	return ss, off, nil
}

func putRequestAttributeSetOfDomains(b []byte, name string, value *domaintree.Node) (int, error) {
	off, err := putRequestAttributeName(b, name)
	if err != nil {
		return 0, err
	}

	n, err := putRequestSetOfDomainsValue(b[off:], value)
	if err != nil {
		return 0, err
	}

	return off + n, err
}

func putRequestSetOfDomainsValue(b []byte, value *domaintree.Node) (int, error) {
	off, err := putRequestAttributeType(b, requestWireTypeSetOfDomains)
	if err != nil {
		return 0, err
	}

	sd := SortSetOfDomains(value)

	if len(sd) > math.MaxUint16 {
		return 0, newRequestTooLongSetOfDomainsValueError(len(sd))
	}

	total := 2 * (len(sd) + 1)
	for _, s := range sd {
		total += len(s)
	}

	if len(b[off:]) < total {
		return 0, newRequestBufferOverflowError()
	}

	binary.LittleEndian.PutUint16(b[off:], uint16(len(sd)))
	off += 2

	for _, s := range sd {
		binary.LittleEndian.PutUint16(b[off:], uint16(len(s)))
		off += 2

		copy(b[off:], s)
		off += len(s)
	}

	return off, nil
}

func getRequestSetOfDomainsValue(b []byte) (*domaintree.Node, int, error) {
	if len(b) < 2 {
		return nil, 0, newRequestBufferUnderflowError()
	}

	sd := new(domaintree.Node)

	count := int(binary.LittleEndian.Uint16(b))
	off := 2

	for i := 0; i < count; i++ {
		d, n, err := getRequestDomainValue(b[off:])
		if err != nil {
			return nil, 0, bindErrorf(err, "%d", i+1)
		}

		off += n

		sd.InplaceInsert(d, i)
	}

	return sd, off, nil
}
