# Network

The Network Package provides Net functionality for the AuraSpeak Project. It includes a Net Server, Net Client, readloop and a router. It utilizes the [AuraSpeak Protocol](https://github.com/AuraSpeak/protocol) an application Protocol that is only designed to provide communication for AuraSpeak.


---

## Requirements

GO Version: 1.25.1

Go dependencies:
- `github.com/auraspeak/protocol` for local development replaced: `replace github.com/auraspeak/protocol => ../protocol`

Indirect:
- `pion/dtls`
- `logrus`

Both are required by `server` and `client`

---

# Structure

## Server

DTLS-UDP-Server that listens on a port. Starts a goroutine per client via readloop.RunReadLoop. Connections are managed over a sync.Map. It provides a Broadcast to distribute packets to all connected clients. On a Server Stop it sends `ClientNeedsDisconnect` to all clients.

## Client

DTLS-UDP-Client using the `Run` method as an main entry point. It connects to a Server via: `Host:Port`, starts read, send and error loops. `Send` sends Raw bytes over a channel.

### Router

Routes packets with protocol.PacketType. To do this, use `OnPacket` with `protocol.PacketType` and `PacketHandler`.

A `PacketHandler` takes `protocol.Packet` and the `peer`. Peer is a string. For the Client the peer is empty; for the Server it is the client address.

## readloop

This package reads from net.Conn, decodes using `protocol.Decode`, calls `router.HandlePacket`; for debugging it also calls a TraceFunc.

---

# Quick start

## Server

`ServerConfig`
- Port
- *dtls.Config
- (Optional) ConnBufSize
- (Optional) TraceFunc

`Functions` / `Methods`
- `Run(ctx)`blocks
- `Stop()` closes the Listener and all clients (including Disconnect Packets) 

## Client

`ClientConfig`
- Host
- Port
- InsecureSkipVerify
- (Optional) RecvBufSize
- (OnConnect)

`Functions` / `Methods`
- `OnPacket(packetType, handler)` Registers a new Packet for routing
- `Run(ctx)` connects and blocks until context cancels or an error occurs.
- `Send(msg []byte)` for sending bytes written to the sendLoop
- `Stop()` for a clean stop

## Routing and Handling
All packet types are described in [AuraSpeak Protocol](https://github.com/AuraSpeak/protocol)/packets.yaml.
The `ClientNeedsDisconnect` disconnects all clients from the Server.

# Testing

Run `go test ./...` to test. Client and Server contains Hooks: `HandlePacketForTest`, `SetConnForTest`, `SetContextForTest`

---

# License

[License](./LICENSE)