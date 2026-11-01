package mesh

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// KeyPair represents a WireGuard Curve25519 keypair.
type KeyPair struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// GenerateKeyPair generates a cryptographically secure X25519 keypair for WireGuard.
func GenerateKeyPair() (*KeyPair, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate X25519 key: %w", err)
	}

	privBase64 := base64.StdEncoding.EncodeToString(privKey.Bytes())
	pubBase64 := base64.StdEncoding.EncodeToString(privKey.PublicKey().Bytes())

	return &KeyPair{
		PrivateKey: privBase64,
		PublicKey:  pubBase64,
	}, nil
}

// NodeWireGuardConfig generates a client WireGuard configuration profile for a Windows 11 node.
func NodeWireGuardConfig(nodePrivateKey, nodeMeshIP, controllerPublicKey, controllerEndpoint, controllerMeshIP string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/16

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = 10.42.0.0/16
PersistentKeepalive = 25
`, nodePrivateKey, nodeMeshIP, controllerPublicKey, controllerEndpoint)
}
