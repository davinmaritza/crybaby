package mesh

import (
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate WireGuard keypair: %v", err)
	}

	if len(kp.PrivateKey) == 0 {
		t.Errorf("expected non-empty private key")
	}

	if len(kp.PublicKey) == 0 {
		t.Errorf("expected non-empty public key")
	}
}

func TestNodeWireGuardConfig(t *testing.T) {
	cfg := NodeWireGuardConfig("privKeyBase64==", "10.42.0.2", "pubKeyController==", "192.168.1.100:51820", "10.42.0.1")

	if !strings.Contains(cfg, "Address = 10.42.0.2/16") {
		t.Errorf("missing mesh IP address in generated config")
	}

	if !strings.Contains(cfg, "PublicKey = pubKeyController==") {
		t.Errorf("missing controller public key in config")
	}

	if !strings.Contains(cfg, "AllowedIPs = 10.42.0.0/16") {
		t.Errorf("missing AllowedIPs subnet in config")
	}
}
