package breaker

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestNewBreaker(t *testing.T) {
	type args struct {
		config Config
	}
	tests := []struct {
		name string
		args args
		want Breaker
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBreaker(tt.args.config); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBreaker() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_breaker_Allow(t *testing.T) {
	type fields struct {
		mu            sync.Mutex
		config        Config
		tripped       bool
		lastTripTime  time.Time
		latencyWindow *latencyWindow
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &breaker{
				mu:            tt.fields.mu,
				config:        tt.fields.config,
				tripped:       tt.fields.tripped,
				lastTripTime:  tt.fields.lastTripTime,
				latencyWindow: tt.fields.latencyWindow,
			}
			if got := b.Allow(); got != tt.want {
				t.Errorf("Allow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_breaker_Done(t *testing.T) {
	type fields struct {
		mu            sync.Mutex
		config        Config
		tripped       bool
		lastTripTime  time.Time
		latencyWindow *latencyWindow
	}
	type args struct {
		startTime time.Time
		endTime   time.Time
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &breaker{
				mu:            tt.fields.mu,
				config:        tt.fields.config,
				tripped:       tt.fields.tripped,
				lastTripTime:  tt.fields.lastTripTime,
				latencyWindow: tt.fields.latencyWindow,
			}
			b.Done(tt.args.startTime, tt.args.endTime)
		})
	}
}

func Test_breaker_Reset(t *testing.T) {
	type fields struct {
		mu            sync.Mutex
		config        Config
		tripped       bool
		lastTripTime  time.Time
		latencyWindow *latencyWindow
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &breaker{
				mu:            tt.fields.mu,
				config:        tt.fields.config,
				tripped:       tt.fields.tripped,
				lastTripTime:  tt.fields.lastTripTime,
				latencyWindow: tt.fields.latencyWindow,
			}
			b.Reset()
		})
	}
}

func Test_breaker_Triggered(t *testing.T) {
	type fields struct {
		mu            sync.Mutex
		config        Config
		tripped       bool
		lastTripTime  time.Time
		latencyWindow *latencyWindow
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &breaker{
				mu:            tt.fields.mu,
				config:        tt.fields.config,
				tripped:       tt.fields.tripped,
				lastTripTime:  tt.fields.lastTripTime,
				latencyWindow: tt.fields.latencyWindow,
			}
			if got := b.Triggered(); got != tt.want {
				t.Errorf("Triggered() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_breaker_isHealthy(t *testing.T) {
	type fields struct {
		mu            sync.Mutex
		config        Config
		tripped       bool
		lastTripTime  time.Time
		latencyWindow *latencyWindow
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &breaker{
				mu:            tt.fields.mu,
				config:        tt.fields.config,
				tripped:       tt.fields.tripped,
				lastTripTime:  tt.fields.lastTripTime,
				latencyWindow: tt.fields.latencyWindow,
			}
			if got := b.isHealthy(); got != tt.want {
				t.Errorf("isHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}
