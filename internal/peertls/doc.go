// Peertls is an implementation of TLS that is suitable for ad hoc peer
// communication without a central certificate authority.
//
// Peertls uses identities on either side of a network connection to establish
// cryptographic guarantees that the holders of the corresponding secrets are
// the ones participating in the connection.
//
// Applications are responsible for checking the Identity of the remote host on
// a connection and determining whether to trust that host.
package peertls
