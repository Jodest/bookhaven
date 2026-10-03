package main

import "testing"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		appEnv  string
		want    command
		wantErr bool
	}{
		{name: "up", args: []string{"up"}, want: command{action: actionUp}},
		{name: "version", args: []string{"version"}, want: command{action: actionVersion}},
		{name: "no command", args: nil, wantErr: true},
		{name: "down defaults to one step", args: []string{"down"}, want: command{action: actionDownSteps, steps: 1}},
		{name: "down 3", args: []string{"down", "3"}, want: command{action: actionDownSteps, steps: 3}},
		{name: "down 0 rejected", args: []string{"down", "0"}, wantErr: true},
		{name: "down negative rejected", args: []string{"down", "-2"}, wantErr: true},
		{name: "down abc rejected", args: []string{"down", "abc"}, wantErr: true},
		{name: "down extra args rejected", args: []string{"down", "1", "2"}, wantErr: true},
		{name: "down-all without flag refused", args: []string{"down-all"}, wantErr: true},
		{name: "down-all with wrong flag refused", args: []string{"down-all", "--yes"}, wantErr: true},
		{name: "down-all with flag", args: []string{"down-all", "--yes-drop-everything"}, appEnv: "development", want: command{action: actionDownAll}},
		{name: "down-all in production without force refused", args: []string{"down-all", "--yes-drop-everything"}, appEnv: "production", wantErr: true},
		{name: "down-all in production with force", args: []string{"down-all", "--yes-drop-everything", "--force"}, appEnv: "production", want: command{action: actionDownAll}},
		{name: "down-all force without confirm refused", args: []string{"down-all", "--force"}, appEnv: "development", wantErr: true},
		{name: "unknown command", args: []string{"drop"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args, tt.appEnv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got command %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
