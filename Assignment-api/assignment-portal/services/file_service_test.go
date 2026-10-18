package services

import "testing"

func TestAllowedUpload(t *testing.T) {
	tests := []struct {
		name        string
		category    string
		extension   string
		contentType string
		want        bool
	}{
		{name: "png image", category: "image", extension: ".png", contentType: "image/png", want: true},
		{name: "wrong image mime", category: "image", extension: ".png", contentType: "application/pdf"},
		{name: "pdf submission", category: "submission", extension: ".pdf", contentType: "application/pdf", want: true},
		{name: "executable disguised as pdf", category: "submission", extension: ".pdf", contentType: "application/x-dosexec"},
		{name: "unsupported archive", category: "submission", extension: ".zip", contentType: "application/zip"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allowedUpload(test.category, test.extension, test.contentType); got != test.want {
				t.Fatalf("allowedUpload() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestValidDocumentSignature(t *testing.T) {
	if !validDocumentSignature("submission", ".pdf", []byte("%PDF-1.7")) {
		t.Fatal("expected valid PDF signature")
	}
	if validDocumentSignature("submission", ".pdf", []byte("MZ executable")) {
		t.Fatal("expected mismatched PDF signature to be rejected")
	}
	if !validDocumentSignature("submission", ".docx", []byte("PK\x03\x04archive")) {
		t.Fatal("expected valid DOCX ZIP signature")
	}
}
