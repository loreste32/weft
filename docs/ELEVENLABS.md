# ElevenLabs — streaming TTS

Real-time text-to-speech via ElevenLabs WebSocket API. Low-latency audio generation for telephony, voice agents, and interactive apps.

```weft
use elevenlabs
```

Env: `ELEVENLABS_API_KEY`

---

## Streaming (one-shot)

Send full text, receive audio chunks as they're generated:

```weft
fn main -> Result {
    stream := elevenlabs.stream("Hello, how can I help you today?", {
        "voice_id": "21m00Tcm4TlvDq8ikWAM",
        "model": "eleven_turbo_v2_5",
        "output_format": "pcm_16000",
        "optimize_latency": 4,
    })?

    while true {
        chunk := stream.recv()?
        if chunk.is_final { break }
        // chunk.audio is base64-encoded PCM
        // forward to SIP media, save to file, or play
        say("got ${len(chunk.audio)} bytes of audio")
    }
    stream.close()
}
```

---

## Bidirectional streaming (lowest latency)

Send text chunks incrementally, get audio back as it's generated. Best for real-time voice agents where you're streaming LLM output directly to TTS:

```weft
fn main -> Result {
    ws := elevenlabs.stream_ws({
        "voice_id": "21m00Tcm4TlvDq8ikWAM",
        "model": "eleven_turbo_v2_5",
        "output_format": "pcm_16000",
    })?

    // send text as LLM generates it
    ws.send("Hello, ")?
    ws.send("thank you for calling. ")?
    ws.send("How can I help you today?")?
    ws.flush()  // signal end of this utterance

    // receive audio chunks
    while true {
        chunk := ws.recv()?
        if chunk.is_final { break }
        // forward audio to caller
    }

    // send more text later (same connection)
    ws.send("Let me look that up for you.")?
    ws.flush()

    ws.close()
}
```

### LLM → ElevenLabs pipeline

Stream LLM output token-by-token into ElevenLabs for minimum latency:

```weft
fn main -> Result {
    ws := elevenlabs.stream_ws({"voice_id": "...", "output_format": "pcm_16000"})?

    // stream LLM response directly to TTS
    for event in llm.stream("Explain quantum computing in simple terms")? {
        if event.kind == "text" {
            ws.send(event.text)?
        }
    }
    ws.flush()

    // collect all audio
    while true {
        chunk := ws.recv()?
        if chunk.is_final { break }
        // play or forward chunk.audio
    }
    ws.close()
}
```

---

## REST (non-streaming)

Generate complete audio from text:

```weft
fn main -> Result {
    result := elevenlabs.speak("Welcome to our service.", {
        "voice_id": "21m00Tcm4TlvDq8ikWAM",
        "model": "eleven_turbo_v2_5",
        "output_format": "mp3_44100_128",
    })?
    // result.audio contains the full audio
    // result.content_type is the MIME type
    // result.size is the byte count
    say("generated ${result.size} bytes")
}
```

---

## List voices

```weft
fn main -> Result {
    voices := elevenlabs.voices()?
    for v in voices {
        say("${v.voice_id}: ${v.name}")
    }
}
```

---

## Options

### Stream options

| Option | Default | What it does |
|--------|---------|-------------|
| `voice_id` | `"21m00Tcm4TlvDq8ikWAM"` | Voice to use (Rachel) |
| `model` | `"eleven_turbo_v2_5"` | Model (turbo for low latency) |
| `output_format` | `"pcm_16000"` | Audio format: `pcm_16000`, `pcm_24000`, `mp3_44100_128` |
| `optimize_latency` | `4` | 1–4, higher = lower latency but less quality |

### Output formats for telephony

| Format | Use case |
|--------|----------|
| `pcm_16000` | FreeSWITCH/Asterisk (16kHz narrowband) |
| `pcm_24000` | Higher quality telephony |
| `pcm_44100` | Full quality playback |
| `mp3_44100_128` | File storage, web playback |
| `ulaw_8000` | Legacy telephony (G.711 mu-law) |

---

## With telecom

Use ElevenLabs as the TTS engine for voice agents:

```weft
use telecom

fn main -> Result {
    tts_cfg := telecom.tts_config({
        "url": "https://api.elevenlabs.io",
        "api_key": env.get("ELEVENLABS_API_KEY"),
        "voice": "21m00Tcm4TlvDq8ikWAM",
    })

    agent := telecom.iva({
        "system": "You are a helpful assistant.",
        "greeting": "Hello, how can I help?",
        "tts_opts": tts_cfg,
        "voice": "21m00Tcm4TlvDq8ikWAM",
    })

    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

---

## Deepgram + ElevenLabs (full pipeline)

Real-time STT → LLM → TTS:

```weft
fn main -> Result {
    // STT: listen to caller
    stt := deepgram.stream({"model": "nova-2", "language": "en"})?

    // TTS: speak to caller
    tts := elevenlabs.stream_ws({"voice_id": "...", "output_format": "pcm_16000"})?

    // audio comes from SIP media fork
    stt.send(audio_from_caller)?

    // when caller finishes speaking
    result := stt.recv()?
    if result.speech_final {
        // send to LLM
        for event in llm.stream(result.transcript)? {
            if event.kind == "text" {
                tts.send(event.text)?  // stream LLM output to TTS
            }
        }
        tts.flush()

        // get audio back and play to caller
        while true {
            chunk := tts.recv()?
            if chunk.is_final { break }
            // send chunk.audio to SIP media
        }
    }
}
```
