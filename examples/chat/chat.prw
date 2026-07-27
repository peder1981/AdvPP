User Function ChatMain()
    Local cPeerId    := ""
    Local cListen    := ""
    Local cBootstrap := ""
    Local oPeer       as Object

    // --- CLI argument parsing -------------------------------------------------
    // Expected shape: --peer-id p1 --listen 127.0.0.1:9000 --bootstrap 127.0.0.1:9001
    //
    // NOTE: the AdvPP VM has no PARAMCOUNT()/PARAMSTR() native yet, so there is
    // no way to read the OS argument vector (ARGV) from AdvPL today. As a
    // practical workaround for spinning up several local peers without shell
    // scripts, peer configuration is read from environment variables instead
    // (GetEnv() IS a real AdvPP native):
    //   ADVPP_PEER_ID, ADVPP_LISTEN, ADVPP_BOOTSTRAP
    // TODO: add PARAMCOUNT()/PARAMSTR() natives to the compiler/VM so literal
    // "--peer-id p1 --listen ..." flags can be parsed. That is a compiler-level
    // change, out of scope for this chat app.
    cPeerId    := GetEnv("ADVPP_PEER_ID", "peer-1")
    cListen    := GetEnv("ADVPP_LISTEN", "127.0.0.1:9000")
    cBootstrap := GetEnv("ADVPP_BOOTSTRAP", "")

    ConOut("=== AdvPP Freenet Distributed Chat ===")
    ConOut("Peer: " + cPeerId)
    ConOut("Listening: " + cListen)

    oPeer := ChatInit(cPeerId, cListen, cBootstrap)

    ChatLoop()

    ConOut("Chat closed.")
return
endFunc

Static Function ChatInit(cPeerId, cListen, cBootstrap)
    Local oPeer      as Object
    Local oTransport as Object
    Local oRouter    as Object
    Local oAPI       as Object
    Local cHost      := ""
    Local cPort      := ""
    Local nColon     := 0
    Local i          := 0

    ConOut("Initializing peer: " + cPeerId)

    // Step 2a: parse listen address "host:port"
    nColon := At(":", cListen)
    if nColon > 0
        cHost := Substr(cListen, 1, nColon - 1)
        cPort := Substr(cListen, nColon + 1)
    else
        cHost := cListen
        cPort := "9000"
    endif

    // Step 2b: create UDP transport
    // TODO: real P2P binding — pkg/p2p/transport.go's Transport
    // (Listen/Send/Close) is not exposed to AdvPL. The AdvPP VM has no
    // socket natives yet, so ChatTransport below only records the bound
    // address; no packet is actually sent or received.
    oTransport := ChatTransport():new(cHost, Val(cPort))
    ConOut("Transport bound (stub): " + cHost + ":" + cPort)

    // Step 2c: create peer
    // Location is derived from SHA-256(peer id) using the real FWHash()
    // native, folded into a Numeric in [0,1) — same semantics as
    // pkg/p2p/types.go's addressToLocation(), computed in pure AdvPL since
    // the Go Peer struct itself is not bound to the VM.
    oPeer := ChatPeer():new(cPeerId, oTransport)

    // Step 2d: optional bootstrap
    if !Empty(cBootstrap)
        // TODO: real P2P binding — no handshake/gossip exchange happens
        // yet; the bootstrap address is only recorded as a pending
        // neighbor so a later task can wire the real ring join.
        oPeer:AddPendingNeighbor(cBootstrap)
        ConOut("Bootstrap requested (stub): " + cBootstrap)
    endif

    // Step 2e: create router
    // TODO: real P2P binding — pkg/p2p/routing.go's greedy FindNextHop
    // needs real neighbor locations from the network; ChatRouter below is
    // a structural stub until that bridge exists.
    oRouter := ChatRouter():new(oPeer)

    // Step 2f: create PeerAPI
    // TODO: real P2P binding — pkg/p2p/api.go's Get/Update are not
    // exposed. ChatPeerAPI below records subscriptions locally so future
    // tasks can bridge Subscribe/Get/Update to the real contract network.
    oAPI := ChatPeerAPI():new(oRouter)

    // Step 2g: subscribe to the chat contract
    oAPI:Subscribe("contract:chat-main")

    // Step 2h: real P2P binding (Task 4) — bring up the actual Go peer
    // (UDP transport, ring routing, PeerAPI backed by a real store) behind
    // the VM<->Go bridge in pkg/vm/p2p_bridge.go, and subscribe it to the
    // chat contract key. This is the same singleton the PeerAPI_Get()/
    // PeerAPI_Update() calls in ChatRead()/ChatSend() use — it is lazily
    // built from ADVPP_PEER_ID/ADVPP_LISTEN/ADVPP_BOOTSTRAP on first use,
    // but subscribing here (right after ChatInit's own bootstrap step)
    // triggers that build immediately, so a joining peer's JOIN handshake
    // (and the STATE_SYNC reply that brings it up to date) happens before
    // the user ever opens the menu.
    PeerAPI_Subscribe("contract:chat-main")

    ConOut("Peer initialized.")
    ConOut("Local location: " + Str(oPeer:nLocation))
    if Len(oPeer:aNeighbors) == 0
        ConOut("Neighbors: (none)")
    else
        for i := 1 to Len(oPeer:aNeighbors)
            ConOut("Neighbors[" + Str(i) + "]: " + oPeer:aNeighbors[i])
        next
    endif

