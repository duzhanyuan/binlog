package binlog

import (
	"testing"
)

func TestPosition_IsZero(t *testing.T) {
	testCases := []struct {
		input Position
		want  bool
	}{
		{
			input: Position{
				FileName: "",
				Offset:   0,
			},
			want: true,
		},
		{
			input: Position{
				FileName: "",
				Offset:   1,
			},
			want: true,
		},
		{
			input: Position{
				FileName: "xxx",
				Offset:   0,
			},
			want: true,
		},
		{
			input: Position{
				FileName: "xxx",
				Offset:   1,
			},
			want: false,
		},
	}

	for _, v := range testCases {
		out := v.input.IsZero()
		if v.want != out {
			t.Fatalf("want != out input: %+v want: %v, out: %v", v.input, v.want, out)
		}
	}
}
