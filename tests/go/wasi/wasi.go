//go:generate go run github.com/bytecodealliance/wasm-tools-go/cmd/wit-bindgen-go generate -w service -o bindings ./wit

package wasi

import (
	"log/slog"
	"strconv"

	"github.com/bytecodealliance/wasm-tools-go/cm"
	incominghandler "github.com/rvolosatovs/west/tests/go/wasi/bindings/wasi/http/incoming-handler"
	"github.com/rvolosatovs/west/tests/go/wasi/bindings/wasi/http/types"
	"github.com/rvolosatovs/west/tests/go/wasi/bindings/west-test/fib/fib"
	"github.com/rvolosatovs/west/tests/go/wasi/bindings/west-test/leftpad/leftpad"
)

func init() {
	incominghandler.Exports.Handle = func(request types.IncomingRequest, responseOut types.ResponseOutparam) {
		if err := handle(request, responseOut); err != nil {
			types.ResponseOutparamSet(responseOut, cm.Err[cm.Result[types.ErrorCodeShape, types.OutgoingResponse, types.ErrorCode]](*err))
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}

func handle(req types.IncomingRequest, out types.ResponseOutparam) *types.ErrorCode {
	switch meth := req.Method(); meth {
	case types.MethodPost():
	default:
		slog.Debug("invalid method", "method", meth)
		return ptr(types.ErrorCodeHTTPRequestMethodInvalid())
	}
	q := req.PathWithQuery()
	if q.None() {
		slog.Debug("missing path")
		return ptr(types.ErrorCodeHTTPRequestURIInvalid())
	}
	n, err := strconv.ParseUint(*q.Some(), 10, 32)
	if err != nil {
		slog.Debug("failed to parse uint32 from path", "err", err)
		return ptr(types.ErrorCodeHTTPRequestURIInvalid())
	}

	reqBodyRes := req.Consume()
	if reqBodyRes.IsErr() {
		slog.Debug("failed to consume request body")
		return ptr(types.ErrorCodeInternalError(cm.Some("failed to consume request body")))
	}
	reqBody := reqBodyRes.OK()
	reqBodyStreamRes := reqBody.Stream()
	if reqBodyStreamRes.IsErr() {
		slog.Debug("failed to get request body stream")
		return ptr(types.ErrorCodeInternalError(cm.Some("failed to get request body stream")))
	}
	reqBodyStream := reqBodyStreamRes.OK()

	slog.Debug("constructing new response")
	resp := types.NewOutgoingResponse(req.Headers())

	slog.Debug("getting response body")
	respBodyRes := resp.Body()
	if respBodyRes.IsErr() {
		slog.Debug("failed to get response body")
		return ptr(types.ErrorCodeInternalError(cm.Some("failed to get response body")))
	}
	respBody := respBodyRes.OK()

	slog.Debug("getting response body stream")
	respBodyStreamRes := respBody.Write()
	if respBodyStreamRes.IsErr() {
		slog.Debug("failed to get response body stream")
		return ptr(types.ErrorCodeInternalError(cm.Some("failed to get response body stream")))
	}
	respBodyStream := respBodyStreamRes.OK()

	slog.Debug("setting response outparam")
	types.ResponseOutparamSet(out, cm.OK[cm.Result[types.ErrorCodeShape, types.OutgoingResponse, types.ErrorCode]](resp))

	slog.Debug("calculating Fibonacci number", "n", n)
	count := fib.Fib(uint32(n))

	slog.Debug("invoking leftpad", "count", count)
	padRes := leftpad.Leftpad(*reqBodyStream, *respBodyStream, count, '🧭')
	if padRes.IsErr() {
		slog.Debug("failed to left-pad stream", "err", padRes.Err().LastOperationFailed().ToDebugString())
		return nil
	}
	flushRes := respBodyStream.Flush()
	if flushRes.IsErr() {
		slog.Debug("failed to flush stream", "err", flushRes.Err().LastOperationFailed().ToDebugString())
		return nil
	}
	respBodyStream.ResourceDrop()

	slog.Debug("finishing outgoing body")
	finishRes := types.OutgoingBodyFinish(*respBody, cm.None[types.Fields]())
	if finishRes.IsErr() {
		slog.Error("failed to finish outgoing body", "err", finishRes.Err())
		return nil
	}
	return nil
}
