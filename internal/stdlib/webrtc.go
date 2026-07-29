//go:build !js

package stdlib

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/loreste/weft/internal/runtime"
)

// WebRTC signaling for browser P2P realtime apps.
// Media/audio/video stays peer-to-peer; Weft only exchanges SDP/ICE over WebSocket rooms.
//
// Client → server (JSON text):
//
//	{"type":"join","room":"lobby","peer":"alice"}
//	{"type":"leave"}
//	{"type":"offer","to":"bob","sdp":"..."}
//	{"type":"answer","to":"alice","sdp":"..."}
//	{"type":"ice","to":"bob","candidate":"..."}
//	{"type":"broadcast","payload":{...}}
//
// Server → client:
//
//	{"type":"welcome","peer":"…","room":"…"}
//	{"type":"peers","peers":["…"]}
//	{"type":"peer-joined","peer":"…"}
//	{"type":"peer-left","peer":"…"}
//	relayed offer/answer/ice with "from"
//	{"type":"error","message":"…"}

type rtcHub struct {
	mu    sync.Mutex
	rooms map[string]map[string]func(string) // room -> peerID -> send(json)
	seq   atomic.Uint64
}

func packageWebRTC(env *runtime.Env) runtime.Value {
	p := pkg()
	set(p, "hub", func(args []runtime.Value) (runtime.Value, error) {
		return newRTCHubValue(env), nil
	}, 0)
	// Default public STUN list for browser RTCPeerConnection config
	set(p, "ice_servers", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.List(
			goToValue(map[string]any{"urls": "stun:stun.l.google.com:19302"}),
			goToValue(map[string]any{"urls": "stun:stun1.l.google.com:19302"}),
		), nil
	}, 0)
	return p
}

func newRTCHubValue(env *runtime.Env) runtime.Value {
	h := &rtcHub{rooms: make(map[string]map[string]func(string))}
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("webrtc.hub."+name, arity, fn)
	}

	// hub.handler() -> fn(conn) for app.ws("/signal", hub.handler())
	put("handler", 0, func(args []runtime.Value) (runtime.Value, error) {
		return runtime.MakeBuiltin("webrtc.signal", 1, func(a []runtime.Value) (runtime.Value, error) {
			if len(a) < 1 {
				return errRes("handler(conn)", "webrtc"), nil
			}
			h.serveConn(env, a[0])
			return runtime.Ok(runtime.Unit()), nil
		}), nil
	})

	// hub.attach(app, path)
	put("attach", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("hub.attach(app, path)", "webrtc"), nil
		}
		app := args[0]
		path := args[1].String()
		wsFn, ok := mapGet(app, "ws")
		if !ok {
			return errRes("hub.attach: expected web.app()", "webrtc"), nil
		}
		handler := runtime.MakeBuiltin("webrtc.signal", 1, func(a []runtime.Value) (runtime.Value, error) {
			if len(a) < 1 {
				return errRes("handler(conn)", "webrtc"), nil
			}
			h.serveConn(env, a[0])
			return runtime.Ok(runtime.Unit()), nil
		})
		if env.Call == nil {
			return errRes("runtime Call not configured", "webrtc"), nil
		}
		if _, err := env.Call(wsFn, []runtime.Value{runtime.Str(path), handler}); err != nil {
			return errRes(err.Error(), "webrtc"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	put("rooms", 0, func(args []runtime.Value) (runtime.Value, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		items := make([]runtime.Value, 0, len(h.rooms))
		for name := range h.rooms {
			items = append(items, runtime.Str(name))
		}
		return runtime.List(items...), nil
	})

	put("peers", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("hub.peers(room)", "webrtc"), nil
		}
		room := args[0].String()
		h.mu.Lock()
		defer h.mu.Unlock()
		peers := h.rooms[room]
		items := make([]runtime.Value, 0, len(peers))
		for id := range peers {
			items = append(items, runtime.Str(id))
		}
		return runtime.List(items...), nil
	})

	return m
}

