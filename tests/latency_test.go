package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"reflect"
	"testing"
	"time"
)

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
		{
			name: "Test aboveThreshold true",
			fields: fields{
				values: []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
				index:  0,
				size:   10,
			},
			args: args{
				threshold: 500,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &breaker.LatencyWindow{
				Values: tt.fields.values,
				Index:  tt.fields.index,
				Size:   tt.fields.size,
			}
			if got := lw.AboveThreshold(tt.args.threshold); got != tt.want {
				t.Errorf("aboveThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latencyWindow_add(t *testing.T) {

	lw := breaker.NewLatencyWindow(10)
	type fields struct {
		values []int64
		index  int
		size   int
	}
	type args struct {
		startTime time.Time
		endTime   time.Time
	}

	timeNow := time.Now()
	// generate 100 latencies between 300 and 1600
	for i := 300; i < 1600; i += 13 {
		latency := time.Duration(i) * time.Millisecond
		startTime := timeNow.Add(-latency)
		endTime := timeNow
		lw.Add(startTime, endTime)
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
				size:   10,
			},
			args: args{
				startTime: time.Now(),
				endTime:   time.Now(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &breaker.LatencyWindow{
				Values: tt.fields.values,
				Index:  tt.fields.index,
				Size:   tt.fields.size,
			}
			lw.Add(tt.args.startTime, tt.args.endTime)
		})
	}

	// print the 95th percentile
	p := 0.95
	percentile := lw.Percentile(p)
	t.Logf("95th percentile: %d ms", percentile)
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
		{
			name: "Test belowThreshold false",
			fields: fields{
				values: []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
				index:  0,
				size:   10,
			},
			args: args{
				threshold: 500,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &breaker.LatencyWindow{
				Values: tt.fields.values,
				Index:  tt.fields.index,
				Size:   tt.fields.size,
			}
			if got := lw.BelowThreshold(tt.args.threshold); got != tt.want {
				t.Errorf("belowThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_latencyWindow_aboveThresholdLatencies(t *testing.T) {
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
		want   []int64
	}{
		{
			name: "Test latencyWindow aboveThresholdLatencies",
			fields: fields{
				values: []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
				index:  0,
				size:   10,
			},
			args: args{
				threshold: 500,
			},
			want: []int64{600, 700, 800, 900, 1000},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lw := &breaker.LatencyWindow{
				Values: tt.fields.values,
				Index:  tt.fields.index,
				Size:   tt.fields.size,
			}
			if got := lw.AboveThresholdLatencies(tt.args.threshold); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("aboveThresholdLatencies() = %v, want %v", got, tt.want)
			}
		})
	}
}
