package security

import "testing"

func TestPasswordHash(t *testing.T) {
	h, err := HashPassword("gizlin-parol-123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "gizlin-parol-123") || CheckPassword(h, "nädogry") {
		t.Fatal("password check failed")
	}
}

func TestParseSSHKey(t *testing.T) {
	k, err := ParseSSHKey("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGvXk2ZJr8q3vH9cK2b5c3XzQ8n1QpZ6YzJp0aXxqv2B test@pc")
	if err != nil {
		t.Fatal(err)
	}
	if k.Type != "ed25519" || k.Fingerprint[:7] != "SHA256:" {
		t.Fatalf("unexpected %+v", k)
	}
	if _, err := ParseSSHKey("not a key"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseSSHKey("-----BEGIN OPENSSH PRIVATE KEY-----"); err == nil {
		t.Fatal("expected private key rejection")
	}
}
