# Deepgram — streaming STT

Real-time speech-to-text via Deepgram's WebSocket API. Low-latency transcription for telephony, voice agents, and live captioning.

```weft
use deepgram
```

Env: `DEEPGRAM_API_KEY`

---

## Streaming (WebSocket)

Open a persistent WebSocket connection, send raw audio chunks, receive transcription events in real-time.

```weft
fn main -> Result {
    stream := deepgram.stream({
        "model": "nova-2",
        "language": "en",
        "encoding": "linear16",
        "sample_rate": 16000,
        "channels": 1,
        "punctuate": true,
        "smart_format": true,
        "interim_results": true,
        "endpointing": 300,
        "vad_events": true,
        "utterance_end_ms": 1000,
    })?

    // send audio chunks (from mic, file, or SIP media fork)
    stream.send(audio_bytes)?

    // receive transcription events
    while true {
        result := stream.recv()?
        match result.event {
            "Results" {
                if result.is_final {
                    say("FINAL: ${result.transcript}")
                } else {
                    say("interim: ${result.transcript}")
                }
                if result.speech_final {
                    say("--- utterance complete ---")
                }
            }
            "UtteranceEnd" {
                say("--- silence detected ---")
            }
            _ { }
        }
    }

    stream.close()
}
```

### Stream options

| Option | Default | What it does |
|--------|---------|-------------|
| `model` | `"nova-2"` | Deepgram model |
| `language` | `"en"` | Language code |
| `encoding` | `"linear16"` | Audio encoding (linear16, mulaw, flac, mp3, opus) |
| `sample_rate` | `16000` | Sample rate in Hz |
| `channels` | `1` | Audio channels |
| `punctuate` | `true` | Add punctuation |
| `smart_format` | `true` | Format numbers, dates, etc. |
| `interim_results` | `true` | Get partial results while speaking |
| `endpointing` | `300` | Milliseconds of silence to detect end of speech |
| `vad_events` | `true` | Voice Activity Detection events |
| `utterance_end_ms` | `1000` | Milliseconds after speech ends to send UtteranceEnd |

### Stream events

| Field | Type | What it means |
|-------|------|--------------|
| `event` | str | `"Results"` or `"UtteranceEnd"` |
| `transcript` | str | The transcribed text |
| `is_final` | bool | True when Deepgram is confident (won't change) |
| `speech_final` | bool | True at natural sentence boundary |
| `confidence` | float | 0.0–1.0 confidence score |

### Stream methods

| Method | What it does |
|--------|-------------|
| `stream.send(bytes)` | Send raw audio data |
| `stream.recv()` | Receive next transcription event |
| `stream.keep_alive()` | Send keep-alive during silence |
| `stream.close()` | Close the connection |

---

## Pre-recorded (REST)

Transcribe an audio file or URL:

```weft
fn main -> Result {
    // from URL
    result := deepgram.transcribe("https://example.com/meeting.wav")?
    say(result.transcript)
    say("confidence: ${result.confidence}")

    // from local file
    result := deepgram.transcribe("/path/to/recording.wav", {
        "model": "nova-2",
        "language": "en",
    })?
    say(result.transcript)
}
```

---

## With telecom

Use Deepgram for real-time transcription in a phone call:

```weft
use telecom

fn main -> Result {
    // configure STT to use Deepgram
    stt_cfg := telecom.stt_config({
        "url": "wss://api.deepgram.com",
        "api_key": env.get("DEEPGRAM_API_KEY"),
    })

    agent := telecom.iva({
        "system": "You are a helpful phone assistant.",
        "greeting": "Hello, how can I help you today?",
        "stt_opts": stt_cfg,
    })

    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

---

## With MCP

Expose Deepgram as an MCP tool:

```weft
fn transcribe_audio(args) -> Result {
    deepgram.transcribe(args.url)
}

fn main {
    mcp.serve_stdio([
        mcp.tool("transcribe", "Transcribe audio from URL", transcribe_audio),
    ])
}
```
