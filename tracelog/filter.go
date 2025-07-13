package tracelog

import (
	"context"
	"time"

	"trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/errs"
	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-opentelemetry/oteltrpc/logs"
	"trpc.group/trpc-go/trpc-opentelemetry/oteltrpc/traces"
)

var DefaultLogFunc = func(ctx context.Context, message string) {
	log.DebugContextf(ctx, "%s", message)
}

func init() {
	filter.Register("tracelog", ServerFilter(), ClientFilter())
}

func ServerFilter() filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {

		start := time.Now()

		rsp, err := next(ctx, req)

		flow, path := buildFlowLog(ctx, rsp, err, logs.FlowKindServer)
		flow.Request.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, req))
		if flow.Request.Body == "" || flow.Request.Body == "null" {
			flow.Request.Body = path
		}
		flow.Response.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, rsp))
		flow.Cost = time.Since(start).String()

		DefaultLogFunc(ctx, flow.OneLineString())

		return rsp, err
	}
}

func ClientFilter() filter.ClientFilter {
	return func(ctx context.Context, req, rsp interface{}, next filter.ClientHandleFunc) error {

		start := time.Now()

		err := next(ctx, req, rsp)

		flow, path := buildFlowLog(ctx, rsp, err, logs.FlowKindClient)
		flow.Request.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, req))
		if flow.Request.Body == "" || flow.Request.Body == "null" {
			flow.Request.Body = path
		}
		flow.Response.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, rsp))
		flow.Cost = time.Since(start).String()

		DefaultLogFunc(ctx, flow.OneLineString())

		return err
	}
}

func buildFlowLog(ctx context.Context, rsp interface{}, err error, kind logs.FlowKind) (*logs.FlowLog, string) {
	msg := trpc.Message(ctx)

	var sourceAddr, targetAddr string
	if msg.RemoteAddr() != nil {
		targetAddr = msg.RemoteAddr().String()
	}
	if msg.LocalAddr() != nil {
		sourceAddr = msg.LocalAddr().String()
	}

	if kind == logs.FlowKindServer {
		sourceAddr, targetAddr = targetAddr, sourceAddr
	}

	code, message, errType := getErrCode(getCodefunc(ctx, rsp, err))

	flow := &logs.FlowLog{
		Kind: kind,
		Source: logs.Service{
			Name:      msg.CallerServiceName(),
			Method:    msg.CallerMethod(),
			Namespace: msg.EnvName(),
			Address:   sourceAddr,
		},
		Target: logs.Service{
			Name:      msg.CalleeServiceName(),
			Method:    msg.CalleeMethod(),
			Namespace: msg.EnvName(),
			Address:   targetAddr,
		},
		Status: logs.Status{
			Code:    int32(code),
			Message: message,
			Type:    toErrorType(errType),
		},
	}

	return flow, msg.ClientRPCName()
}

func toErrorType(t int) string {
	switch t {
	case errs.ErrorTypeBusiness:
		return "business"
	case errs.ErrorTypeCalleeFramework:
		return "callee_framework"
	case errs.ErrorTypeFramework:
		return "framework"
	default:
		return ""
	}
}

func getCodefunc(ctx context.Context, rsp interface{}, err error) (int, error) {
	if err != nil {
		return int(errs.Code(err)), err
	}
	switch v := rsp.(type) {
	case interface {
		GetRetcode() int32
	}:
		return int(v.GetRetcode()), err
	case interface {
		GetRetCode() int32
	}:
		return int(v.GetRetCode()), err
	case interface {
		GetCode() int32
	}:
		return int(v.GetCode()), err
	default:
		return 0, err
	}
}

func getErrCode(errCode int, err error) (int, string, int) {
	var (
		code, errType int
		msg           string
	)

	if err == nil {
		return errCode, msg, errType
	}

	if e, ok := err.(*errs.Error); ok {
		code, msg, errType = int(e.Code), e.Msg, e.Type
	} else {
		code, msg = int(errs.RetUnknown), err.Error()
	}
	return code, msg, errType
}
