package bookdata

import (
	"reflect"
	"testing"
)

func TestOrderOpenLibraryIDs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"q332581 example", []string{"OL27479W", "OL27516W", "OL27455W"}, []string{"OL27455W", "OL27479W", "OL27516W"}},
		{"invalid dropped", []string{"", "foo", "OL12", "ol5W", "OLW", "OL5X", " OL7W "}, []string{"OL7W"}},
		{"duplicates removed", []string{"OL5W", "OL5W", "OL5M", "OL5M"}, []string{"OL5W", "OL5M"}},
		{"works before editions", []string{"OL1M", "OL9W", "OL2M", "OL3W"}, []string{"OL3W", "OL9W", "OL1M"}},
		{"numeric not lexical", []string{"OL100W", "OL20W", "OL3W"}, []string{"OL3W", "OL20W", "OL100W"}},
		{"cap of 3", []string{"OL4W", "OL3W", "OL2W", "OL1W", "OL1M"}, []string{"OL1W", "OL2W", "OL3W"}},
		{"nil", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrderOpenLibraryIDs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
