package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/loreste/weft/pkg/weft"
)

func cmdInfo() int {
	fmt.Printf("weft %s (%s/%s)\n\n", weft.Version, runtime.GOOS, runtime.GOARCH)

	// system
	fmt.Println("== System ==")
	hostname, _ := os.Hostname()
	fmt.Printf("  hostname    %s\n", hostname)
	fmt.Printf("  os          %s\n", runtime.GOOS)
	fmt.Printf("  arch        %s\n", runtime.GOARCH)
	fmt.Printf("  cpus        %d\n", runtime.NumCPU())
	fmt.Printf("  go          %s\n", runtime.Version())
	fmt.Printf("  pid         %d\n", os.Getpid())
	if u := os.Getenv("USER"); u != "" {
		fmt.Printf("  user        %s\n", u)
	}
	if h := os.Getenv("HOME"); h != "" {
		fmt.Printf("  home        %s\n", h)
	}
	fmt.Println()

	// run a weft script to get sysinfo
	src := `fn main -> Result {
    mem := sysinfo.memory()?
    disk := sysinfo.disk("/")?
    up := sysinfo.uptime()?
    load := sysinfo.loadavg()?
    ifaces := sysinfo.net_interfaces()?

    say("== Memory ==")
    say("  total       ${mem.total} bytes")
    say("  used        ${mem.used} bytes")
    say("  available   ${mem.available} bytes")
    say("  percent     ${mem.percent}%")

    say("")
    say("== Disk (/) ==")
    say("  total       ${disk.total} bytes")
    say("  used        ${disk.used} bytes")
    say("  free        ${disk.free} bytes")
    say("  percent     ${disk.percent}%")

    say("")
    say("== Uptime ==")
    say("  seconds     ${up.seconds}")
    say("  human       ${up.human}")

    say("")
    say("== Load ==")
    say("  1m 5m 15m   $load")

    say("")
    say("== Network Interfaces ==")
    for iface in ifaces {
        if iface.up {
            addrs := str.join(iface.addrs, ", ")
            say("  ${iface.name}  mac=${iface.mac}  mtu=${iface.mtu}  $addrs")
        }
    }
}`

	ctx := weft.New(weft.Options{Stdout: os.Stdout, Stderr: os.Stderr})
	if err := ctx.RunSource(nil, "info.weft", src); err != nil {
		// if sysinfo fails (e.g. Wasm), show Go-level info
		fmt.Println("== Runtime ==")
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Printf("  go_alloc    %d MB\n", m.Alloc/1024/1024)
		fmt.Printf("  go_sys      %d MB\n", m.Sys/1024/1024)
		fmt.Printf("  goroutines  %d\n", runtime.NumGoroutine())
	}

	fmt.Println()
	fmt.Println("== Weft ==")
	fmt.Printf("  version     %s\n", weft.Version)
	fmt.Printf("  stdlib      %d packages\n", len(weft.StdlibNames()))
	fmt.Printf("  registry    %s\n", os.Getenv("WEFT_REGISTRY"))
	if r := os.Getenv("WEFT_REGISTRY"); r == "" {
		fmt.Printf("  registry    https://registry.weftproject.dev (default)\n")
	}

	// check services
	fmt.Println()
	fmt.Println("== Services ==")
	providers := []struct{ name, env string }{
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"deepgram", "DEEPGRAM_API_KEY"},
		{"elevenlabs", "ELEVENLABS_API_KEY"},
		{"ollama", "OLLAMA_HOST"},
		{"vllm", "VLLM_BASE_URL"},
	}
	for _, p := range providers {
		v := os.Getenv(p.env)
		if v != "" {
			if strings.Contains(strings.ToLower(p.env), "key") {
				fmt.Printf("  %-14s configured (%s=***)\n", p.name, p.env)
			} else {
				fmt.Printf("  %-14s %s\n", p.name, v)
			}
		}
	}

	fmt.Println()
	fmt.Printf("  time        %s\n", time.Now().Format(time.RFC3339))
	return 0
}
