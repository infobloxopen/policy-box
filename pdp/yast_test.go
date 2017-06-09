package pdp

import (
	"gopkg.in/yaml.v2"
	"strings"
	"testing"
)

const YASTTestPolicy = `# Test policy
attributes:
  b: boolean
  s: string
  a: address
  "n": network
  d: domain
  ss: Set of Strings
  sn: Set of Networks
  sd: Set of Domains

policies:
  id: root
  policies:
    - id: PolicySet #1
      target:
        - any:
            - all:
                - equal:
                    - attr: s
                    - val:
                        type: string
                        content: test
                - contains:
                    - val:
                        type: network
                        content: 127.0.0.0/8
                    - attr: a

            - contains:
                - val:
                    type: Set of Domains
                    content: ["test.com", "example.com", "test.net", "example.net"]
                - attr: d

        - contains:
            - val:
               type: Set of Strings
               content: ["test", "example"]
            - attr: s

      policies:
        - id: Policy #1.1
          target:
            - equal:
                - attr: s
                - val:
                    type: string
                    content: test

          rules:
            - id: Rule #1.1.1
              target:
                - contains:
                    - attr: d
                    - val:
                        type: Set of Domains
                        content: ["test.com", "example.com", "test.net", "example.net"]

              condition:
                and:
                  - not:
                      - attr: b
                  - or:
                      - equal:
                          - attr: s
                          - val:
                              type: string
                              content: example
                      - contains:
                          - attr: s
                          - val:
                              type: string
                              content: substring

              obligations:
                - d: example.org

              effect: Permit

          alg:
            id: Mapper
            map:
              selector:
                type: String
                path:
                  - attr: d
                content: domains_to_rules

  alg: FirstApplicableEffect`

var YASTTestContent map[string]interface{} = map[string]interface{}{
	"domains_to_rules": map[interface{}]interface{}{"test.com": "Rule #1.1.1"}}

func TestUnmarshalYAST(t *testing.T) {
	ctx := NewYASTCtx("")
	p, err := ctx.UnmarshalYAST([]byte(YASTTestPolicy), YASTTestContent)
	if err != nil {
		t.Errorf("Expected no errors but got:\n%#v\n\n%s\n", err, err)
	} else {
		if p == nil {
			t.Errorf("Expected policy but got nothing")
		}
	}
}

func prepareTestYAST(s string, attrs map[string]AttributeType, includes map[string]interface{}, t *testing.T) (YastCtx, interface{}) {
	c := NewYASTCtx("")
	c.attrs = attrs
	c.includes = includes

	var v interface{}
	err := yaml.Unmarshal([]byte(s), &v)
	if err != nil {
		t.Fatalf("Invalid YAST: %s", err)
	}

	return c, v
}

func assertError(err error, msg string, t *testing.T) {
	if err == nil {
		t.Errorf("Didn't get error as expected")
	} else {
		s := err.Error()
		if !strings.Contains(s, msg) {
			t.Errorf("Expected \"%s\" error but got:\n%#v\n\n%s\n", msg, err, err)
		}
	}
}
