package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"
)

var le = binary.LittleEndian

func attr(t uint16, p []byte) []byte {
	b := make([]byte, (len(p)+7)&^3)
	le.PutUint16(b, uint16(len(p)+4))
	le.PutUint16(b[2:], t)
	copy(b[4:], p)
	return b
}
func attrs(b []byte) (map[uint16][]byte, error) {
	m := map[uint16][]byte{}
	for len(b) >= 4 {
		n := int(le.Uint16(b))
		if n < 4 || n > len(b) {
			return nil, fmt.Errorf("invalid attribute")
		}
		m[le.Uint16(b[2:])&0x3fff] = b[4:n]
		a := (n + 3) &^ 3
		if a > len(b) {
			break
		}
		b = b[a:]
	}
	return m, nil
}
func request(fd int, typ uint16, seq uint32, cmd byte, payload []byte) (map[uint16][]byte, error) {
	b := make([]byte, 20+len(payload))
	le.PutUint32(b, uint32(len(b)))
	le.PutUint16(b[4:], typ)
	le.PutUint16(b[6:], 1)
	le.PutUint32(b[8:], seq)
	b[16] = cmd
	b[17] = 1
	copy(b[20:], payload)
	if err := syscall.Sendto(fd, b, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return nil, err
	}
	result := map[uint16][]byte{}
	for {
		buf := make([]byte, 8192)
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			return nil, err
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			if m.Header.Seq != seq {
				continue
			}
			if m.Header.Type == 3 {
				return result, nil
			}
			if m.Header.Type == 2 {
				if len(m.Data) < 4 {
					return nil, fmt.Errorf("short netlink error")
				}
				code := int32(le.Uint32(m.Data))
				if code != 0 {
					return nil, syscall.Errno(-code)
				}
				continue
			}
			if len(m.Data) < 4 {
				return nil, fmt.Errorf("short response")
			}
			a, err := attrs(m.Data[4:])
			if err != nil {
				return nil, err
			}
			for k, v := range a {
				result[k] = append(result[k], v...)
			}
			if m.Header.Flags&2 == 0 {
				return result, nil
			}
		}
	}
}
func imm(v uint64) uint64 { return 2<<16 | v }
func reg(v uint64) uint64 { return 1<<16 | v }
func ins(op, a, b, c uint64) []byte {
	p := make([]byte, 8)
	le.PutUint64(p, op|a<<10|b<<28|c<<46)
	return p
}

type Port struct {
	Port      int    `json:"port"`
	Link      bool   `json:"link"`
	SpeedCode uint32 `json:"speed_code"`
	Speed     int    `json:"speed_mbps"`
}

func run() error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, 16)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	if err = syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return err
	}
	syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: 3})
	family, err := request(fd, 16, 1, 3, attr(2, []byte("mdio\x00")))
	if err != nil {
		return err
	}
	if len(family[1]) != 2 {
		return fmt.Errorf("MDIO interface unavailable")
	}
	// These writes select an ASIC status address and issue the existing driver's READ command.
	// No ASIC configuration write command is used. The kernel locks the bus for this entire program.
	var prog []byte
	for _, i := range [][]byte{ins(1, imm(29), imm(21), reg(7)), ins(8, reg(7), 0, 0), ins(3, reg(7), imm(4), reg(0)), ins(7, reg(0), imm(0), imm(0)), ins(1, imm(29), imm(22), reg(4)), ins(1, imm(29), imm(23), reg(5)), ins(1, imm(29), imm(24), reg(6))} {
		prog = append(prog, i...)
	}
	for _, address := range []uint64{0x63e8, 0x63f0} {
		for _, i := range [][]byte{ins(1, imm(29), imm(21), reg(0)), ins(8, reg(0), 0, 0), ins(2, imm(29), imm(22), imm(address)), ins(2, imm(29), imm(21), imm(0x1b)), ins(1, imm(29), imm(21), reg(0)), ins(8, reg(0), 0, 0), ins(1, imm(29), imm(23), reg(1)), ins(8, reg(1), 0, 0), ins(1, imm(29), imm(24), reg(2)), ins(8, reg(2), 0, 0)} {
			prog = append(prog, i...)
		}
	}
	for _, i := range [][]byte{ins(2, imm(29), imm(22), reg(4)), ins(2, imm(29), imm(23), reg(5)), ins(2, imm(29), imm(24), reg(6))} {
		prog = append(prog, i...)
	}
	copy(prog[24:32], ins(7, reg(0), imm(0), imm(uint64(len(prog)/8-4))))
	payload := append(attr(1, []byte("mdio-bus\x00")), attr(3, prog)...)
	out, err := request(fd, le.Uint16(family[1]), 2, 1, payload)
	if err != nil {
		return err
	}
	if v := out[5]; len(v) >= 4 && int32(le.Uint32(v)) != 0 {
		return fmt.Errorf("MDIO transfer: %d", int32(le.Uint32(v)))
	}
	data := out[4]
	if len(data) != 36 {
		return fmt.Errorf("invalid status response: %d", len(data))
	}
	ctrl := le.Uint32(data)
	data = data[4:]
	var values [2]uint32
	for i := range values {
		p := data[i*16:]
		if le.Uint32(p)&4 != 0 || le.Uint32(p[4:])&4 != 0 {
			return fmt.Errorf("switch busy, retry")
		}
		values[i] = le.Uint32(p[8:]) | le.Uint32(p[12:])<<16
	}
	speeds := []int{10, 100, 1000, 500, 10000, 2500, 5000}
	ports := make([]Port, 8)
	for i := range ports {
		code := (values[1] >> uint(i*4)) & 15
		speed := 0
		if values[0]&(1<<uint(i)) != 0 && int(code) < len(speeds) {
			speed = speeds[code]
		}
		ports[i] = Port{i, values[0]&(1<<uint(i)) != 0, code, speed}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"ports": ports, "registers": values, "control": ctrl, "checked_at": time.Now().Format(time.RFC3339)})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
