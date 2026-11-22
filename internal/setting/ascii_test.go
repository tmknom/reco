package setting

import "testing"

func TestUpperASCII(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ASCII lower case letters from a to z become upper case", input: "az_Version9", want: "AZ_VERSION9"},
		{name: "characters next to the lower case range are kept", input: "`{", want: "`{"},
		{name: "non-ASCII letters that Unicode folds to ASCII are kept", input: "ſé", want: "ſé"},
		{name: "invalid UTF-8 bytes are kept as they are", input: "a\xff", want: "A\xff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := upperASCII(tc.input); got != tc.want {
				t.Errorf("upperASCII(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
