// pkg/vm/p2p_native.go
//
// AdvPL-callable natives for the VM<->Go P2P bridge (pkg/vm/p2p_bridge.go).
// Registered the same way as FWHash/GetEnv in natives.go: uppercase name ->
// func(args []advplrt.Value) (advplrt.Value, error), merged into the VM's
// native function table in registerNatives().
package vm

import (
	"encoding/json"
	"fmt"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerP2PNatives wires the four bridge natives the task-4 brief asks
// for: PeerAPI_Get, PeerAPI_Update, Router_FindNextHop, PeerAPI_Subscribe.
// examples/chat/chat.prw calls these as plain global functions (no :method
// dispatch) — they don't touch the AdvPL-side ChatPeer/ChatRouter/
// ChatPeerAPI stand-in classes from Task 3 at all, which keep existing only
// as display stand-ins in ChatInit().
func registerP2PNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// PeerAPI_Get(cKey) As Array: current merged state for cKey, decoded
	// from the bridge's JSON array-of-messages into an Array of JsonObject
	// (props FROM/TEXT/TS, matching how OP_GET_PROP uppercases `:from`).
	// Returns an empty Array (not an error) when the key hasn't been
	// written yet — chat.prw's ChatRead() only checks Empty(aMessages).
	natives["PEERAPI_GET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := advplrt.ToString(getArg(args, 0))

		bridge, err := getChatBridge()
		if err != nil {
			return advplrt.NewArray([]advplrt.Value{}), fmt.Errorf("PeerAPI_Get: %w", err)
		}

		data, err := bridge.get(key)
		if err != nil {
			// Key not written yet — not an error from AdvPL's point of view.
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		var decoded []any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return advplrt.NewArray([]advplrt.Value{}), fmt.Errorf("PeerAPI_Get: decode state for %q: %w", key, err)
		}

		return jsonValueToAdvpl(decoded), nil
	}

	// PeerAPI_Update(cKey, oData, cMergeOpName) As Logical: encodes oData
	// (a JsonObject, e.g. ChatSend()'s {from,text,ts} message) to JSON,
	// merges it into cKey's state via the named merge op (see
	// chatMergeOps in p2p_bridge.go — "ChatContractMerge" for chat.prw),
	// and broadcasts the merged result to known ring neighbors.
	natives["PEERAPI_UPDATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := advplrt.ToString(getArg(args, 0))
		mergeName := advplrt.ToString(getArg(args, 2))

		payload, err := json.Marshal(advplValueToJSON(getArg(args, 1)))
		if err != nil {
			return advplrt.False, fmt.Errorf("PeerAPI_Update: encode update: %w", err)
		}

		bridge, err := getChatBridge()
		if err != nil {
			return advplrt.False, fmt.Errorf("PeerAPI_Update: %w", err)
		}

		if err := bridge.update(key, payload, mergeName); err != nil {
			return advplrt.False, fmt.Errorf("PeerAPI_Update: %w", err)
		}
		return advplrt.True, nil
	}

	// Router_FindNextHop(cTargetId) As Character: greedy ring routing
	// (pkg/p2p/routing.go) toward SHA-256(cTargetId)'s ring location.
	// Returns this peer's own ID when it's already the terminal hop.
	natives["ROUTER_FINDNEXTHOP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		target := advplrt.ToString(getArg(args, 0))

		bridge, err := getChatBridge()
		if err != nil {
			return advplrt.NewString(""), fmt.Errorf("Router_FindNextHop: %w", err)
		}

		return advplrt.NewString(bridge.findNextHop(target)), nil
	}

	// PeerAPI_Subscribe(cKey) As Logical: registers interest in cKey and,
	// for any neighbor already known (post-bootstrap JOIN), immediately
	// asks it for the current state instead of waiting for the next
	// UPDATE broadcast.
	natives["PEERAPI_SUBSCRIBE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := advplrt.ToString(getArg(args, 0))

		bridge, err := getChatBridge()
		if err != nil {
			return advplrt.False, fmt.Errorf("PeerAPI_Subscribe: %w", err)
		}

		bridge.subscribe(key)
		return advplrt.True, nil
	}
}
