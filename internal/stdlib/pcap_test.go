package stdlib_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func runPcap(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out, Stderr: &out})
	if err := ctx.RunSource(context.Background(), "pcap.weft", src); err != nil {
		t.Fatal(err, out.String())
	}
	return strings.TrimSpace(out.String())
}

func TestPcapWriteAndRead(t *testing.T) {
	pcapPath := filepath.Join(t.TempDir(), "test.pcap")
	src := `
use pcap
fn main -> Result {
    eth := pcap.ethernet({
        "dst": "ff:ff:ff:ff:ff:ff",
        "src": "00:11:22:33:44:55",
        "payload": pcap.ipv4({
            "src": "192.168.1.1",
            "dst": "10.0.0.1",
            "payload": pcap.tcp({
                "src_port": 12345,
                "dst_port": 80,
                "flags": "SYN",
            }),
        }),
    })
    pcap.write("` + pcapPath + `", [eth])?
    pkts := pcap.read("` + pcapPath + `")?
    say(len(pkts))
    say(pkts[0].len)
}
`
	out := runPcap(t, src)
	if !strings.Contains(out, "1") {
		t.Fatalf("expected 1 packet: %s", out)
	}

	// Verify file is valid pcap
	data, _ := os.ReadFile(pcapPath)
	if len(data) < 24 {
		t.Fatal("pcap too short")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0xa1b2c3d4 {
		t.Fatalf("bad magic: %08x", magic)
	}
}

func TestPcapUDP(t *testing.T) {
	pcapPath := filepath.Join(t.TempDir(), "udp.pcap")
	src := `
use pcap
fn main -> Result {
    pkt := pcap.ethernet({
        "dst": "ff:ff:ff:ff:ff:ff",
        "src": "00:11:22:33:44:55",
        "payload": pcap.ipv4({
            "src": "10.0.0.1",
            "dst": "10.0.0.2",
            "proto": 17,
            "payload": pcap.udp({
                "src_port": 53,
                "dst_port": 1234,
                "payload": "hello DNS",
            }),
        }),
    })
    pcap.write("` + pcapPath + `", [pkt])?
    say("ok")
}
`
	out := runPcap(t, src)
	if out != "ok" {
		t.Fatal(out)
	}
	st, _ := os.Stat(pcapPath)
	if st.Size() < 24 {
		t.Fatal("too small")
	}
}

func TestPcapRawPacket(t *testing.T) {
	pcapPath := filepath.Join(t.TempDir(), "raw.pcap")
	src := `
use pcap
fn main -> Result {
    raw := pcap.hex("ffffffffffff001122334455080045000028000000004006000ac0a80101c0a80102")
    pcap.write("` + pcapPath + `", [raw])?
    pkts := pcap.read("` + pcapPath + `")?
    say(len(pkts))
}
`
	out := runPcap(t, src)
	if out != "1" {
		t.Fatal(out)
	}
}

func TestPcapMultiplePackets(t *testing.T) {
	pcapPath := filepath.Join(t.TempDir(), "multi.pcap")
	src := `
use pcap
fn main -> Result {
    p1 := pcap.raw("hello")
    p2 := pcap.raw("world")
    p3 := pcap.raw("test")
    pcap.write("` + pcapPath + `", [p1, p2, p3])?
    pkts := pcap.read("` + pcapPath + `")?
    say(len(pkts))
}
`
	out := runPcap(t, src)
	if out != "3" {
		t.Fatal(out)
	}
}

func TestPcapPacketWithTimestamp(t *testing.T) {
	pcapPath := filepath.Join(t.TempDir(), "ts.pcap")
	src := `
use pcap
fn main -> Result {
    pkt := pcap.packet("hello", 1700000000)
    pcap.write("` + pcapPath + `", [pkt])?
    pkts := pcap.read("` + pcapPath + `")?
    say(pkts[0].len)
}
`
	out := runPcap(t, src)
	if out != "5" {
		t.Fatal(out)
	}
}

func TestPcapTCPFlags(t *testing.T) {
	src := `
use pcap
fn main {
    syn := pcap.tcp({"src_port": 1, "dst_port": 2, "flags": "SYN"})
    say(syn[13])
    synack := pcap.tcp({"src_port": 1, "dst_port": 2, "flags": "SYN|ACK"})
    say(synack[13])
    fin := pcap.tcp({"src_port": 1, "dst_port": 2, "flags": "FIN"})
    say(fin[13])
}
`
	out := runPcap(t, src)
	lines := strings.Split(out, "\n")
	if len(lines) < 3 {
		t.Fatal(out)
	}
	if lines[0] != "2" { // SYN = 0x02
		t.Fatalf("SYN flag: %s", lines[0])
	}
	if lines[1] != "18" { // SYN|ACK = 0x12
		t.Fatalf("SYN|ACK flag: %s", lines[1])
	}
	if lines[2] != "1" { // FIN = 0x01
		t.Fatalf("FIN flag: %s", lines[2])
	}
}

func TestPcapHex(t *testing.T) {
	src := `
use pcap
fn main {
    b := pcap.hex("48 65 6c 6c 6f")
    say(len(b))
}
`
	out := runPcap(t, src)
	if out != "5" {
		t.Fatal(out)
	}
}

func TestPcapReadBadFile(t *testing.T) {
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf, Stderr: &buf})
	err := ctx.RunSource(context.Background(), "t.weft", `
use pcap
fn main -> Result {
    pcap.read("/nonexistent")?
}`)
	if err == nil {
		t.Fatal("should error on missing file")
	}
}

func TestPcapReadNotPcap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pcap")
	os.WriteFile(path, []byte("not a pcap"), 0644)
	var buf bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &buf, Stderr: &buf})
	err := ctx.RunSource(context.Background(), "t.weft", `
use pcap
fn main -> Result {
    pcap.read("`+path+`")?
}`)
	if err == nil {
		t.Fatal("should error on bad pcap")
	}
}

func TestPcapEthernetDefaultType(t *testing.T) {
	src := `
use pcap
fn main {
    eth := pcap.ethernet({
        "dst": "ff:ff:ff:ff:ff:ff",
        "src": "00:11:22:33:44:55",
    })
    // byte 12-13 should be 0x0800 (IPv4)
    say(eth[12])
    say(eth[13])
}
`
	out := runPcap(t, src)
	lines := strings.Split(out, "\n")
	if lines[0] != "8" || lines[1] != "0" { // 0x08, 0x00
		t.Fatalf("ether type: %s", out)
	}
}