return oPeer

Static Function ChatLoop()
    Local nOpt  := 0
    Local cOpt  := ""

    do while .T.
        ConOut("")
        ConOut("Chat Menu:")
        ConOut("  1. Read messages")
        ConOut("  2. Send message")
        ConOut("  3. Peer info")
        ConOut("  4. Exit")
        // accept reads a raw line (Character) — convert to Numeric before
        // the do case below, which compares against numeric option codes.
        accept cOpt
        nOpt := Val(cOpt)

        do case
            case nOpt == 1
                ChatRead()
            case nOpt == 2
                ChatSend()
            case nOpt == 3
                ChatInfo()
            case nOpt == 4
                exit
        endcase
    enddo
return

Static Function ChatRead()
    Local aMessages
    Local i
    Local oMsg

    // Call real PeerAPI_Get through the bridge
    aMessages := PeerAPI_Get("contract:chat-main")

    if empty(aMessages)
        ConOut("No messages." + Chr(10))
        return
    endif

    for i := 1 to len(aMessages)
        oMsg := aMessages[i]
        // Display message (format: from (timestamp): text)
        ConOut(oMsg:from + " (" + oMsg:ts + "): " + oMsg:text + Chr(10))
    next
return

Static Function ChatSend()
    Local cText
    Local lResult
    Local cFrom

    ConOut("Message (empty to cancel): ")
    accept cText

    if empty(cText)
        return
    endif

    // Get peer ID for sender
    cFrom := GetEnv("ADVPP_PEER_ID")
    if empty(cFrom)
        cFrom := "peer-unknown"
    endif

    ConOut("Sending..." + Chr(10))

    // Create message object
    Local oMsg as Object
    oMsg := JsonObject():New()
    oMsg:from := cFrom
    oMsg:text := cText
    oMsg:ts := Str(Date()) + Str(Seconds())

    // Call real PeerAPI_Update through the bridge
    // merge function "ChatContractMerge" should be registered in natives
    lResult := PeerAPI_Update("contract:chat-main", oMsg, "ChatContractMerge")

    if lResult
        ConOut("Sent." + Chr(10))
    else
        ConOut("Failed to send." + Chr(10))
    endif
return

Static Function ChatInfo()
    ConOut("TODO: Show peer info")
return

// ---------------------------------------------------------------------------
// P2P peer stand-ins (Task 3 scope)
//
// AdvPP's P2P stack (pkg/p2p/) is Go-only today — Peer, Transport, Router
// and PeerAPI are not bound to the AdvPL VM (no socket natives, no exported
// class bridge). The classes below are structural stubs with the same
// shape/roles as their Go counterparts so ChatInit() has a real object graph
// to build and later tasks (4+) have a clear seam to bridge into the actual
// Go network code. Real network I/O calls are marked with "TODO: real P2P
// binding". Location derivation (SHA-256 -> [0,1)) IS real — it only depends
// on FWHash(), an existing AdvPP native.
//
// GOTCHA: the AdvPP VM's method dispatch (pkg/vm/vm.go callMethod/findMethod)
// is case-sensitive against the declared method name. `method new(...)` is
// only reachable by calling `:new(...)` in lowercase — `:New(...)` (or any
// other casing) fails with "unknown method New on object X", because
// findMethod() only tries the exact-case name and the fully-uppercased name,
// never the declared casing case-insensitively. Real Clipper/AdvPL method
// calls are case-insensitive, so this is a VM bug (not a language quirk) —
// worth fixing in the compiler, out of scope for this app.
// ---------------------------------------------------------------------------

