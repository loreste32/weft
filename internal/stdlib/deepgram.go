//go:build !js

package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageDeepgram — Deepgram STT SDK with real-time streaming.
func packageDeepgram(env *runtime.Env) runtime.Value {
	p := pkg()

	// deepgram.stream(opts?) -> Result[map]
	// Open a streaming STT WebSocket connection.
	// opts: {api_key, model, language, punctuate, smart_format, encoding, sample_rate, channels, interim_results, endpointing, vad_events, utterance_end_ms, filler_words}
	set(p, "stream", func(args []runtime.Value) (runtime.Value, error) {
		apiKey := getDeepgramKey(env)
		if apiKey == "" {
			return errRes("DEEPGRAM_API_KEY required", "deepgram"), nil
		}

		model := "nova-2"
		language := "en"
		encoding := "linear16"
		sampleRate := 16000
		channels := 1
		punctuate := true
		smartFormat := true
		interimResults := true
		endpointing := 300
		vadEvents := true
		utteranceEndMs := 1000

		if len(args) >= 1 && args[0].Kind == runtime.KindMap {
			mo := args[0].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["api_key"]; ok && v.Kind != runtime.KindNull {
				apiKey = v.String()
			}
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				model = v.String()
			}
			if v, ok := mo.Vals["language"]; ok && v.Kind != runtime.KindNull {
				language = v.String()
			}
			if v, ok := mo.Vals["encoding"]; ok && v.Kind != runtime.KindNull {
				encoding = v.String()
			}
			if v, ok := mo.Vals["sample_rate"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					sampleRate = int(n)
				}
			}
			if v, ok := mo.Vals["channels"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					channels = int(n)
				}
			}
			if v, ok := mo.Vals["endpointing"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					endpointing = int(n)
				}
			}
			if v, ok := mo.Vals["utterance_end_ms"]; ok {
				if n, e := runtime.AsInt(v); e == nil {
					utteranceEndMs = int(n)
				}
			}
			if v, ok := mo.Vals["punctuate"]; ok {
				punctuate = v.B
			}
			if v, ok := mo.Vals["smart_format"]; ok {
				smartFormat = v.B
			}
			if v, ok := mo.Vals["interim_results"]; ok {
				interimResults = v.B
			}
			if v, ok := mo.Vals["vad_events"]; ok {
				vadEvents = v.B
			}
		}

		wsURL := fmt.Sprintf("wss://api.deepgram.com/v1/listen?model=%s&language=%s&encoding=%s&sample_rate=%d&channels=%d&punctuate=%v&smart_format=%v&interim_results=%v&endpointing=%d&vad_events=%v&utterance_end_ms=%d",
			model, language, encoding, sampleRate, channels, punctuate, smartFormat, interimResults, endpointing, vadEvents, utteranceEndMs)

		conn, err := wsClientDialWithHeaders(wsURL, map[string]string{
			"Authorization": "Token " + apiKey,
		})
		if err != nil {
			return errRes("deepgram stream connect: "+err.Error(), "deepgram"), nil
		}

		return runtime.Ok(wrapDeepgramStream(conn)), nil
	}, 1)

	// deepgram.transcribe(audio_url_or_data, opts?) -> Result[map]
	// Transcribe audio via REST API (pre-recorded).
	set(p, "transcribe", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("deepgram.transcribe(audio_url_or_data, opts?)", "deepgram"), nil
		}
		apiKey := getDeepgramKey(env)
		if apiKey == "" {
			return errRes("DEEPGRAM_API_KEY required", "deepgram"), nil
		}

		source := args[0].String()
		model := "nova-2"
		language := "en"
		punctuate := true
		smartFormat := true

		if len(args) >= 2 && args[1].Kind == runtime.KindMap {
			mo := args[1].Obj.(*runtime.MapObj)
			if v, ok := mo.Vals["model"]; ok && v.Kind != runtime.KindNull {
				model = v.String()
			}
			if v, ok := mo.Vals["language"]; ok && v.Kind != runtime.KindNull {
				language = v.String()
			}
		}

		url := fmt.Sprintf("https://api.deepgram.com/v1/listen?model=%s&language=%s&punctuate=%v&smart_format=%v",
			model, language, punctuate, smartFormat)

		var body io.Reader
		contentType := "application/json"
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			body = strings.NewReader(fmt.Sprintf(`{"url":"%s"}`, source))
		} else {
			// assume file path
			f, err := os.Open(source)
			if err != nil {
				return errRes("open audio: "+err.Error(), "deepgram"), nil
			}
			defer f.Close()
			body = f
			contentType = "audio/wav"
		}

		req, _ := http.NewRequest("POST", url, body)
		req.Header.Set("Authorization", "Token "+apiKey)
		req.Header.Set("Content-Type", contentType)

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return errRes("deepgram request: "+err.Error(), "deepgram"), nil
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return errRes(fmt.Sprintf("deepgram HTTP %d: %s", resp.StatusCode, string(respBody)), "deepgram"), nil
		}

		var result map[string]any
		json.Unmarshal(respBody, &result)

		// extract transcript
		transcript := ""
		confidence := 0.0
		if results, ok := result["results"].(map[string]any); ok {
			if chans, ok := results["channels"].([]any); ok && len(chans) > 0 {
				if ch, ok := chans[0].(map[string]any); ok {
					if alts, ok := ch["alternatives"].([]any); ok && len(alts) > 0 {
						if alt, ok := alts[0].(map[string]any); ok {
							if t, ok := alt["transcript"].(string); ok {
								transcript = t
							}
							if c, ok := alt["confidence"].(float64); ok {
								confidence = c
							}
						}
					}
				}
			}
		}

		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = append(mo.Keys, "transcript", "confidence", "raw")
		mo.Vals["transcript"] = runtime.Str(transcript)
		mo.Vals["confidence"] = runtime.Float(confidence)
		mo.Vals["raw"] = goToValue(result)
		return runtime.Ok(m), nil
	}, 2)

	return p
}

