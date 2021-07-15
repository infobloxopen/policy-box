package server

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/infobloxopen/themis/pdp"
	pb "github.com/infobloxopen/themis/pdp-service"
)

func (s *Server) newContext(c *pdp.LocalContentStorage, in []byte) (*pdp.Context, error) {
	ctx, err := pdp.NewContextFromBytes(c, in)
	if err != nil {
		return nil, newContextCreationError(err)
	}

	return ctx, nil
}

func makeFailureResponse(err error) []byte {
	b, err := pdp.MakeIndeterminateResponse(err)
	if err != nil {
		panic(err)
	}

	return b
}

func makeFailureResponseWithAllocator(f func(n int) ([]byte, error), err error) []byte {
	b, err := pdp.MakeIndeterminateResponseWithAllocator(f, err)
	if err != nil {
		panic(err)
	}

	return b
}

func makeFailureResponseWithBuffer(b []byte, err error) []byte {
	n, err := pdp.MakeIndeterminateResponseWithBuffer(b, err)
	if err != nil {
		panic(err)
	}

	return b[:n]
}

func (s *Server) rawValidate(p *pdp.PolicyStorage, c *pdp.LocalContentStorage, in []byte) ([]byte, error) {
	if p == nil {
		return makeFailureResponse(newMissingPolicyError()), newMissingPolicyError()
	}

	ctx, err := s.newContext(c, in)
	if err != nil {
		return makeFailureResponse(err), err
	}

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithField("context", ctx).Debug("Request context")
	}

	r := p.Root().Calculate(ctx)

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithFields(log.Fields{
			"effect": pdp.EffectNameFromEnum(r.Effect),
			"reason": r.Status,
			"obligations": obligations{
				ctx: ctx,
				o:   r.Obligations,
			},
		}).Debug("Response")
	}

	return r.Marshal(ctx)
}

func (s *Server) rawValidateWithAllocator(p *pdp.PolicyStorage, c *pdp.LocalContentStorage, in []byte, f func(n int) ([]byte, error)) []byte {
	if p == nil {
		return makeFailureResponseWithAllocator(f, newMissingPolicyError())
	}

	ctx, err := s.newContext(c, in)
	if err != nil {
		return makeFailureResponseWithAllocator(f, err)
	}

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithField("context", ctx).Debug("Request context")
	}

	r := p.Root().Calculate(ctx)

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithFields(log.Fields{
			"effect": pdp.EffectNameFromEnum(r.Effect),
			"reason": r.Status,
			"obligations": obligations{
				ctx: ctx,
				o:   r.Obligations,
			},
		}).Debug("Response")
	}

	out, err := r.MarshalWithAllocator(f, ctx)
	if err != nil {
		panic(err)
	}

	return out
}

func (s *Server) rawValidateToBuffer(p *pdp.PolicyStorage, c *pdp.LocalContentStorage, in []byte, out []byte) ([]byte, error) {
	if p == nil {
		return makeFailureResponseWithBuffer(out, newMissingPolicyError()), newMissingPolicyError()
	}

	ctx, err := s.newContext(c, in)
	if err != nil {
		return makeFailureResponseWithBuffer(out, err), err
	}

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithField("context", ctx).Debug("Request context")
	}

	r := p.Root().Calculate(ctx)

	if s.opts.logger.Level >= log.DebugLevel {
		s.opts.logger.WithFields(log.Fields{
			"effect": pdp.EffectNameFromEnum(r.Effect),
			"reason": r.Status,
			"obligations": obligations{
				ctx: ctx,
				o:   r.Obligations,
			},
		}).Debug("Response")
	}

	n, err := r.MarshalToBuffer(out, ctx)
	return out[:n], err
}

// Validate is a server handler for gRPC call
// It handles PDP decision requests
// Return variables are named, so they can be passed to validate
// hooks.
func (s *Server) Validate(ctx context.Context, in *pb.Msg) (*pb.Msg, error) {

	var preCtx context.Context
	if s.opts.validatePreHook != nil {
		preCtx = s.opts.validatePreHook(ctx)
	}

	// Initialize message
	msg := new(pb.Msg)
	var err error

	defer func() {
		if s.opts.validatePostHook != nil {
			// Pass context from preHook to postHook
			s.opts.validatePostHook(preCtx, msg, err)
		}
	}()

	s.RLock()
	p := s.p
	c := s.c
	s.RUnlock()

	if s.opts.autoResponseSize {
		msg.Body, err = s.rawValidate(p, c, in.Body)
		return msg, err
	}

	b := s.pool.Get()
	msg.Body, err = s.rawValidateToBuffer(p, c, in.Body, b)
	s.pool.Put(b)

	return msg, err
}

type obligations struct {
	ctx *pdp.Context
	o   []pdp.AttributeAssignment
}

func (o obligations) String() string {
	if len(o.o) <= 0 {
		return "no attributes"
	}

	lines := make([]string, len(o.o)+1)
	lines[0] = "attributes:"
	for i, e := range o.o {
		id, t, v, err := e.Serialize(o.ctx)
		if err != nil {
			lines[i+1] = fmt.Sprintf("- %d: %s", i+1, err)
		} else {
			lines[i+1] = fmt.Sprintf("- %s.(%s): %q", id, t, v)
		}
	}

	return strings.Join(lines, "\n")
}
