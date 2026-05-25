package naming

import "testing"

func TestPackageName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "app"},
		{name: "mixed case", in: "My_App", want: "my-app"},
		{name: "leading digit", in: "123service", want: "app-123service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PackageName(tt.in); got != tt.want {
				t.Fatalf("PackageName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
