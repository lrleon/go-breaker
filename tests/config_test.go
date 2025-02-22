package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"reflect"
	"testing"
)

func Test_loadConfig(t *testing.T) {
	type args struct {
		path string
	}
	var tests []struct {
		name    string
		args    args
		want    breaker.Config
		wantErr bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := breaker.LoadConfig(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_saveConfig(t *testing.T) {
	type args struct {
		path   string
		config *breaker.Config
	}
	var tests []struct {
		name    string
		args    args
		wantErr bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := breaker.SaveConfig(tt.args.path, tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("SaveConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