class ChatPeer
    data cId          as Character
    data oTransport   as Object
    data nLocation    as Numeric
    data aNeighbors   as Array

    method new(cId as Character, oTransport as Object) as object Constructor
    method AddPendingNeighbor(cAddr as Character) as Logical
endclass

method new(cId as Character, oTransport as Object) as object class ChatPeer
    ::cId := cId
    ::oTransport := oTransport
    ::nLocation := ChatHashToLocation(FWHash(cId))
    ::aNeighbors := {}
return Self

method AddPendingNeighbor(cAddr as Character) as Logical class ChatPeer
    aAdd(::aNeighbors, cAddr)
return .T.

class ChatTransport
    data cHost as Character
    data nPort as Numeric

    method new(cHost as Character, nPort as Numeric) as object Constructor
    method Send(cAddr as Character, cMessage as Character) as Logical
    method Close() as Logical
endclass

method new(cHost as Character, nPort as Numeric) as object class ChatTransport
    ::cHost := cHost
    ::nPort := nPort
    // TODO: real P2P binding — bind a UDP socket via pkg/p2p/transport.go.
    // No socket natives exist in the AdvPP VM yet; this only stores the
    // address for later use.
return Self

method Send(cAddr as Character, cMessage as Character) as Logical class ChatTransport
    // TODO: real P2P binding — no socket send available yet.
    ConOut("(stub) would send to " + cAddr + ": " + cMessage)
return .T.

method Close() as Logical class ChatTransport
return .T.

class ChatRouter
    data oPeer as Object

    method new(oPeer as Object) as object Constructor
    method FindNextHop(cTargetId as Character) as Character
endclass

method new(oPeer as Object) as object class ChatRouter
    ::oPeer := oPeer
return Self

method FindNextHop(cTargetId as Character) as Character class ChatRouter
    // TODO: real P2P binding — pkg/p2p/routing.go's greedy ring routing
    // needs live neighbor locations from the network. With no neighbors
    // connected yet, forwarding always resolves to self (single-peer MVP).
    if Len(::oPeer:aNeighbors) == 0
        return ::oPeer:cId
    endif
return ::oPeer:aNeighbors[1]

class ChatPeerAPI
    data oRouter        as Object
    data aSubscriptions as Array

    method new(oRouter as Object) as object Constructor
    method Subscribe(cKey as Character) as Logical
    method Get(cKey as Character) as Character
    method Update(cKey as Character, cData as Character) as Logical
endclass

method new(oRouter as Object) as object class ChatPeerAPI
    ::oRouter := oRouter
    ::aSubscriptions := {}
return Self

method Subscribe(cKey as Character) as Logical class ChatPeerAPI
    aAdd(::aSubscriptions, cKey)
    ConOut("Subscribed to: " + cKey)
return .T.

method Get(cKey as Character) as Character class ChatPeerAPI
    // TODO: real P2P binding — pkg/p2p/api.go PeerAPI.Get() not exposed.
return ""

method Update(cKey as Character, cData as Character) as Logical class ChatPeerAPI
    // TODO: real P2P binding — pkg/p2p/api.go PeerAPI.Update() not exposed.
return .F.

// ---------------------------------------------------------------------------
// Location helper: SHA-256(peer id) -> Numeric in [0,1)
// Uses only the first 8 hex chars (32 bits) of FWHash()'s digest to stay
// well within double precision. Same intent as pkg/p2p/types.go's
// addressToLocation(), reimplemented in pure AdvPL.
// ---------------------------------------------------------------------------

Static Function ChatHashToLocation(cHex)
    Local nValue := 0
    Local i      := 0
    Local nLen   := 8 // first 32 bits of the SHA-256 digest

    for i := 1 to nLen
        nValue := nValue * 16 + ChatHexDigit(Substr(cHex, i, 1))
    next

return nValue / 4294967296.0 // 16^8

Static Function ChatHexDigit(cChar)
    Local nAsc := Asc(Lower(cChar))

    if nAsc >= Asc("0") .and. nAsc <= Asc("9")
        return nAsc - Asc("0")
    endif
return nAsc - Asc("a") + 10
