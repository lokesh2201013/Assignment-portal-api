package credentials

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptCredential(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	encrypted, err := Encrypt(key, "smtp-password")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "smtp-password" || !IsEncrypted(encrypted) {
		t.Fatalf("credential was not stored in encrypted form: %q", encrypted)
	}
	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "smtp-password" {
		t.Fatalf("Decrypt() = %q, want original credential", decrypted)
	}
}

func TestDecryptRejectsWrongKeyAndPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	encrypted, err := Encrypt(key, "smtp-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(bytes.Repeat([]byte{0x24}, 32), encrypted); err == nil {
		t.Fatal("Decrypt() accepted a credential encrypted with another key")
	}
	if _, err := Decrypt(key, "legacy-plaintext"); err == nil {
		t.Fatal("Decrypt() accepted plaintext")
	}
}

func TestParseKeyRequires32Bytes(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	parsed, err := ParseKey(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed, key) {
		t.Fatal("ParseKey() returned different key bytes")
	}
	if _, err := ParseKey(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("ParseKey() accepted a key shorter than 32 bytes")
	}
	if _, err := ParseKey(strings.Repeat("!", 44)); err == nil {
		t.Fatal("ParseKey() accepted invalid base64")
	}
}
