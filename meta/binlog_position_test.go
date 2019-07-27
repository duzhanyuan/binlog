package meta

import (
	"testing"
)

func TestBinlogPosition_IsZero(t *testing.T) {
	testCases := []struct {
		input BinlogPosition
		want  bool
	}{
		struct {
			input BinlogPosition
			want  bool
		}{
			input: BinlogPosition{
				FileName: "",
				Offset:   0,
			},
			want: true,
		},
		struct {
			input BinlogPosition
			want  bool
		}{
			input: BinlogPosition{
				FileName: "",
				Offset:   1,
			},
			want: true,
		},
		struct {
			input BinlogPosition
			want  bool
		}{
			input: BinlogPosition{
				FileName: "xxx",
				Offset:   0,
			},
			want: true,
		},
		struct {
			input BinlogPosition
			want  bool
		}{
			input: BinlogPosition{
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
