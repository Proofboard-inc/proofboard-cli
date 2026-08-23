package api

import "testing"

// The service sends message as a string on some endpoints and as an array of
// validation failures on others. Reading it only as a string meant an array
// decoded to nothing, so a rejected sync carried no reason at all while the
// service had listed five of them.
func TestAPIMessageDecodesStringAndArray(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"string", `"deviceSignature is required"`, "deviceSignature is required"},
		{"array", `["shas should not be empty","timestamps should not be empty"]`,
			"shas should not be empty; timestamps should not be empty"},
		{"array with blanks", `["only this",""]`, "only this"},
		{"absent", ``, ""},
		{"object is not usable", `{"detail":"x"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decodeAPIMessage([]byte(c.raw)); got != c.want {
				t.Errorf("decodeAPIMessage(%s) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}
