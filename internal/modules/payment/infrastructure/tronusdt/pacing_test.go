package tronusdt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRequestPacingSerializesStartsAndRespectsCancellation(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	rpc, err := NewRPCWithInterval(server.URL, "", 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	rpc.client = server.Client()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out any
			if e := rpc.call(context.Background(), "/", map[string]any{}, &out); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if len(starts) != 4 {
		t.Fatal("missing request")
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 20*time.Millisecond {
			t.Fatal("requests not paced")
		}
	}
	rpc.nextRequest = time.Now().Add(time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	var out any
	begin := time.Now()
	if rpc.call(ctx, "/", map[string]any{}, &out) == nil || time.Since(begin) > 300*time.Millisecond {
		t.Fatal("pacing ignored cancellation")
	}
	if len(starts) != 4 {
		t.Fatal("cancelled request sent")
	}
}
func TestInvalidRequestIntervalRejected(t *testing.T) {
	for _, d := range []time.Duration{-time.Millisecond, 6 * time.Second} {
		if _, e := NewRPCWithInterval("https://node.example", "", d); e == nil {
			t.Fatal("invalid interval accepted")
		}
	}
}