func (h *rtcHub) serveConn(env *runtime.Env, connVal runtime.Value) {
	if env.Call == nil {
		return
	}
	sendFn, _ := mapGet(connVal, "send")
	recvFn, _ := mapGet(connVal, "recv")
	if sendFn.Kind != runtime.KindBuiltin && sendFn.Kind != runtime.KindFunc {
		return
	}

	sendJSON := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		_, _ = env.Call(sendFn, []runtime.Value{runtime.Str(string(b))})
	}

	var (
		peerID string
		room   string
	)

	leave := func() {
		if peerID == "" || room == "" {
			return
		}
		h.mu.Lock()
		if peers := h.rooms[room]; peers != nil {
			delete(peers, peerID)
			for id, send := range peers {
				if id == peerID {
					continue
				}
				b, _ := json.Marshal(map[string]any{"type": "peer-left", "peer": peerID})
				send(string(b))
			}
			if len(peers) == 0 {
				delete(h.rooms, room)
			}
		}
		h.mu.Unlock()
		peerID, room = "", ""
	}
	defer leave()

	for {
		ret, err := env.Call(recvFn, nil)
		if err != nil {
			return
		}
		text := ""
		if ret.Kind == runtime.KindResult {
			ro := ret.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				return
			}
			text = ro.Val.String()
		} else {
			text = ret.String()
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(text), &msg); err != nil {
			sendJSON(map[string]any{"type": "error", "message": "invalid json"})
			continue
		}
		typ, _ := msg["type"].(string)
		switch typ {
		case "join":
			leave()
			room, _ = msg["room"].(string)
			if room == "" {
				room = "default"
			}
			peerID, _ = msg["peer"].(string)
			if peerID == "" {
				peerID = fmt.Sprintf("peer-%d", h.seq.Add(1))
			}
			send := func(s string) {
				_, _ = env.Call(sendFn, []runtime.Value{runtime.Str(s)})
			}
			h.mu.Lock()
			if h.rooms[room] == nil {
				h.rooms[room] = make(map[string]func(string))
			}
			if _, taken := h.rooms[room][peerID]; taken {
				peerID = fmt.Sprintf("%s-%d", peerID, h.seq.Add(1))
			}
			// notify existing
			var others []string
			for id, sfn := range h.rooms[room] {
				others = append(others, id)
				b, _ := json.Marshal(map[string]any{"type": "peer-joined", "peer": peerID})
				sfn(string(b))
			}
			h.rooms[room][peerID] = send
			h.mu.Unlock()

			sendJSON(map[string]any{"type": "welcome", "peer": peerID, "room": room})
			sendJSON(map[string]any{"type": "peers", "peers": others})

		case "leave":
			leave()
			sendJSON(map[string]any{"type": "left"})

		case "offer", "answer", "ice", "signal":
			if peerID == "" {
				sendJSON(map[string]any{"type": "error", "message": "join a room first"})
				continue
			}
			to, _ := msg["to"].(string)
			if to == "" {
				sendJSON(map[string]any{"type": "error", "message": "missing to"})
				continue
			}
			msg["from"] = peerID
			msg["room"] = room
			b, _ := json.Marshal(msg)
			if !h.relay(room, to, string(b)) {
				sendJSON(map[string]any{"type": "error", "message": "peer not found: " + to})
			}

		case "broadcast":
			if peerID == "" {
				sendJSON(map[string]any{"type": "error", "message": "join a room first"})
				continue
			}
			out := map[string]any{
				"type":    "broadcast",
				"from":    peerID,
				"room":    room,
				"payload": msg["payload"],
			}
			b, _ := json.Marshal(out)
			h.broadcast(room, peerID, string(b))

		default:
			sendJSON(map[string]any{"type": "error", "message": "unknown type: " + typ})
		}
	}
}

func (h *rtcHub) relay(room, to, raw string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	peers := h.rooms[room]
	if peers == nil {
		return false
	}
	send, ok := peers[to]
	if !ok {
		return false
	}
	send(raw)
	return true
}

func (h *rtcHub) broadcast(room, from, raw string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, send := range h.rooms[room] {
		if id == from {
			continue
		}
		send(raw)
	}
}
