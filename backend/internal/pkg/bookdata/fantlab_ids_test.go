package bookdata

import (
	"reflect"
	"testing"
)

func TestOrderFantLabWorkIDs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []int
	}{
		{"plain digits", []string{"363346"}, []int{363346}},
		{"work prefix and spaces", []string{" work7193 ", "WORK12"}, []int{12, 7193}},
		{"dedupe and sort", []string{"30", "5", "30"}, []int{5, 30}},
		{"cap at two", []string{"9", "3", "7"}, []int{3, 7}},
		{"junk dropped", []string{"", "abc", "0", "-4", "12x", "https://fantlab.ru/work5"}, []int{}},
		{"nil", nil, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrderFantLabWorkIDs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
