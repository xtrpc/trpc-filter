package trace

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

func ServerFilter() filter.ServerFilter {
	return func(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {

		start := time.Now()

		rsp, err := next(ctx, req)

		flow := buildFlowLog(ctx, req, err)
		flow.Request.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, req))
		flow.Response.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, rsp))
		flow.Cost = time.Since(start).String()

		log.DebugContextf(ctx, "%s", flow.OneLineString())

		return rsp, err
	}
}

func ClientFilter() filter.ClientFilter {
	return func(ctx context.Context, req, rsp interface{}, next filter.ClientHandleFunc) error {

		start := time.Now()

		err := next(ctx, req, rsp)

		flow := buildFlowLog(ctx, req, err)
		flow.Request.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, req))
		flow.Response.Body = fixStringTooLong(traces.ProtoMessageToCustomJSONStringWithContext(ctx, rsp))
		flow.Cost = time.Since(start).String()

		log.DebugContextf(ctx, "%s", flow.OneLineString())

		return err
	}
}

func buildFlowLog(ctx context.Context, rsp interface{}, err error) *logs.FlowLog {
	msg := trpc.Message(ctx)

	var sourceAddr, targetAddr string
	if msg.RemoteAddr() != nil {
		targetAddr = msg.RemoteAddr().String()
	}
	if msg.LocalAddr() != nil {
		sourceAddr = msg.LocalAddr().String()
	}

	sourceAddr, targetAddr = targetAddr, sourceAddr

	code, message, errType := getErrCode(getCodefunc(ctx, rsp, err))

	flow := &logs.FlowLog{
		Kind: logs.FlowKind(2),
		Source: logs.Service{
			Name:      msg.CallerServiceName(),
			Method:    msg.CallerMethod(),
			Namespace: msg.EnvName(),
			Address:   sourceAddr,
		},
		Target: logs.Service{
			Name:      msg.CalleeServiceName(),
			Method:    msg.CalleeMethod(),
			Address:   targetAddr,
			Namespace: msg.EnvName(),
		},
		Status: logs.Status{
			Code:    int32(code),
			Message: message,
			Type:    toErrorType(errType),
		},
	}

	return flow
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
