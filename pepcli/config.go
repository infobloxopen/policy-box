package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/infobloxopen/themis/pepcli/perf"
	"github.com/infobloxopen/themis/pepcli/test"
)

type config struct {
	servers stringSet
	hotSpot bool
	input   string
	count   int
	streams int
	output  string

	cmdConf interface{}
	cmd     cmdExec
}

type stringSet []string

func (s *stringSet) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSet) Set(v string) error {
	*s = append(*s, v)
	return nil
}

var conf = config{}

type (
	cmdExec       func(addr string, in, out string, n int, conf interface{}) error
	cmdFlagParser func(args []string) interface{}

	command struct {
		exec   cmdExec
		parser cmdFlagParser
	}

	cmdDesc struct {
		name string
		desc string
	}
)

var (
	cmds = map[string]command{
		test.Name: {
			exec:   test.Exec,
			parser: test.FlagsParser,
		},
		perf.Name: {
			exec:   perf.Exec,
			parser: perf.FlagsParser,
		},
	}

	descs = []cmdDesc{
		{
			name: test.Name,
			desc: test.Description,
		},
		{
			name: perf.Name,
			desc: perf.Description,
		},
	}
)

func init() {
	flag.Usage = usage

	flag.Var(&conf.servers, "s", "PDP server to work with (default 127.0.0.1:5555, "+
		"allowed use multiple to distribute load)")
	flag.BoolVar(&conf.hotSpot, "hot-spot", false, "enables \"hot spot\" balancer (works only for gRPC streaming")
	flag.StringVar(&conf.input, "i", "requests.yaml", "file with YAML formatted list of requests to send to PDP")
	flag.IntVar(&conf.count, "n", 0, "number or requests to send\n\t"+
		"(default and value less than one means all requests from file)")
	flag.IntVar(&conf.streams, "streams", 0, "number of streams to use with gRPC streaming (< 1 unary gRPC)")
	flag.StringVar(&conf.output, "o", "", "file to write command output (default stdout)")

	flag.Parse()

	if len(conf.servers) <= 0 {
		conf.servers = stringSet{"127.0.0.1:5555"}
	}

	count := flag.NArg()
	if count < 1 {
		fmt.Fprint(os.Stderr, "no command provided\n")
		flag.Usage()
		os.Exit(2)
	}

	name := flag.Arg(0)
	cmd, ok := cmds[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "command provided but not defined: %s\n", name)
		flag.Usage()
		os.Exit(2)
	}

	var args []string
	if count > 1 {
		args = flag.Args()[1:count]
	}

	conf.cmdConf = cmd.parser(args)
	conf.cmd = cmd.exec
}

func usage() {
	base := path.Base(os.Args[0])
	fmt.Fprintf(os.Stderr,
		"Usage of %s:\n\n"+
			"  %s [GLOBAL OPTIONS] command [OPTIONS]\n\n"+
			"GLOBAL OPTIONS:\n", base, base)
	flag.PrintDefaults()

	s := make([]string, len(descs))
	for i, desc := range descs {
		s[i] = fmt.Sprintf("%s - %s", desc.name, desc.desc)
	}

	fmt.Fprintf(os.Stderr, "\nCOMMANDS:\n  %s\n", strings.Join(s, "\n  "))
}
