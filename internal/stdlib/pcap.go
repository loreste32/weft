//go:build !js

package stdlib

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packagePcap builds and captures PCAP files for ops/security scripts.
//
//	pcap.write("out.pcap", packets)?
//	pcap.ethernet({dst: "ff:ff:ff:ff:ff:ff", src: "00:11:22:33:44:55", payload: ...})
//	pcap.ipv4({src: "10.0.0.1", dst: "10.0.0.2", proto: 6, payload: ...})
//	pcap.tcp({src_port: 12345, dst_port: 80, flags: "SYN", payload: "..."})
//	pcap.udp({src_port: 53, dst_port: 1234, payload: "..."})
//	pcap.raw(bytes_list)  — raw packet from byte values
//	pcap.read("in.pcap")? — read packets from file
func packagePcap() runtime.Value {
	p := pkg()

	// pcap.write(path, packets) -> Result
	set(p, "write", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 || args[1].Kind != runtime.KindList {
			return errRes("pcap.write(path, [packets])", "pcap"), nil
		}
		path := args[0].String()
		pkts := args[1].Obj.(*runtime.ListObj).Items
		if err := writePcap(path, pkts); err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return runtime.Ok(runtime.Int(int64(len(pkts)))), nil
	}, 2)

	// pcap.read(path) -> Result[[{ts, len, data}]]
	set(p, "read", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("pcap.read(path)", "pcap"), nil
		}
		pkts, err := readPcap(args[0].String())
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return runtime.Ok(runtime.List(pkts...)), nil
	}, 1)

	// pcap.ethernet(opts) -> bytes list
	set(p, "ethernet", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("pcap.ethernet({dst, src, type?, payload})", "pcap"), nil
		}
		b, err := buildEthernet(args[0])
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return bytesToList(b), nil
	}, 1)

	// pcap.ipv4(opts) -> bytes list
	set(p, "ipv4", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("pcap.ipv4({src, dst, proto?, ttl?, payload})", "pcap"), nil
		}
		b, err := buildIPv4(args[0])
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return bytesToList(b), nil
	}, 1)

	// pcap.tcp(opts) -> bytes list
	set(p, "tcp", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("pcap.tcp({src_port, dst_port, flags?, seq?, ack?, payload?})", "pcap"), nil
		}
		b, err := buildTCP(args[0])
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return bytesToList(b), nil
	}, 1)

	// pcap.udp(opts) -> bytes list
	set(p, "udp", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("pcap.udp({src_port, dst_port, payload?})", "pcap"), nil
		}
		b, err := buildUDP(args[0])
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return bytesToList(b), nil
	}, 1)

	// pcap.raw(bytes_or_str) -> bytes list
	set(p, "raw", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("pcap.raw(data)", "pcap"), nil
		}
		b := toBytes(args[0])
		return bytesToList(b), nil
	}, 1)

	// pcap.packet(data, ts?) -> packet map  wraps raw data with timestamp
	set(p, "packet", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("pcap.packet(data, ts?)", "pcap"), nil
		}
		data := toBytes(args[0])
		ts := time.Now()
		if len(args) >= 2 {
			if n, err := runtime.AsInt(args[1]); err == nil {
				ts = time.Unix(n, 0)
			}
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"ts", "len", "data"}
		mo.Vals["ts"] = runtime.Float(float64(ts.UnixNano()) / 1e9)
		mo.Vals["len"] = runtime.Int(int64(len(data)))
		mo.Vals["data"] = bytesToList(data)
		return m, nil
	}, 2)

	// pcap.hex(str) -> bytes list  parse hex string "48656c6c6f" or "48 65 6c"
	set(p, "hex", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("pcap.hex(hex_string)", "pcap"), nil
		}
		b, err := parseHexStr(args[0].String())
		if err != nil {
			return errRes(err.Error(), "pcap"), nil
		}
		return bytesToList(b), nil
	}, 1)

	return p
}

// --- PCAP file I/O ---

const (
	pcapMagic   = 0xa1b2c3d4
	pcapVerMaj  = 2
	pcapVerMin  = 4
	pcapSnapLen = 65535
	pcapLinkEth = 1
)

