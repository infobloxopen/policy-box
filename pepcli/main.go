package main

import (
	"fmt"
	"os"

	"github.com/infobloxopen/themis/pep"
)

func main() {
	opts := []pep.Option{}
	if conf.streams > 0 {
		opts = append(opts,
			pep.WithStreams(conf.streams),
		)
	}

	if len(conf.servers) > 0 {
		if conf.streams > 0 && conf.hotSpot {
			opts = append(opts,
				pep.WithHotSpotBalancer(conf.servers...),
			)
		} else {
			opts = append(opts,
				pep.WithRoundRobinBalancer(conf.servers...),
			)
		}
	}

	err := conf.cmd(
		conf.servers[0],
		opts,
		conf.input,
		conf.output,
		conf.count,
		conf.cmdConf,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
