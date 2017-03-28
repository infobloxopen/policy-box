package main

//go:generate bash -c "mkdir -p $GOPATH/src/github.com/infobloxopen/policy-box/pdp-service && protoc -I $GOPATH/src/github.com/infobloxopen/policy-box/proto/ $GOPATH/src/github.com/infobloxopen/policy-box/proto/service.proto --go_out=plugins=grpc:$GOPATH/src/github.com/infobloxopen/policy-box/pdp-service && ls $GOPATH/src/github.com/infobloxopen/policy-box/pdp-service"

//go:generate bash -c "mkdir -p $GOPATH/src/github.com/infobloxopen/policy-box/pdp-control && protoc -I $GOPATH/src/github.com/infobloxopen/policy-box/proto/ $GOPATH/src/github.com/infobloxopen/policy-box/proto/control.proto --go_out=plugins=grpc:$GOPATH/src/github.com/infobloxopen/policy-box/pdp-control && ls $GOPATH/src/github.com/infobloxopen/policy-box/pdp-control"

import (
	"net"
	"sync"

	log "github.com/Sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"

	pbc "github.com/infobloxopen/policy-box/pdp-control"
	pbs "github.com/infobloxopen/policy-box/pdp-service"

	"github.com/infobloxopen/policy-box/pdp"

	ot "github.com/opentracing/opentracing-go"
	"errors"
)

type Transport struct {
	Interface net.Listener
	Protocol  *grpc.Server
}

type Server struct {
	Path   string
	Policy pdp.EvaluableType
	Lock   *sync.RWMutex

	Requests Transport
	Control  Transport

	Updates *Queue
}

func NewServer(path string) *Server {
	return &Server{Path: path, Lock: &sync.RWMutex{}, Updates: NewQueue()}
}

func (s *Server) LoadPolicies(path string) error {
	if len(path) == 0 {
		log.Error("Invalid path specified. Failed to Load Policies.")
		return errors.New("Invalid path specified.")
	}

	log.WithField("policy", path).Info("Loading policy")
	p, err := pdp.UnmarshalYASTFromFile(path, s.Path)
	if err != nil {
		log.WithFields(log.Fields{"policy": path, "error": err}).Error("Failed load policy")
		return err
	}

	s.Policy = p
	return nil
}

func (s *Server) ListenRequests(addr string) error {
	log.WithField("address", addr).Info("Opening service port")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithFields(log.Fields{"address": addr, "error": err}).Fatal("Failed to open service port")
		return err
	}

	s.Requests.Interface = ln
	return nil
}

func (s *Server) ListenControl(addr string) error {
	log.WithField("address", addr).Info("Opening control port")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.WithFields(log.Fields{"address": addr, "error": err}).Fatal("Failed to open control port")
		return err
	}

	s.Control.Interface = ln
	return nil
}

func (s *Server) Serve(tracer ot.Tracer) {
	go func()  {
		log.Info("Creating control protocol handler")
		s.Control.Protocol = grpc.NewServer()
		pbc.RegisterPDPControlServer(s.Control.Protocol, s)

		log.Info("Serving control requests")
		s.Control.Protocol.Serve(s.Control.Interface)
	}()

	log.Info("Creating service protocol handler")
	if tracer == nil {
		s.Requests.Protocol = grpc.NewServer()
	} else {
		onlyIfParent := func(parentSpanCtx ot.SpanContext, method string, req, resp interface{}) bool {
			return parentSpanCtx != nil
		}
		intercept := otgrpc.OpenTracingServerInterceptor(tracer, otgrpc.IncludingSpans(onlyIfParent))
		s.Requests.Protocol = grpc.NewServer(grpc.UnaryInterceptor(intercept))
	}
	pbs.RegisterPDPServer(s.Requests.Protocol, s)

	log.Info("Serving decision requests")
	s.Requests.Protocol.Serve(s.Requests.Interface)
}
