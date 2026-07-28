package auth

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "already E.164", in: "+70000000001", want: "+70000000001"},
		{name: "leading 8", in: "89261234567", want: "+79261234567"},
		{name: "leading 7 no plus", in: "79261234567", want: "+79261234567"},
		{name: "bare mobile prefix", in: "9261234567", want: "+79261234567"},
		{name: "formatted with spaces and dashes", in: "+7 (926) 123-45-67", want: "+79261234567"},
		{name: "too short", in: "12345", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePhone(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizePhone(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePhone(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePhoneIdempotent(t *testing.T) {
	first, err := NormalizePhone("89261234567")
	if err != nil {
		t.Fatalf("NormalizePhone() error = %v", err)
	}
	second, err := NormalizePhone(first)
	if err != nil {
		t.Fatalf("NormalizePhone() second pass error = %v", err)
	}
	if first != second {
		t.Errorf("NormalizePhone() not idempotent: %q != %q", first, second)
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "+79261234567", want: "********4567"},
		{in: "1234", want: "****"},
		{in: "12", want: "**"},
	}

	for _, tt := range tests {
		if got := MaskPhone(tt.in); got != tt.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
