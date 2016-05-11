// @generated Code generated by thrift-gen. Do not modify.

// Package tcollector is generated code used to make or handle TChannel calls using Thrift.
package tcollector

import (
	"fmt"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/uber/tchannel-go/thrift"
)

// Interfaces for the service and client for the services defined in the IDL.

// TChanTCollector is the interface that defines the server handler and client interface.
type TChanTCollector interface {
	GetSamplingStrategy(ctx thrift.Context, serviceName string) (*SamplingStrategyResponse, error)
	Submit(ctx thrift.Context, span *Span) (*Response, error)
	SubmitBatch(ctx thrift.Context, spans []*Span) ([]*Response, error)
}

// Implementation of a client and service handler.

type tchanTCollectorClient struct {
	thriftService string
	client        thrift.TChanClient
}

func NewTChanTCollectorInheritedClient(thriftService string, client thrift.TChanClient) *tchanTCollectorClient {
	return &tchanTCollectorClient{
		thriftService,
		client,
	}
}

// NewTChanTCollectorClient creates a client that can be used to make remote calls.
func NewTChanTCollectorClient(client thrift.TChanClient) TChanTCollector {
	return NewTChanTCollectorInheritedClient("TCollector", client)
}

func (c *tchanTCollectorClient) GetSamplingStrategy(ctx thrift.Context, serviceName string) (*SamplingStrategyResponse, error) {
	var resp TCollectorGetSamplingStrategyResult
	args := TCollectorGetSamplingStrategyArgs{
		ServiceName: serviceName,
	}
	success, err := c.client.Call(ctx, c.thriftService, "getSamplingStrategy", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

func (c *tchanTCollectorClient) Submit(ctx thrift.Context, span *Span) (*Response, error) {
	var resp TCollectorSubmitResult
	args := TCollectorSubmitArgs{
		Span: span,
	}
	success, err := c.client.Call(ctx, c.thriftService, "submit", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

func (c *tchanTCollectorClient) SubmitBatch(ctx thrift.Context, spans []*Span) ([]*Response, error) {
	var resp TCollectorSubmitBatchResult
	args := TCollectorSubmitBatchArgs{
		Spans: spans,
	}
	success, err := c.client.Call(ctx, c.thriftService, "submitBatch", &args, &resp)
	if err == nil && !success {
	}

	return resp.GetSuccess(), err
}

type tchanTCollectorServer struct {
	handler TChanTCollector
}

// NewTChanTCollectorServer wraps a handler for TChanTCollector so it can be
// registered with a thrift.Server.
func NewTChanTCollectorServer(handler TChanTCollector) thrift.TChanServer {
	return &tchanTCollectorServer{
		handler,
	}
}

func (s *tchanTCollectorServer) Service() string {
	return "TCollector"
}

func (s *tchanTCollectorServer) Methods() []string {
	return []string{
		"getSamplingStrategy",
		"submit",
		"submitBatch",
	}
}

func (s *tchanTCollectorServer) Handle(ctx thrift.Context, methodName string, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	switch methodName {
	case "getSamplingStrategy":
		return s.handleGetSamplingStrategy(ctx, protocol)
	case "submit":
		return s.handleSubmit(ctx, protocol)
	case "submitBatch":
		return s.handleSubmitBatch(ctx, protocol)

	default:
		return false, nil, fmt.Errorf("method %v not found in service %v", methodName, s.Service())
	}
}

func (s *tchanTCollectorServer) handleGetSamplingStrategy(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req TCollectorGetSamplingStrategyArgs
	var res TCollectorGetSamplingStrategyResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.GetSamplingStrategy(ctx, req.ServiceName)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = r
	}

	return err == nil, &res, nil
}

func (s *tchanTCollectorServer) handleSubmit(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req TCollectorSubmitArgs
	var res TCollectorSubmitResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.Submit(ctx, req.Span)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = r
	}

	return err == nil, &res, nil
}

func (s *tchanTCollectorServer) handleSubmitBatch(ctx thrift.Context, protocol athrift.TProtocol) (bool, athrift.TStruct, error) {
	var req TCollectorSubmitBatchArgs
	var res TCollectorSubmitBatchResult

	if err := req.Read(protocol); err != nil {
		return false, nil, err
	}

	r, err :=
		s.handler.SubmitBatch(ctx, req.Spans)

	if err != nil {
		return false, nil, err
	} else {
		res.Success = r
	}

	return err == nil, &res, nil
}