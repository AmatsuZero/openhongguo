//go:build js && wasm

package main

import (
	"context"
	"errors"
	"syscall/js"
	"testing"
	"time"

	"github.com/beilunyang/openhongguo/sdk"
)

func TestJSTransportHonorsContextEvenWhenCallbackIgnoresCancellation(t *testing.T) {
	var signal, resolve js.Value
	executor := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve = args[0]
		return nil
	})
	defer executor.Release()
	transport := js.FuncOf(func(_ js.Value, args []js.Value) any {
		signal = args[0].Get("signal")
		return js.Global().Get("Promise").New(executor)
	})
	defer transport.Release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (jsTransport{fn: transport.Value}).Do(ctx, &sdk.Request{Method: "GET", URL: "https://example.test/"})
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline error = %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("transport did not return when its context expired")
	}
	if signal.Type() != js.TypeObject || !signal.Get("aborted").Bool() {
		t.Fatal("the JS request did not receive an aborted signal")
	}
	// The original callback can settle late without invoking released Go funcs.
	resolve.Invoke(map[string]any{"status": 200, "headers": map[string]any{}, "body": js.Global().Get("Uint8Array").New(0)})
	time.Sleep(time.Millisecond)
}

func TestJSTransportDoesNotStartCanceledRequest(t *testing.T) {
	called := false
	transport := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		called = true
		return map[string]any{"status": 200, "headers": map[string]any{}}
	})
	defer transport.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (jsTransport{fn: transport.Value}).Do(ctx, &sdk.Request{Method: "GET", URL: "https://example.test/"})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("canceled request started: called=%v err=%v", called, err)
	}
}
