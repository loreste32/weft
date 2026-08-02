# Telecom module

Build voice agents, IVR flows, and real-time telephony applications. One module for call control, STT/TTS, DTMF, routing, queues, CDR, and direct integration with **FreeSWITCH** (ESL) and **Asterisk** (ARI).

```bash
weft get telecom
```

```weft
use telecom
```

Capabilities required: `http`, `ws`, `fs`, `env`, `socket`.

> **Requires a SIP server.** The telecom module handles the application layer — your Weft scripts control what happens on calls. The actual SIP signaling and media is handled by FreeSWITCH or Asterisk. See [SIP server setup](#sip-server-setup) below to get one running.

---

## SIP server setup

You need a running SIP server before any telecom code works. Pick one:

### FreeSWITCH (recommended for IVA)

```bash
# Ubuntu/Debian
sudo apt install -y gnupg2 wget lsb-release
wget -O - https://files.freeswitch.org/repo/deb/debian-release/fscomm.gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/freeswitch.gpg
echo "deb [signed-by=/usr/share/keyrings/freeswitch.gpg] https://files.freeswitch.org/repo/deb/debian-release/ $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/freeswitch.list
sudo apt update && sudo apt install -y freeswitch-meta-all

# start
sudo systemctl enable freeswitch
sudo systemctl start freeswitch
```

Enable the Event Socket for Weft:

```bash
# /etc/freeswitch/autoload_configs/event_socket.conf.xml
# (usually enabled by default)
```

```xml
<configuration name="event_socket.conf" description="Socket Client">
  <settings>
    <param name="nat-map" value="false"/>
    <param name="listen-ip" value="127.0.0.1"/>
    <param name="listen-port" value="8021"/>
    <param name="password" value="ClueCon"/>
    <param name="apply-inbound-acl" value="loopback.auto"/>
  </settings>
</configuration>
```

Verify it works:

```bash
# test ESL connection
weft run -e 'esl := telecom.esl_connect(null, null, null)?; say(telecom.esl_status(esl)?); telecom.esl_close(esl)?'
```

#### Route calls to your Weft app (outbound mode)

Add to the FreeSWITCH dialplan so incoming calls hit your Weft script:

```xml
<!-- /etc/freeswitch/dialplan/default.xml -->
<extension name="weft-app">
  <condition field="destination_number" expression="^(10[0-9]{2})$">
    <action application="socket" data="127.0.0.1:9090 async full"/>
  </condition>
</extension>
```

Then run your Weft outbound server:

```weft
use telecom

fn main {
    telecom.esl_outbound_server(9090, fn(channel_data, send) {
        send("sendmsg\ncall-command: execute\nexecute-app-name: answer\n")?
        send("sendmsg\ncall-command: execute\nexecute-app-name: playback\nexecute-app-arg: /usr/share/freeswitch/sounds/en/us/callie/ivr/ivr-welcome.wav\n")?
    })
}
```

#### Route calls to your Weft webhook (HTTP mode)

Use `mod_httapi` or `mod_xml_curl` to send call events to your HTTP server:

```weft
use telecom

fn main -> Result {
    agent := telecom.iva({
        "system": "You are a helpful assistant.",
        "greeting": "Hello, how can I help?",
    })
    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

### Asterisk

```bash
# Ubuntu/Debian
sudo apt install -y asterisk

# or from source for latest
cd /usr/src
wget https://downloads.asterisk.org/pub/telephony/asterisk/asterisk-21-current.tar.gz
tar xf asterisk-21-current.tar.gz && cd asterisk-21.*
./configure && make && sudo make install && sudo make samples
```

Enable ARI:

```ini
; /etc/asterisk/http.conf
[general]
enabled=yes
bindaddr=127.0.0.1
bindport=8088

; /etc/asterisk/ari.conf
[general]
enabled=yes

[weft]
type=user
password=weft123
read_only=no
```

Create a Stasis dialplan so calls enter your Weft app:

```ini
; /etc/asterisk/extensions.conf
[default]
exten => _X.,1,NoOp(Sending to Weft)
 same => n,Stasis(weft)
 same => n,Hangup()
```

Reload Asterisk:

```bash
sudo asterisk -rx "core reload"
```

Verify:

```bash
weft run -e 'ari := telecom.ari_connect({"password": "weft123", "app": "weft"}); say(telecom.ari_endpoints(ari)?)'
```

### Cloud SIP (no server to manage)

If you don't want to run your own SIP server, use a cloud provider that sends webhooks:

| Provider | How Weft connects |
|----------|------------------|
| Twilio | Webhook HTTP — `telecom.webhook_server` receives TwiML-style events |
| Vonage (Nexmo) | Webhook HTTP — NCCO-style actions |
| Telnyx | Webhook HTTP — similar event format |
| SignalWire | FreeSWITCH-based — ESL or webhook |

For cloud providers, your Weft webhook server receives call events and returns action instructions. No ESL/ARI needed — just HTTP:

```weft
use telecom

fn main -> Result {
    agent := telecom.iva({
        "system": "Customer support agent.",
        "greeting": "Thanks for calling. How can I help?",
        "tools": [llm.tool("lookup_order", lookup_order)],
    })

    // expose on public URL (use ngrok for dev)
    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

### SIP softphones for testing

You don't need real phones to test. Use a SIP softphone:

| App | Platform | Free |
|-----|----------|------|
| Otel | Desktop (all) | Yes |
| Otel | Web browser | Yes |
| Otel | macOS/Windows | Yes |
| Otel | Android/iOS | Yes |

Register the softphone against your FreeSWITCH or Asterisk at `127.0.0.1`, extension `1001`, password `1234` (default).

### Minimal test stack

Fastest way to test telecom code:

```bash
# 1. install FreeSWITCH
sudo apt install -y freeswitch-meta-all

# 2. install weft + telecom module
curl -fsSL https://weftproject.dev/install.sh | sh
weft get telecom

# 3. write your IVA
cat > iva.weft << 'WEFT'
use telecom

fn main -> Result {
    agent := telecom.iva({
        "system": "You are a test assistant.",
        "greeting": "Hello! This is a test. Say something.",
    })
    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
WEFT

# 4. run it
weft run iva.weft

# 5. call extension 1001 from a softphone → hear greeting
```

---

## IVA — Interactive Voice Agent

An IVA is an LLM-powered voice assistant. You define tools it can call during a phone conversation — check balances, schedule appointments, look up records — and the module handles the conversation loop, STT/TTS, and call flow.

```weft
use telecom

fn check_balance(account) -> Result {
    db := db.open("sqlite:accounts.db")?
    rows := db.query("SELECT balance FROM accounts WHERE id = ?", [account])?
    if len(rows) > 0 { Ok(rows[0].balance) }
    else { Err("account not found") }
}

fn transfer_call(department) {
    telecom.transfer("sip:$department@pbx.local", null)
}

fn main -> Result {
    agent := telecom.iva({
        "system": "You are a helpful bank phone assistant. Help callers check balances, transfer to departments, and answer questions about their accounts.",
        "tools": [
            llm.tool("check_balance", check_balance, "Check account balance by account number"),
            llm.tool("transfer_call", transfer_call, "Transfer to a department: billing, support, loans"),
        ],
        "greeting": "Welcome to Acme Bank. How can I help you today?",
        "voice": "nova",
        "language": "en-US",
        "max_turns": 30,
    })

    // start webhook server for your SIP platform
    telecom.webhook_server(8080, fn(event) {
        telecom.iva_handle_event(agent, event)?
    })
}
```

### IVA events

The IVA processes these event types from your SIP platform:

| Event type | When | IVA action |
|-----------|------|------------|
| `call_start` | New inbound call | Answer + play greeting + start listening |
| `speech` | STT result ready | Send to LLM → generate response → play TTS |
| `dtmf` | Caller pressed digits | Tell LLM "caller pressed: 123" |
| `call_end` | Hangup | Clean up conversation state |

### IVA configuration

| Field | Default | What it does |
|-------|---------|-------------|
| `system` | Generic assistant | System prompt for the LLM |
| `tools` | `[]` | Weft functions the LLM can call |
| `greeting` | None (silent) | First thing spoken when call starts |
| `voice` | `"default"` | TTS voice ID |
| `language` | `"en-US"` | STT/TTS language |
| `max_turns` | `50` | Max conversation turns before auto-hangup |
| `end_on` | `["goodbye", "bye", ...]` | Phrases that trigger hangup |
| `llm_opts` | `{}` | Passed to `llm.ask` (model, max_steps, etc.) |
| `stt_opts` / `tts_opts` | `{}` | STT/TTS service config |

---

## Call flow actions

Build call flows by composing actions. These produce maps that your SIP platform translates to signaling.

```weft
use telecom

fn main -> Result {
    // answer, play greeting, collect digits, route
    actions := telecom.actions([
        telecom.answer(null),
        telecom.speak("Welcome. Press 1 for sales, 2 for support.", null),
        telecom.gather({"kind": "dtmf", "max_digits": 1, "timeout": 5}),
    ])
    say(json.pretty(actions))
}
```

### Available actions

| Function | What it does |
|----------|-------------|
| `telecom.answer(opts?)` | Answer the call |
| `telecom.hangup(opts?)` | End the call |
| `telecom.play(src, opts?)` | Play audio file or TTS |
| `telecom.speak(text, opts?)` | TTS shorthand |
| `telecom.gather(opts)` | Collect DTMF or speech |
| `telecom.record(opts?)` | Record the call |
| `telecom.pause(seconds?)` | Silence |
| `telecom.transfer(dest, opts?)` | Transfer (blind or attended) |
| `telecom.bridge(call_id, opts?)` | Bridge two legs |
| `telecom.conference(room, opts?)` | Join conference |
| `telecom.redirect(url)` | Redirect to different webhook |
| `telecom.reject(reason?)` | Reject incoming call |
| `telecom.actions(list)` | Bundle actions into response |

---

## DTMF menus

```weft
use telecom

fn main -> Result {
    // simple press-1-for-X menu
    menu := telecom.dtmf_menu(
        "Press 1 for sales, 2 for support, 3 for billing.",
        {
            "1": telecom.transfer("sip:sales@pbx", null),
            "2": telecom.transfer("sip:support@pbx", null),
            "3": telecom.transfer("sip:billing@pbx", null),
        },
        {"retries": 3, "timeout": 5}
    )
    say(json.pretty(menu))
}
```

### Digit collection

```weft
// collect account number
pin := telecom.dtmf_collect("Please enter your 6-digit account number.", 6, {
    "timeout": 10,
    "finish_on_key": "#",
})

// yes/no confirmation
confirm := telecom.dtmf_confirm("Transfer to billing?", null)
```

---

## STT (Speech-to-Text)

```weft
use telecom

fn main -> Result {
    // configure STT
    cfg := telecom.stt_config({
        "url": "https://stt.example.com",
        "api_key": env.get("STT_KEY"),
        "language": "en-US",
    })

    // transcribe an audio file
    result := telecom.stt_transcribe("https://example.com/recording.wav", cfg)?
    say("Text: ${result.text}")
    say("Confidence: ${result.confidence}")

    // get WebSocket URL for streaming STT
    ws_url := telecom.stt_stream_url(cfg)
    say("Stream: $ws_url")
}
```

Environment: `WEFT_STT_URL`, `WEFT_STT_KEY`.

---

## TTS (Text-to-Speech)

```weft
use telecom

fn main -> Result {
    cfg := telecom.tts_config({
        "url": "https://tts.example.com",
        "api_key": env.get("TTS_KEY"),
        "voice": "nova",
    })

    // synthesize speech
    result := telecom.tts_speak("Your balance is $42.50", cfg)?
    say("Audio: ${result.audio_url}")
    say("Duration: ${result.duration_ms}ms")

    // list available voices
    voices := telecom.tts_voices(cfg)?
    for v in voices { say(v.name) }
}
```

Environment: `WEFT_TTS_URL`, `WEFT_TTS_KEY`.

---

## SSML prompts

Build Speech Synthesis Markup Language for natural-sounding prompts:

```weft
use telecom

fn main {
    // speak digits individually
    say(telecom.ssml([
        {"text": "Your account number is "},
        {"say_as": {"interpret_as": "digits", "text": "123456"}},
        {"pause_dur": "500ms"},
        {"text": "Thank you."},
    ]))

    // shorthand helpers
    say(telecom.prompt("digits", "8005551234"))   // phone number
    say(telecom.prompt("spell", "ACME"))           // letter by letter
    say(telecom.prompt("currency", "$42.50"))      // money
}
```

---

## Routing

### DID routing

```weft
use telecom

fn main -> Result {
    handler := telecom.did_route("+18005551234", {
        "+18005551234": {"action": "queue", "queue": "sales"},
        "+18005559999": {"action": "queue", "queue": "support"},
        "+44*": {"action": "transfer", "dest": "sip:uk@pbx"},
        "*": {"action": "ivr", "menu": "main"},
    })
    say(json.pretty(handler))
}
```

### Time-of-day routing

```weft
use telecom

fn main {
    handler := telecom.time_route([
        {"start": "08:00", "end": "18:00", "days": ["mon","tue","wed","thu","fri"],
         "handler": {"action": "queue", "queue": "main"}},
        {"handler": {"action": "voicemail"}},
    ])
    say(json.pretty(handler))
}
```

### Skills-based routing

```weft
use telecom

fn main {
    agents := [
        {"id": "a1", "skills": ["billing", "english"], "available": true, "calls_today": 3},
        {"id": "a2", "skills": ["billing", "spanish"], "available": true, "calls_today": 1},
        {"id": "a3", "skills": ["support", "english"], "available": false, "calls_today": 5},
    ]
    best := telecom.skill_route({"skill_tags": ["billing", "spanish"]}, agents)
    say(best.id)  // "a2"
}
```

Also: `telecom.geo_route`, `telecom.round_robin`, `telecom.failover`.

---

## Call queues

```weft
use telecom

fn main {
    q := telecom.queue_create("support", {
        "max_wait_sec": 300,
        "max_size": 50,
        "announce_position": true,
    })

    // add callers
    telecom.queue_add(q, "call-001", {"priority": 2, "skill_tags": ["billing"]})
    telecom.queue_add(q, "call-002", {"priority": 1})
    telecom.queue_add(q, "call-003", {"priority": 3, "skill_tags": ["urgent"]})

    // next caller (highest priority, longest wait)
    next := telecom.queue_next(q)
    say("next: ${next.call_id}")  // call-003 (priority 3)

    // stats
    stats := telecom.queue_stats(q)
    say("waiting: ${stats.size}, avg wait: ${stats.avg_wait_sec}s")
}
```

---

## CDR (Call Detail Records)

```weft
use telecom

fn main -> Result {
    // create a CDR from a call event
    record := telecom.cdr({
        "call_id": "abc-123",
        "direction": "inbound",
        "from": "+18005551234",
        "to": "+18005559999",
        "duration_sec": 180,
        "status": "answered",
        "agent_id": "a1",
    })

    // store to JSONL file
    telecom.cdr_store(record, "cdrs.jsonl")?
    say("CDR stored")
}
```

---

## Dial plan

```weft
use telecom

fn main {
    // normalize numbers to E.164
    say(telecom.normalize("(800) 555-1234", "1"))   // +18005551234
    say(telecom.normalize("07911123456", "44"))       // +447911123456

    // detect country
    say(telecom.country("+33612345678"))  // FR
    say(telecom.country("+18005551234"))  // US/CA

    // toll-free check
    say(telecom.is_tollfree("+18005551234"))  // true
    say(telecom.is_tollfree("+12125551234"))  // false

    // mask for display
    say(telecom.mask("+18005551234"))  // +1***551234

    // SIP URI
    say(telecom.sip_uri("1001", "pbx.local", {"transport": "tcp"}))
    // sip:1001@pbx.local;transport=tcp
}
```

---

## FreeSWITCH ESL

Connect directly to FreeSWITCH via the Event Socket Library for real-time call control.

**Protocol handling:** The ESL client uses proper Content-Length-based frame parsing with partial-read buffering. Command replies and async events are separated — `recv_event()` queues events that arrive during command execution. Both `\n\n` and `\r\n\r\n` delimiters are supported.

**Safety limits:** Max header block 64KB, max body 10MB, max 128 headers per frame, max 10k queued events, and a parser buffer capped at the 10MB body limit plus 64KB of frame overshoot. These prevent unbounded memory growth from malformed or high-volume FreeSWITCH streams.

**Concurrency model:** After authentication, one reader owns socket reads and a coordinator owns command writes and response/event routing. `command()` and `recv_event()`/`recv_event_timeout()` may be used concurrently; event frames are delivered before command replies, and timed-out event waits are removed from the coordinator queue. Normal connection reads install no hidden deadline, so an idle ESL event stream remains open until the caller closes it or sets a deadline.

**Offline verification:** `weft test packages/telecom -q -run parser` covers framing and validation. CI also runs `TestESLBlackBoxProcess`, which launches the actual Weft CLI against a local mock TCP server and verifies authentication, coalesced frames, event-before-reply routing, concurrent commands, timeout cancellation, and cleanup. This test does not claim live FreeSWITCH compatibility; release validation still requires a real FreeSWITCH/Asterisk environment and SIP scenarios.

### Inbound mode

```weft
use telecom

fn main -> Result {
    esl := telecom.esl_connect("127.0.0.1", 8021, "ClueCon")?
    say(telecom.esl_status(esl)?)

    // subscribe to events
    telecom.esl_subscribe(esl, "CHANNEL_ANSWER DTMF CHANNEL_HANGUP")?

    // originate a call
    telecom.esl_originate(esl, "sofia/internal/1001@domain", "&park", null)?

    // show active channels
    say(telecom.esl_show(esl, "channels")?)

    telecom.esl_close(esl)?
}
```

### Call control

```weft
use telecom

fn handle_call(esl, uuid) -> Result {
    telecom.esl_answer(esl, uuid)?
    telecom.esl_playback(esl, uuid, "/sounds/welcome.wav")?

    // IVR: collect digits
    telecom.esl_play_and_get_digits(esl, uuid, {
        "min": 1, "max": 4, "tries": 3, "timeout_ms": 5000,
        "terminator": "#",
        "file": "/sounds/enter-pin.wav",
        "invalid_file": "/sounds/invalid.wav",
        "var_name": "pin",
    })?

    // bridge to another extension
    telecom.esl_bridge(esl, uuid, "sofia/internal/1002@domain")?

    // or transfer
    telecom.esl_transfer(esl, uuid, "1002", "default")?

    // record
    telecom.esl_record(esl, uuid, "/recordings/$uuid.wav", 300, null)?

    // conference
    telecom.esl_conference(esl, uuid, "room1", {"muted": false})?

    telecom.esl_hangup(esl, uuid, "NORMAL_CLEARING")?
}
```

### Outbound mode

FreeSWITCH connects to your Weft script when a call hits the dialplan:

```xml
<!-- FreeSWITCH dialplan -->
<action application="socket" data="10.0.0.5:9090 async full"/>
```

```weft
use telecom

fn main {
    telecom.esl_outbound_server(9090, fn(channel_data, send) {
        // answer the call
        send("sendmsg\ncall-command: execute\nexecute-app-name: answer\n")?

        // play greeting
        send("sendmsg\ncall-command: execute\nexecute-app-name: playback\nexecute-app-arg: /sounds/welcome.wav\n")?

        say("handling call from: $channel_data")
    })
}
```

---

## Asterisk ARI

Control Asterisk via the REST Interface + WebSocket events.

### Connect and listen

```weft
use telecom

fn main -> Result {
    ari := telecom.ari_connect({
        "host": "127.0.0.1",
        "port": "8088",
        "username": "admin",
        "password": "secret",
        "app": "myapp",
    })

    // list endpoints
    endpoints := telecom.ari_endpoints(ari)?
    for ep in endpoints { say(ep) }

    // listen for events (blocking)
    telecom.ari_listen(ari, fn(event) {
        say("event: ${event.type}")

        if event.type == "StasisStart" {
            id := event.channel.id
            say("call from: ${event.channel.caller.number}")

            // answer and play
            telecom.ari_answer(ari, id)?
            telecom.ari_play(ari, id, "sound:hello-world", null)?
        }

        if event.type == "ChannelDtmfReceived" {
            say("DTMF: ${event.digit}")
        }
    })?
}
```

### Bridge two calls

```weft
use telecom

fn bridge_calls(ari, chan_a, chan_b) -> Result {
    bridge := telecom.ari_create_bridge(ari, "mixing")?
    telecom.ari_answer(ari, chan_a)?
    telecom.ari_answer(ari, chan_b)?
    telecom.ari_add_to_bridge(ari, bridge.id, "$chan_a,$chan_b")?
    Ok(bridge)
}
```

### Originate and record

```weft
use telecom

fn main -> Result {
    ari := telecom.ari_connect(null)

    // originate outbound call
    channel := telecom.ari_originate(ari, "PJSIP/1001", {
        "caller_id": "5551234",
    })?
    say("channel: ${channel.id}")

    // record
    telecom.ari_record(ari, channel.id, "voicemail-001", {
        "format": "wav",
        "max_duration": 120,
        "beep": true,
    })?

    // hold / mute
    telecom.ari_hold(ari, channel.id)?
    telecom.ari_mute(ari, channel.id, "in")?
}
```

---

## Environment variables

| Variable | Default | What it does |
|----------|---------|-------------|
| `WEFT_STT_URL` | — | STT service endpoint |
| `WEFT_STT_KEY` | — | STT API key |
| `WEFT_TTS_URL` | — | TTS service endpoint |
| `WEFT_TTS_KEY` | — | TTS API key |
| `FREESWITCH_HOST` | `127.0.0.1` | FreeSWITCH ESL host |
| `FREESWITCH_ESL_PORT` | `8021` | FreeSWITCH ESL port |
| `FREESWITCH_ESL_PASSWORD` | `ClueCon` | FreeSWITCH ESL password |
| `ASTERISK_HOST` | `127.0.0.1` | Asterisk ARI host |
| `ASTERISK_ARI_PORT` | `8088` | Asterisk ARI port |
| `ASTERISK_ARI_USER` | `asterisk` | Asterisk ARI username |
| `ASTERISK_ARI_PASSWORD` | `asterisk` | Asterisk ARI password |

---

## Full exports

47 functions across call flows, IVA, STT/TTS, DTMF, routing, dial plan, queues, CDR, FreeSWITCH ESL, and Asterisk ARI.

```bash
weft registry info telecom
```