func getDeepgramKey(env *runtime.Env) string {
	if k, ok := getenv(env, "DEEPGRAM_API_KEY"); ok {
		return k
	}
	return ""
}

func wrapDeepgramStream(conn *wsClientConn) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	putFn := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("deepgram.stream."+name, arity, fn)
	}

	// stream.send(audio_bytes) — send raw audio data
	putFn("send", 1, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("stream.send(data)", "deepgram"), nil
		}
		data := []byte(args[0].String())
		if err := conn.SendBinary(data); err != nil {
			return errRes(err.Error(), "deepgram"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// stream.recv() -> Result[map]  {transcript, is_final, speech_final, confidence, words}
	putFn("recv", 0, func(args []runtime.Value) (runtime.Value, error) {
		msg, err := conn.Recv()
		if err != nil {
			return errRes(err.Error(), "deepgram"), nil
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(msg), &event); err != nil {
			return errRes("parse: "+err.Error(), "deepgram"), nil
		}

		result := runtime.NewMap()
		rmo := result.Obj.(*runtime.MapObj)
		putVal := func(k string, v runtime.Value) {
			rmo.Keys = append(rmo.Keys, k)
			rmo.Vals[k] = v
		}

		msgType, _ := event["type"].(string)
		putVal("event", runtime.Str(msgType))

		if msgType == "Results" {
			channel, _ := event["channel"].(map[string]any)
			if channel != nil {
				alts, _ := channel["alternatives"].([]any)
				if len(alts) > 0 {
					alt, _ := alts[0].(map[string]any)
					transcript, _ := alt["transcript"].(string)
					confidence, _ := alt["confidence"].(float64)
					putVal("transcript", runtime.Str(transcript))
					putVal("confidence", runtime.Float(confidence))
				}
			}
			isFinal, _ := event["is_final"].(bool)
			speechFinal, _ := event["speech_final"].(bool)
			putVal("is_final", runtime.Bool(isFinal))
			putVal("speech_final", runtime.Bool(speechFinal))
		} else if msgType == "UtteranceEnd" {
			putVal("transcript", runtime.Str(""))
			putVal("is_final", runtime.Bool(true))
			putVal("speech_final", runtime.Bool(true))
		}

		return runtime.Ok(result), nil
	})

	// stream.close() — send close message
	putFn("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		// send Deepgram close message
		conn.Send(`{"type": "CloseStream"}`)
		conn.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	// stream.keep_alive() — send keep-alive
	putFn("keep_alive", 0, func(args []runtime.Value) (runtime.Value, error) {
		conn.Send(`{"type": "KeepAlive"}`)
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}
