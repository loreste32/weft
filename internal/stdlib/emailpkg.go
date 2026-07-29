//go:build !js

package stdlib

import (
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/loreste/weft/internal/netsafe"
	"github.com/loreste/weft/internal/runtime"
)

// packageEmail — send/parse email (Python email + smtplib lite).
func packageEmail(env *runtime.Env) runtime.Value {
	p := pkg()

	// email.parse(raw) -> Result[{from,to,subject,body,headers}]
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("email.parse(raw)", "email"), nil
		}
		msg, err := mail.ReadMessage(strings.NewReader(args[0].String()))
		if err != nil {
			return errRes(err.Error(), "email"), nil
		}
		body, err := io.ReadAll(io.LimitReader(msg.Body, 8<<20))
		if err != nil {
			return errRes(err.Error(), "email"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k, v string) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.Str(v)
		}
		put("from", msg.Header.Get("From"))
		put("to", msg.Header.Get("To"))
		put("cc", msg.Header.Get("Cc"))
		put("subject", msg.Header.Get("Subject"))
		put("date", msg.Header.Get("Date"))
		put("body", string(body))
		hdrs := runtime.NewMap()
		hmo := hdrs.Obj.(*runtime.MapObj)
		for k, vs := range msg.Header {
			if len(vs) > 0 {
				hmo.Keys = append(hmo.Keys, k)
				hmo.Vals[k] = runtime.Str(vs[0])
			}
		}
		mo.Keys = append(mo.Keys, "headers")
		mo.Vals["headers"] = hdrs
		return runtime.Ok(m), nil
	}, 1)

	// email.build({from,to,subject,body,cc?,headers?}) -> str
	set(p, "build", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("email.build({from,to,subject,body})", "email"), nil
		}
		from := mapGetStr(args[0], "from", "")
		to := mapGetStr(args[0], "to", "")
		subject := mapGetStr(args[0], "subject", "")
		body := mapGetStr(args[0], "body", "")
		cc := mapGetStr(args[0], "cc", "")
		var b strings.Builder
		fmt.Fprintf(&b, "From: %s\r\n", from)
		fmt.Fprintf(&b, "To: %s\r\n", to)
		if cc != "" {
			fmt.Fprintf(&b, "Cc: %s\r\n", cc)
		}
		fmt.Fprintf(&b, "Subject: %s\r\n", subject)
		fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
		fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n")
		if hdrs, ok := mapGet(args[0], "headers"); ok && hdrs.Kind == runtime.KindMap {
			hmo := hdrs.Obj.(*runtime.MapObj)
			for _, k := range hmo.Keys {
				fmt.Fprintf(&b, "%s: %s\r\n", k, hmo.Vals[k].String())
			}
		}
		b.WriteString("\r\n")
		b.WriteString(body)
		return runtime.Str(b.String()), nil
	}, 1)

	// email.send(opts) -> Result
	set(p, "send", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("email.send({host,from,to,subject,body,...})", "email"), nil
		}
		opts := args[0]
		host := mapGetStr(opts, "host", mapGetStr(opts, "smtp", ""))
		if host == "" {
			host = envOr(env, "SMTP_HOST", "")
		}
		if host == "" {
			return errRes("email.send: host required (or SMTP_HOST)", "email"), nil
		}
		port := mapGetStr(opts, "port", envOr(env, "SMTP_PORT", "587"))
		from := mapGetStr(opts, "from", envOr(env, "SMTP_FROM", ""))
		subject := mapGetStr(opts, "subject", "")
		body := mapGetStr(opts, "body", "")
		user := mapGetStr(opts, "user", envOr(env, "SMTP_USER", ""))
		pass := mapGetStr(opts, "password", envOr(env, "SMTP_PASSWORD", ""))
		var to []string
		if t, ok := mapGet(opts, "to"); ok {
			switch t.Kind {
			case runtime.KindList:
				for _, it := range t.Obj.(*runtime.ListObj).Items {
					if s := strings.TrimSpace(it.String()); s != "" {
						to = append(to, s)
					}
				}
			default:
				for _, p := range strings.Split(t.String(), ",") {
					if s := strings.TrimSpace(p); s != "" {
						to = append(to, s)
					}
				}
			}
		}
		if from == "" || len(to) == 0 {
			return errRes("email.send: from and to required", "email"), nil
		}
		// Strip CR/LF/NUL so LLM/user text cannot inject SMTP headers (Bcc, etc.).
		from = smtpHeaderSafe(from)
		subject = smtpHeaderSafe(subject)
		for i := range to {
			to[i] = smtpHeaderSafe(to[i])
		}
		if from == "" || len(to) == 0 {
			return errRes("email.send: from and to required (after header sanitize)", "email"), nil
		}
		if err := netsafe.CheckHost(host); err != nil {
			return errRes("email.send blocked: "+err.Error(), "email"), nil
		}
		addr := net.JoinHostPort(host, port)
		msg := []byte(fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
			from, strings.Join(to, ", "), subject, time.Now().Format(time.RFC1123Z), body,
		))
		var auth smtp.Auth
		if user != "" {
			auth = smtp.PlainAuth("", user, pass, host)
		}
		if err := smtp.SendMail(addr, auth, from, to, msg); err != nil {
			return errRes(err.Error(), "email"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 1)

	return p
}

func envOr(env *runtime.Env, key, def string) string {
	if v, ok := getenv(env, key); ok && v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// smtpHeaderSafe strips CR/LF/NUL so values cannot inject SMTP headers.
func smtpHeaderSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return -1
		}
		return r
	}, s)
}
