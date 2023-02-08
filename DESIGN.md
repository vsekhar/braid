# Braid Design

* Layered design, each layer consumes the application_data of the previous layer.

* Layers
  * Ordering: tracks timestamp, ancestors (should it track incremental_entry_count?)
  * Pruning: tracks braid size, IEC for each ancestor?
  * Reputation: tracks public key signatures, maintains table of reputable authors

* Not sure. These might be too intertwined.

## Decomposition

* Braid protocol assumes external source of:
  * Membership/acceptance
  * Ordering

* Do we need external ordering? Can this be a function of local timestamps and
acceptance criteria?

## Modules

* Receiver
  * Manages: connections (with Sender)
  * Receives stream of messages from other nodes
  * Receives fill requests from other nodes, forwards to Sender
* Validator
  * Manages peer authors (reputation, allowlist, blocklist)
  * Checks authorship against allowlist/blocklist
* Geneologist
  * Manages: messages pending fill
  * Checks parentage and contributions
  * If missing parents, submits fill request to Sender (with stashed message)
    * Periodically deletes messages without fills
  * If message is a fill response, stashes new parent and submits new fill
  request for upstream if necessary, otherwise re-orders and sends to Author
* Author
  * Manages: frontier
  * Adds message to frontier and recomputes pending parent table
  * Receives new payloads from application and produces a new message
* Sender
  * Manages: connections (with Receiver)
  * Sends new messages from Author
  * Sends fill requests from Geneologist
  * Sends fill responses from Receiver

## TODO
