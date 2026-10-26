package service

import "testing"

func TestSMTPAddressFormatsNumericPort(t *testing.T) {
	if got, want := smtpAddress("smtp.example.com", 587), "smtp.example.com:587"; got != want {
		t.Fatalf("smtpAddress() = %q, want %q", got, want)
	}
	if got, want := smtpAddress("2001:db8::1", 465), "[2001:db8::1]:465"; got != want {
		t.Fatalf("smtpAddress() = %q, want %q", got, want)
	}
}
