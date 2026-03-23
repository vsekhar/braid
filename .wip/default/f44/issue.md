---
priority: p2
type: task
created: 2026-03-22T20:24:29-04:00
updated: 2026-03-22T20:24:29-04:00
---

# Use varint length prefix in wire protocol

## Objective

Replace the 4-byte big-endian uint32 length prefix in the wire protocol with `encoding/binary` varint encoding.

## Location

- `wire.go` — `WriteEnvelope` and `ReadEnvelope` functions
- `braid.proto` — comment on the `Envelope` message (lines 78-80)

## Approach

### `wire.go` — `WriteEnvelope`

Replace `binary.Write(w, binary.BigEndian, uint32(len(data)))` with varint encoding. Use `binary.PutUvarint` to encode into a small buffer, then write the buffer:

```go
var buf [binary.MaxVarintLen64]byte
n := binary.PutUvarint(buf[:], uint64(len(data)))
if _, err := w.Write(buf[:n]); err != nil {
    return fmt.Errorf("writing length prefix: %w", err)
}
```

### `wire.go` — `ReadEnvelope`

Replace `binary.Read(r, binary.BigEndian, &length)` with varint decoding. Since varint is variable-length, read one byte at a time using `binary.ReadUvarint` with a `ByteReader`. Wrap `r` with `bufio.NewReader` if it doesn't implement `io.ByteReader`:

```go
br, ok := r.(io.ByteReader)
if !ok {
    br = bufio.NewReader(r)
}
length, err := binary.ReadUvarint(br)
```

Note: if wrapping with `bufio.NewReader`, the subsequent `io.ReadFull` must use the same buffered reader to avoid losing bytes. Alternatively, implement a minimal `byteReader` wrapper that reads one byte at a time from the underlying `io.Reader` to avoid buffering issues:

```go
type byteReader struct{ io.Reader }
func (b byteReader) ReadByte() (byte, error) {
    var buf [1]byte
    _, err := io.ReadFull(b.Reader, buf[:])
    return buf[0], err
}
```

This avoids importing `bufio` and avoids the buffered-reader pitfall entirely.

### `braid.proto` — `Envelope` comment

Update the comment on lines 78-80 from:

```
// Envelope is the wire format for peer-to-peer communication.
// Each envelope is sent as a 4-byte big-endian length prefix followed
// by the serialized Envelope bytes.
```

to:

```
// Envelope is the wire format for peer-to-peer communication.
// Each envelope is sent as a varint-encoded length prefix followed
// by the serialized Envelope bytes.
```

## Acceptance Criteria

- [ ] `WriteEnvelope` uses `binary.PutUvarint` for the length prefix
- [ ] `ReadEnvelope` uses `binary.ReadUvarint` for the length prefix
- [ ] `braid.proto` Envelope comment updated to say "varint-encoded length prefix"
- [ ] Existing tests pass (round-trip encoding/decoding still works)
