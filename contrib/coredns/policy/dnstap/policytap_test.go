package dnstap

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/dnstap/taprw"
	dtest "github.com/coredns/coredns/plugin/dnstap/test"
	"github.com/coredns/coredns/plugin/test"
	tap "github.com/dnstap/golang-dnstap"
	"github.com/golang/protobuf/proto"
	pb "github.com/infobloxopen/themis/pdp-service"
	"github.com/miekg/dns"
)

type testIORoutine struct {
	dnstapChan chan tap.Dnstap
}

func newIORoutine(timeout time.Duration) testIORoutine {
	ch := make(chan tap.Dnstap, 1)
	tapIO := testIORoutine{dnstapChan: ch}
	// close channel by timeout to prevent checker from waiting forever
	go func() {
		time.Sleep(timeout)
		close(ch)
	}()
	return tapIO
}

func (tapIO testIORoutine) Dnstap(msg tap.Dnstap) {
	tapIO.dnstapChan <- msg
}

func TestSendCRExtraNoMsg(t *testing.T) {
	trapper := dtest.TrapTapper{Full: true}
	tapRW := taprw.ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &test.ResponseWriter{},
		Tapper:         &trapper,
	}
	proxyRW := NewProxyWriter(&tapRW)

	io := newIORoutine(100 * time.Millisecond)
	tapIO := NewPolicyDnstapSender(io)
	tapIO.SendCRExtraMsg(proxyRW, nil)
	_, ok := <-io.dnstapChan
	if ok {
		t.Errorf("Unexpected msg received")
		return
	}
}

func TestSendCRExtraInvalidMsg(t *testing.T) {
	msg := dns.Msg{}
	msg.SetQuestion("test.com.", dns.TypeA)
	msg.Answer = []dns.RR{
		test.A("test.com.		600	IN	A			10.240.0.1"),
	}
	msg.Rcode = -1

	trapper := dtest.TrapTapper{Full: true}
	tapRW := taprw.ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &test.ResponseWriter{},
		Tapper:         &trapper,
	}
	proxyRW := NewProxyWriter(&tapRW)
	proxyRW.WriteMsg(&msg)

	io := newIORoutine(100 * time.Millisecond)
	tapIO := NewPolicyDnstapSender(io)
	tapIO.SendCRExtraMsg(proxyRW, nil)
	_, ok := <-io.dnstapChan
	if ok {
		t.Errorf("Unexpected msg received")
		return
	}
}

func TestSendCRExtraMsg(t *testing.T) {
	msg := dns.Msg{}
	msg.SetQuestion("test.com.", dns.TypeA)
	msg.Answer = []dns.RR{
		test.A("test.com.		600	IN	A			10.240.0.1"),
	}

	trapper := dtest.TrapTapper{Full: true}
	tapRW := taprw.ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &test.ResponseWriter{},
		Tapper:         &trapper,
	}
	proxyRW := NewProxyWriter(&tapRW)
	proxyRW.WriteMsg(&msg)

	attrs := []*pb.Attribute{
		{Id: "attr1", Type: "address", Value: "10.240.0.1"},
		{Id: "attr2", Type: "string", Value: "value2"},
	}

	io := newIORoutine(5000 * time.Millisecond)
	tapIO := NewPolicyDnstapSender(io)
	tapIO.SendCRExtraMsg(proxyRW, attrs)
	checkCRExtraResult(t, io, proxyRW, attrs)

	if l := len(trapper.Trap); l != 0 {
		t.Errorf("Dnstap unexpectedly sent %d messages", l)
		return
	}
}

func checkCRExtraResult(t *testing.T, io testIORoutine, proxyRW *ProxyWriter, attrs []*pb.Attribute) {
	dnstapMsg, ok := <-io.dnstapChan
	if !ok {
		t.Errorf("Receiving Dnstap message was timed out")
		return
	}
	extra := &Extra{}
	err := proto.Unmarshal(dnstapMsg.Extra, extra)
	if err != nil {
		t.Errorf("Failed to unmarshal Extra (%v)", err)
		return
	}

	checkExtraAttrs(t, extra.GetAttrs(), attrs)
	checkCRMessage(t, dnstapMsg.Message, proxyRW)
}

func checkExtraAttrs(t *testing.T, actual []*DnstapAttribute, expected []*pb.Attribute) {
	if len(actual) != len(expected) {
		t.Errorf("Expected %d attributes, found %d", len(expected), len(actual))
		return
	}

checkAttr:
	for _, a := range actual {
		for _, e := range expected {
			if e.GetId() == a.GetId() {
				if string(a.GetValue()) != e.GetValue() {
					t.Errorf("Attribute %s: expected %v , found %v", e.GetId(), e, a)
					return
				}
				continue checkAttr
			}
		}
		t.Errorf("Unexpected attribute found %v", a)
	}
}

func checkCRMessage(t *testing.T, msg *tap.Message, proxyRW *ProxyWriter) {
	if msg == nil {
		t.Errorf("CR message not found")
		return
	}

	d := dtest.TestingData()
	bin, err := proxyRW.msg.Pack()
	if err != nil {
		t.Errorf("Failed to pack message (%v)", err)
		return
	}
	d.Packed = bin
	expMsg := d.ToClientResponse()
	if !dtest.MsgEqual(expMsg, msg) {
		t.Errorf("Unexpected message: expected: %v\nactual: %v", expMsg, msg)
	}
}

func TestProxyWriter(t *testing.T) {
	trapper := dtest.TrapTapper{Full: true}
	//	tapRW := taprw.ResponseWriter{
	//		Query:          new(dns.Msg),
	//		ResponseWriter: &test.ResponseWriter{},
	//		Tapper:         &trapper,
	//	}
	proxyRW := NewProxyWriter(&taprw.ResponseWriter{
		Query:          new(dns.Msg),
		ResponseWriter: &test.ResponseWriter{},
		Tapper:         &trapper,
	})

	if proxyRW == nil {
		t.Error("Failed to create ProxyWriter")
	}
	send := proxyRW.ResponseWriter.Send
	if send == nil || send.Cq || send.Cr {
		t.Error("Failed to turn off standard CQ or CR dnstap message")
	}

	proxyRW = NewProxyWriter(&test.ResponseWriter{})
	if proxyRW != nil {
		t.Error("ProxyWriter was unexpextedly created")
	}
}