func writePcap(path string, packets []runtime.Value) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// global header
	hdr := make([]byte, 24)
	binary.LittleEndian.PutUint32(hdr[0:], pcapMagic)
	binary.LittleEndian.PutUint16(hdr[4:], pcapVerMaj)
	binary.LittleEndian.PutUint16(hdr[6:], pcapVerMin)
	// timezone, sigfigs = 0
	binary.LittleEndian.PutUint32(hdr[16:], pcapSnapLen)
	binary.LittleEndian.PutUint32(hdr[20:], pcapLinkEth)
	if _, err := f.Write(hdr); err != nil {
		return err
	}

	now := time.Now()
	for i, pkt := range packets {
		data, ts := extractPacketData(pkt, now.Add(time.Duration(i)*time.Microsecond))
		if len(data) == 0 {
			continue
		}
		rec := make([]byte, 16)
		binary.LittleEndian.PutUint32(rec[0:], uint32(ts.Unix()))
		binary.LittleEndian.PutUint32(rec[4:], uint32(ts.Nanosecond()/1000))
		binary.LittleEndian.PutUint32(rec[8:], uint32(len(data)))
		binary.LittleEndian.PutUint32(rec[12:], uint32(len(data)))
		if _, err := f.Write(rec); err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func extractPacketData(v runtime.Value, fallbackTS time.Time) ([]byte, time.Time) {
	switch v.Kind {
	case runtime.KindList:
		return listToBytes(v), fallbackTS
	case runtime.KindStr:
		return []byte(v.S), fallbackTS
	case runtime.KindMap:
		mo := v.Obj.(*runtime.MapObj)
		ts := fallbackTS
		if tsv, ok := mo.Vals["ts"]; ok {
			if tsv.Kind == runtime.KindFloat {
				sec := int64(tsv.F)
				nsec := int64((tsv.F - float64(sec)) * 1e9)
				ts = time.Unix(sec, nsec)
			} else if n, err := runtime.AsInt(tsv); err == nil {
				ts = time.Unix(n, 0)
			}
		}
		if d, ok := mo.Vals["data"]; ok {
			return toBytes(d), ts
		}
		return nil, ts
	default:
		return nil, fallbackTS
	}
}

func readPcap(path string) ([]runtime.Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 24 {
		return nil, fmt.Errorf("pcap too short")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != pcapMagic {
		return nil, fmt.Errorf("not a pcap file (magic %08x)", magic)
	}
	off := 24
	var pkts []runtime.Value
	for off+16 <= len(data) {
		tsSec := binary.LittleEndian.Uint32(data[off:])
		tsUsec := binary.LittleEndian.Uint32(data[off+4:])
		inclLen := binary.LittleEndian.Uint32(data[off+8:])
		off += 16
		if off+int(inclLen) > len(data) {
			break
		}
		pktData := data[off : off+int(inclLen)]
		off += int(inclLen)

		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		mo.Keys = []string{"ts", "len", "data"}
		mo.Vals["ts"] = runtime.Float(float64(tsSec) + float64(tsUsec)/1e6)
		mo.Vals["len"] = runtime.Int(int64(inclLen))
		mo.Vals["data"] = bytesToList(pktData)
		pkts = append(pkts, m)
	}
	return pkts, nil
}

// --- packet builders ---

func buildEthernet(opts runtime.Value) ([]byte, error) {
	mo := opts.Obj.(*runtime.MapObj)
	dst, err := parseMAC(pcapStr(mo.Vals["dst"], ""))
	if err != nil {
		return nil, fmt.Errorf("dst: %w", err)
	}
	src, err := parseMAC(pcapStr(mo.Vals["src"], ""))
	if err != nil {
		return nil, fmt.Errorf("src: %w", err)
	}
	etherType := uint16(0x0800) // IPv4 default
	if v, ok := mo.Vals["type"]; ok {
		if n, err := runtime.AsInt(v); err == nil {
			etherType = uint16(n)
		}
	}

	frame := make([]byte, 14)
	copy(frame[0:6], dst)
	copy(frame[6:12], src)
	binary.BigEndian.PutUint16(frame[12:], etherType)

	if payload, ok := mo.Vals["payload"]; ok {
		frame = append(frame, toBytes(payload)...)
	}
	return frame, nil
}

func buildIPv4(opts runtime.Value) ([]byte, error) {
	mo := opts.Obj.(*runtime.MapObj)
	srcIP := net.ParseIP(pcapStr(mo.Vals["src"], "0.0.0.0")).To4()
	dstIP := net.ParseIP(pcapStr(mo.Vals["dst"], "0.0.0.0")).To4()
	if srcIP == nil || dstIP == nil {
		return nil, fmt.Errorf("invalid IP address")
	}

	proto := byte(6) // TCP default
	if v, ok := mo.Vals["proto"]; ok {
		if n, err := runtime.AsInt(v); err == nil {
			proto = byte(n)
		}
	}
	ttl := byte(64)
	if v, ok := mo.Vals["ttl"]; ok {
		if n, err := runtime.AsInt(v); err == nil {
			ttl = byte(n)
		}
	}

	var payload []byte
	if v, ok := mo.Vals["payload"]; ok {
		payload = toBytes(v)
	}

	totalLen := 20 + len(payload)
	hdr := make([]byte, 20)
	hdr[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(hdr[2:], uint16(totalLen))
	hdr[8] = ttl
	hdr[9] = proto
	copy(hdr[12:16], srcIP)
	copy(hdr[16:20], dstIP)

	// checksum
	binary.BigEndian.PutUint16(hdr[10:], ipChecksum(hdr))

	return append(hdr, payload...), nil
}

func buildTCP(opts runtime.Value) ([]byte, error) {
	mo := opts.Obj.(*runtime.MapObj)
	srcPort := mapGetInt(opts, "src_port", 0)
	dstPort := mapGetInt(opts, "dst_port", 0)
	seq := mapGetInt(opts, "seq", 0)
	ack := mapGetInt(opts, "ack", 0)

	flags := byte(0)
	if v, ok := mo.Vals["flags"]; ok {
		flags = parseTCPFlags(v.String())
	}

	var payload []byte
	if v, ok := mo.Vals["payload"]; ok {
		payload = toBytes(v)
	}

	hdr := make([]byte, 20)
	binary.BigEndian.PutUint16(hdr[0:], uint16(srcPort))
	binary.BigEndian.PutUint16(hdr[2:], uint16(dstPort))
	binary.BigEndian.PutUint32(hdr[4:], uint32(seq))
	binary.BigEndian.PutUint32(hdr[8:], uint32(ack))
	hdr[12] = 0x50 // data offset 5 (20 bytes), no options
	hdr[13] = flags
	binary.BigEndian.PutUint16(hdr[14:], 65535) // window

	return append(hdr, payload...), nil
}

func buildUDP(opts runtime.Value) ([]byte, error) {
	srcPort := mapGetInt(opts, "src_port", 0)
	dstPort := mapGetInt(opts, "dst_port", 0)

	var payload []byte
	if v, ok := opts.Obj.(*runtime.MapObj).Vals["payload"]; ok {
		payload = toBytes(v)
	}

	udpLen := 8 + len(payload)
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint16(hdr[0:], uint16(srcPort))
	binary.BigEndian.PutUint16(hdr[2:], uint16(dstPort))
	binary.BigEndian.PutUint16(hdr[4:], uint16(udpLen))
	// checksum 0 (optional for IPv4)

	return append(hdr, payload...), nil
}

// --- helpers ---

func parseMAC(s string) ([]byte, error) {
	if s == "" {
		return make([]byte, 6), nil
	}
	hw, err := net.ParseMAC(s)
	if err != nil {
		return nil, err
	}
	return hw, nil
}

func parseTCPFlags(s string) byte {
	var flags byte
	for _, part := range strings.Split(strings.ToUpper(s), "|") {
		part = strings.TrimSpace(part)
		switch part {
		case "FIN":
			flags |= 0x01
		case "SYN":
			flags |= 0x02
		case "RST":
			flags |= 0x04
		case "PSH":
			flags |= 0x08
		case "ACK":
			flags |= 0x10
		case "URG":
			flags |= 0x20
		}
	}
	return flags
}

func ipChecksum(hdr []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(hdr); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(hdr[i:]))
	}
	if len(hdr)%2 == 1 {
		sum += uint32(hdr[len(hdr)-1]) << 8
	}
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return ^uint16(sum)
}

func parseHexStr(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd hex length")
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &b[i/2])
		if err != nil {
			return nil, fmt.Errorf("bad hex at %d: %s", i, s[i:i+2])
		}
	}
	return b, nil
}

func pcapStr(v runtime.Value, def string) string {
	if v.Kind == runtime.KindStr {
		return v.S
	}
	return def
}

func toBytes(v runtime.Value) []byte {
	switch v.Kind {
	case runtime.KindStr:
		return []byte(v.S)
	case runtime.KindList:
		return listToBytes(v)
	default:
		return []byte(v.String())
	}
}

func listToBytes(v runtime.Value) []byte {
	lo := v.Obj.(*runtime.ListObj)
	b := make([]byte, len(lo.Items))
	for i, it := range lo.Items {
		if n, err := runtime.AsInt(it); err == nil {
			b[i] = byte(n)
		}
	}
	return b
}

func bytesToList(b []byte) runtime.Value {
	items := make([]runtime.Value, len(b))
	for i, v := range b {
		items[i] = runtime.Int(int64(v))
	}
	return runtime.List(items...)
}
