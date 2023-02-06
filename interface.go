package braid

// Identity is a public reference to a specific peer.
//
// Identity is conceptually similar to a cryptographic public key and can be
// shared freely.
//
// Identity is not validated or signed by any other authority and it is up to
// the application to determine if the peer presenting an Identity is
// trustworthy.
type Identity interface {
	// Equals returns true if and only if the Identities are equal.
	Equals(q Identity) bool
}

// Secret is a private value used to act as an Identity.
//
// Secret is conceptually similar to a cryptographic private key and should not
// be shared.
type Secret interface {
	// Identity returns the shareable Identity corresponding to this Secret.
	Identity() Identity
}
