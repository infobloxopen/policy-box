package pep

import (
	"fmt"
	"strings"
	"testing"

	"github.com/infobloxopen/themis/pdp"
	"github.com/infobloxopen/themis/pdpserver/server"
)

const allPermitPolicy = `# Policy for client tests
attributes:
  x: string

policies:
  alg: FirstApplicableEffect
  rules:
  - effect: Permit
    obligations:
    - x:
       val:
         type: string
         content: AllPermitRule
`

func TestUnaryClientValidation(t *testing.T) {
	pdpServer := startTestPDPServer(allPermitPolicy, 5555, t)
	defer func() {
		if logs := pdpServer.Stop(); len(logs) > 0 {
			t.Logf("server logs:\n%s", logs)
		}
	}()

	c := NewClient()
	err := c.Connect("127.0.0.1:5555")
	if err != nil {
		t.Fatalf("expected no error but got %s", err)
	}
	defer c.Close()

	in := decisionRequest{
		Direction: "Any",
		Policy:    "AllPermitPolicy",
		Domain:    "example.com",
	}
	var out decisionResponse
	err = c.Validate(in, &out)
	if err != nil {
		t.Errorf("expected no error but got %s", err)
	}

	if out.Effect != pdp.EffectPermit || out.Reason != nil || out.X != "AllPermitRule" {
		t.Errorf("got unexpected response: %s", out)
	}
}

func startTestPDPServer(p string, s uint16, t *testing.T) *loggedServer {
	service := fmt.Sprintf("127.0.0.1:%d", s)
	primary := newServer(
		server.WithServiceAt(service),
	)

	if err := primary.s.ReadPolicies(strings.NewReader(p)); err != nil {
		t.Fatalf("can't read policies: %s", err)
	}

	if err := waitForPortClosed(service); err != nil {
		t.Fatalf("port still in use: %s", err)
	}
	go func() {
		if err := primary.s.Serve(); err != nil {
			t.Fatalf("server failed: %s", err)
		}
	}()

	if err := waitForPortOpened(service); err != nil {
		if logs := primary.Stop(); len(logs) > 0 {
			t.Logf("server logs:\n%s", logs)
		}

		t.Fatalf("can't connect to PDP server: %s", err)
	}
	return primary
}
