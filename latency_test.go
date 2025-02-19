package breaker

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func Test_breaker_latencyOK(t *testing.T) {
	type fields struct {
		mu           sync.Mutex
		config       Config
		tripped      bool
		lastTripTime time.Time
		latency      *latencyWindow
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
				mu:           tt.fields.mu,
				config:       tt.fields.config,
				tripped:      tt.fields.tripped,
				lastTripTime: tt.fields.lastTripTime,
				latency:      tt.fields.latency,
			}
			if got := b.latencyOK(); got != tt.want {
				t.Errorf("latencyOK() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latencyWindow_aboveThreshold(t *testing.T) {
	type fields struct {
		values []int64
		index  int
		size   int
	}
	type args struct {
		threshold int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &latencyWindow{
				values: tt.fields.values,
				index:  tt.fields.index,
				size:   tt.fields.size,
			}
			if got := lw.aboveThreshold(tt.args.threshold); got != tt.want {
				t.Errorf("aboveThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latencyWindow_add(t *testing.T) {
	type fields struct {
		values []int64
		index  int
		size   int
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
		{
			name: "Test latencyWindow add",
			fields: fields{
				values: make([]int64, 100),
				index:  0,
				size:   100,
			},
			args: args{
				startTime: time.Now(),
				endTime:   time.Now(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &latencyWindow{
				values: tt.fields.values,
				index:  tt.fields.index,
				size:   tt.fields.size,
			}
			lw.add(tt.args.startTime, tt.args.endTime)
		})
	}
}

func Test_latencyWindow_belowThreshold(t *testing.T) {
	type fields struct {
		values []int64
		index  int
		size   int
	}
	type args struct {
		threshold int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &latencyWindow{
				values: tt.fields.values,
				index:  tt.fields.index,
				size:   tt.fields.size,
			}
			if got := lw.belowThreshold(tt.args.threshold); got != tt.want {
				t.Errorf("belowThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latencyWindow_percentile(t *testing.T) {
	type fields struct {
		values []int64
		index  int
		size   int
	}
	type args struct {
		p float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &latencyWindow{
				values: tt.fields.values,
				index:  tt.fields.index,
				size:   tt.fields.size,
			}
			if got := lw.percentile(tt.args.p); got != tt.want {
				t.Errorf("percentile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newLatencyWindow(t *testing.T) {
	type args struct {
		size int
	}
	tests := []struct {
		name string
		args args
		want *latencyWindow
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newLatencyWindow(tt.args.size); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newLatencyWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}
