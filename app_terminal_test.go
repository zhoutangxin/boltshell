package main

import "testing"

func TestSanitizeTerminalOutput_RemovesWGarbage(t *testing.T) {
	garbage := ""
	for i := 0; i < 120; i++ {
		garbage += "w"
	}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "inline uppercase W before login",
			input: "\x1b[?1;2c" + strings.Repeat("W", 120) + "Last login: Thu",
			want:  "Last login: Thu",
		},
		{
			name:  "line of w only",
			input: "\r\n" + garbage + "\r\nLast login:\r\n",
			want:  "\r\nLast login:\r\n",
		},
		{
			name:  "preserve normal prompt",
			input: "[root@localhost ~]# ",
			want:  "[root@localhost ~]# ",
		},
		{
			name:  "strip DA echo",
			input: "\x1b[?62;1;2;6;22;22;21;2c",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeTerminalOutput(tc.input)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
