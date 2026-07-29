//go:build !js

package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageElevenLabs — ElevenLabs TTS SDK with real-time streaming.
func packageElevenLabs(env *runtime.Env) runtime.Value {
	p := pkg()

	// elevenlabs.stream(text, opts?) -> Result[map]
	// Open a streaming TTS WebSocket connection for low-latency audio.
	// Returns a stream handle with recv() to get audio chunks.
	set(p, "stream", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("elevenlabs.stream(text, opts?)", "elevenlabs"), nil
		}
		apiKey := getElevenLabsKey(env)
		if apiKey == "" {
			return errRes("ELEVENLABS_API_KEY required", "elevenlabs"), nil
		}

		text := args[0].String()
		voiceID := "21m00Tcm4TlvDq8ikWAM" // default: Rachel
		modelID := "eleven_turbo_v2_5"
		outputFormat := "pcm_16000"
		optimizeLatency := 4

		if len(args) >= 2 && args[1].Kind == runtime.KindMap {
			mo := args[1].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["voice_id"]; ok && v.Kind != runtime.KindNull {
				voiceID = v.String()
			}
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				modelID = v.String()
			}
			if v, ok := mo.Vals["output_format"]; ok && v.Kind != runtime.KindNull {
				outputFormat = v.String()
			}
			if v, ok := mo.Vals["optimize_latency"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					optimizeLatency = int(n)
				}
			}
		}

		wsURL := fmt.Sprintf("wss://api.elevenlabs.io/v1/text-to-speech/%s/stream-input?model_id=%s&output_format=%s&optimize_streaming_latency=%d",
			voiceID, modelID, outputFormat, optimizeLatency)

		conn, err := wsClientDialWithHeaders(wsURL, map[string]string{
			"xi-api-key": apiKey,
		})
		if err != nil {
			return errRes("elevenlabs stream connect: "+err.Error(), "elevenlabs"), nil
		}

		// send initial config + text
		initMsg, _ := json.Marshal(map[string]any{
			"text":              " ",
			"voice_settings":    map[string]any{"stability": 0.5, "similarity_boost": 0.8},
			"generation_config": map[string]any{"chunk_length_schedule": []int{120, 160, 250, 290}},
		})
		conn.Send(string(initMsg))

		// send the actual text
		textMsg, _ := json.Marshal(map[string]any{
			"text": text,
		})
		conn.Send(string(textMsg))

		// send empty string to signal end of text
		endMsg, _ := json.Marshal(map[string]any{
			"text": "",
		})
		conn.Send(string(endMsg))

		return runtime.Ok(wrapElevenLabsStream(conn)), nil
	}, 2)

	// elevenlabs.stream_ws(opts?) -> Result[map]
	// Open a bidirectional streaming WebSocket for real-time TTS.
	// Send text chunks, receive audio chunks — lowest latency.
	set(p, "stream_ws", func(args []runtime.Value) (runtime.Value, error) {
		apiKey := getElevenLabsKey(env)
		if apiKey == "" {
			return errRes("ELEVENLABS_API_KEY required", "elevenlabs"), nil
		}

		voiceID := "21m00Tcm4TlvDq8ikWAM"
		modelID := "eleven_turbo_v2_5"
		outputFormat := "pcm_16000"

		if len(args) >= 1 && args[0].Kind == runtime.KindMap {
			mo := args[0].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["voice_id"]; ok && v.Kind != runtime.KindNull {
				voiceID = v.String()
			}
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				modelID = v.String()
			}
			if v, ok := mo.Vals["output_format"]; ok && v.Kind != runtime.KindNull {
				outputFormat = v.String()
			}
		}

		wsURL := fmt.Sprintf("wss://api.elevenlabs.io/v1/text-to-speech/%s/stream-input?model_id=%s&output_format=%s&optimize_streaming_latency=4",
			voiceID, modelID, outputFormat)

		conn, err := wsClientDialWithHeaders(wsURL, map[string]string{
			"xi-api-key": apiKey,
		})
		if err != nil {
			return errRes("elevenlabs ws connect: "+err.Error(), "elevenlabs"), nil
		}

		// send initial config
		initMsg, _ := json.Marshal(map[string]any{
			"text":              " ",
			"voice_settings":    map[string]any{"stability": 0.5, "similarity_boost": 0.8},
			"generation_config": map[string]any{"chunk_length_schedule": []int{120, 160, 250, 290}},
		})
		conn.Send(string(initMsg))

		return runtime.Ok(wrapElevenLabsBidiStream(conn)), nil
	}, 1)

	// elevenlabs.speak(text, opts?) -> Result[map]
	// REST API: synthesize text to audio. Returns {audio, content_type}.
	set(p, "speak", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("elevenlabs.speak(text, opts?)", "elevenlabs"), nil
		}
		apiKey := getElevenLabsKey(env)
		if apiKey == "" {
			return errRes("ELEVENLABS_API_KEY required", "elevenlabs"), nil
		}

		text := args[0].String()
		voiceID := "21m00Tcm4TlvDq8ikWAM"
		modelID := "eleven_turbo_v2_5"
		outputFormat := "mp3_44100_128"

		if len(args) >= 2 && args[1].Kind == runtime.KindMap {
			mo := args[1].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["voice_id"]; ok && v.Kind != runtime.KindNull {
				voiceID = v.String()
			}
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				modelID = v.String()
			}
			if v, ok := mo.Vals["output_format"]; ok && v.Kind != runtime.KindNull {
				outputFormat = v.String()
			}
		}

		url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s?output_format=%s", voiceID, outputFormat)
		body, _ := json.Marshal(map[string]any{
			"text":     text,
			"model_id": modelID,
			"voice_settings": map[string]any{
				"stability":        0.5,
				"similarity_boost": 0.8,
			},
		})

		req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
		req.Header.Set("xi-api-key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return errRes("elevenlabs request: "+err.Error(), "elevenlabs"), nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			errBody, _ := io.ReadAll(resp.Body)
			return errRes(fmt.Sprintf("elevenlabs HTTP %d: %s", resp.StatusCode, string(errBody)), "elevenlabs"), nil
		}

		audio, _ := io.ReadAll(resp.Body)
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "audio", "content_type", "size")
		mo.Vals["audio"] = runtime.Str(string(audio))
		mo.Vals["content_type"] = runtime.Str(resp.Header.Get("Content-Type"))
		mo.Vals["size"] = runtime.Int(int64(len(audio)))
		return runtime.Ok(m), nil
	}, 2)

	// elevenlabs.voices() -> Result[[map]]
	set(p, "voices", func(args []runtime.Value) (runtime.Value, error) {
		apiKey := getElevenLabsKey(env)
		if apiKey == "" {
			return errRes("ELEVENLABS_API_KEY required", "elevenlabs"), nil
		}

		req, _ := http.NewRequest("GET", "https://api.elevenlabs.io/v1/voices", nil)
		req.Header.Set("xi-api-key", apiKey)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return errRes(err.Error(), "elevenlabs"), nil
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var result struct {
			Voices []struct {
				VoiceID string `json:"voice_id"`
				Name    string `json:"name"`
			} `json:"voices"`
		}
		json.Unmarshal(body, &result)

		items := make([]runtime.Value, 0, len(result.Voices))
		for _, v := range result.Voices {
			vm := runtime.NewMap()
			vmo := vm.Obj.(*runtime.MapObj)
			vmo.Keys = append(vmo.Keys, "voice_id", "name")
			vmo.Vals["voice_id"] = runtime.Str(v.VoiceID)
			vmo.Vals["name"] = runtime.Str(v.Name)
			items = append(items, vm)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 0)

	return p
}

func getElevenLabsKey(env *runtime.Env) string {
	if k, ok := getenv(env, "ELEVENLABS_API_KEY"); ok {
		return k
	}
	return ""
}

func wrapElevenLabsStream(conn *wsClientConn) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("elevenlabs.stream."+name, arity, fn)
	}

	// stream.recv() -> Result[map]  {audio, is_final}
	// audio is base64-encoded PCM/MP3 chunk
	putFn("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, err := conn.Recv()
		if err != nil {
			return errRes(err.Error(), "elevenlabs"), nil
		}
		var event map[string]any
		json.Unmarshal([]byte(msg), &event)

		result := runtime.NewMap()
		rmo := result.Obj.(*runtime.MapObj)
		putVal := func(k string, v runtime.Value) {
			rmo.Keys = append(rmo.Keys, k)
			rmo.Vals[k] = v
		}

		audio, _ := event["audio"].(string)
		isFinal, _ := event["isFinal"].(bool)
		putVal("audio", runtime.Str(audio))
		putVal("is_final", runtime.Bool(isFinal || audio == ""))
		return runtime.Ok(result), nil
	})

	// stream.close()
	putFn("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		conn.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}

func wrapElevenLabsBidiStream(conn *wsClientConn) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("elevenlabs.ws."+name, arity, fn)
	}

	// ws.send(text) — send a text chunk for synthesis
	putFn("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ws.send(text)", "elevenlabs"), nil
		}
		msg, _ := json.Marshal(map[string]any{"text": args[0].String()})
		conn.Send(string(msg))
		return runtime.Ok(runtime.Unit()), nil
	})

	// ws.flush() — signal end of current text, trigger generation
	putFn("flush", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, _ := json.Marshal(map[string]any{"text": ""})
		conn.Send(string(msg))
		return runtime.Ok(runtime.Unit()), nil
	})

	// ws.recv() -> Result[map]  {audio, is_final}
	putFn("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, err := conn.Recv()
		if err != nil {
			return errRes(err.Error(), "elevenlabs"), nil
		}
		var event map[string]any
		json.Unmarshal([]byte(msg), &event)

		result := runtime.NewMap()
		rmo := result.Obj.(*runtime.MapObj)
		audio, _ := event["audio"].(string)
		isFinal, _ := event["isFinal"].(bool)
		rmo.Keys = append(rmo.Keys, "audio", "is_final")
		rmo.Vals["audio"] = runtime.Str(audio)
		rmo.Vals["is_final"] = runtime.Bool(isFinal || audio == "")
		return runtime.Ok(result), nil
	})

	// ws.close()
	putFn("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, _ := json.Marshal(map[string]any{"text": ""})
		conn.Send(string(msg))
		conn.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}
