// Package main implements the CAMO gossip agent.
// Spec reference: Section 9 — Node Discovery — Signed Gossip Protocol
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// keyFile is the filename used to persist the node keypair.
// Spec reference: Section 5.4 — Identity
const keyFile = "node.key"

// nodeKey holds the node's persistent Ed25519 keypair.
// The public key is the canonical server_id across the gossip network.
type nodeKey struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// serverID returns the base64url-encoded public key.
// This is the canonical node identifier. Spec reference: Section 5.4
func (k *nodeKey) serverID() string {
	return base64.RawURLEncoding.EncodeToString(k.PublicKey)
}

// sign signs the given message with the node's private key.
// Returns base64-encoded signature.
func (k *nodeKey) sign(message []byte) string {
	sig := ed25519.Sign(k.PrivateKey, message)
	return base64.StdEncoding.EncodeToString(sig)
}

// verify checks a base64-encoded signature against a public key and message.
func verify(pubKeyB64, sigB64 string, message []byte) error {
	pubKey, err := base64.RawURLEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return errors.New("invalid public key encoding")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	if !ed25519.Verify(pubKey, message, sig) {
		return errors.New("signature verification failed")
	}
	return nil
}

// persistedKey is the on-disk representation of the keypair.
type persistedKey struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// loadOrGenerateKey loads the node keypair from dataDir, generating
// a new one if none exists. The keypair persists across restarts.
// Spec reference: Section 5.4
func loadOrGenerateKey(dataDir string) (*nodeKey, error) {
	path := filepath.Join(dataDir, keyFile)

	data, err := os.ReadFile(path)
	if err == nil {
		var pk persistedKey
		if err := json.Unmarshal(data, &pk); err != nil {
			return nil, err
		}
		pub, err := base64.StdEncoding.DecodeString(pk.PublicKey)
		if err != nil {
			return nil, err
		}
		priv, err := base64.StdEncoding.DecodeString(pk.PrivateKey)
		if err != nil {
			return nil, err
		}
		return &nodeKey{
			PublicKey:  ed25519.PublicKey(pub),
			PrivateKey: ed25519.PrivateKey(priv),
		}, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// Generate new keypair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	pk := persistedKey{
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(priv),
	}
	data, err = json.MarshalIndent(pk, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}

	return &nodeKey{PublicKey: pub, PrivateKey: priv}, nil
}
