package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
)

var (
	from string
	to   string
	mtu  int
)

func proxy(from net.Conn) {
	defer from.Close()

	conn, err := net.Dial("udp", to)
	if err != nil {
		log.Printf("Failed to dial %s: %s", to, err)
		return
	}
	defer conn.Close()

	chErr := make(chan error)
	go func() {
		lenBuf := make([]byte, 2)
		buf := make([]byte, mtu)
		for {
			n, err := io.ReadFull(from, lenBuf)
			if err != nil {
				chErr <- fmt.Errorf("Failed to read full, %d != 2: %s", n, err)
				return
			}

			bufLen := int(lenBuf[0])<<8 + int(lenBuf[1])
			if bufLen > mtu {
				chErr <- fmt.Errorf("Message too large, %d > %d", bufLen, mtu)
				return
			}

			if n, err = io.ReadFull(from, buf[0:bufLen]); err != nil {
				chErr <- fmt.Errorf("Failed to read full, %d != %d: %s", n, bufLen, err)
				return
			}

			if n, err = conn.Write(buf[0:bufLen]); err != nil {
				chErr <- fmt.Errorf("Failed to write: %s", n, bufLen, err)
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, mtu+2)
		for {
			bufLen, err := conn.Read(buf[2:])
			if err != nil {
				chErr <- fmt.Errorf("Failed to read: %s", err)
				return
			}
			buf[0] = byte(bufLen >> 8)
			buf[1] = byte(bufLen)

			if n, err := from.Write(buf[0 : bufLen+2]); err != nil {
				chErr <- fmt.Errorf("Failed to write, %d != %d: %s", n, bufLen+2, err)
				return
			}
		}
	}()

	err = <-chErr
	log.Printf("Error on proxy: %s", err)
}

func main() {
	flag.StringVar(&from, "from", "0.0.0.0:1194", "tcp listen address")
	flag.StringVar(&to, "to", "localhost:1194", "openvpn udp address")
	flag.IntVar(&mtu, "mtu", 1500, "maximum MTU")
	flag.Parse()

	ln, err := net.Listen("tcp", from)
	if err != nil {
		log.Fatalf("Failed to listen %s: %s", from, err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("Failed to accept: %s", err)
		}

		go proxy(conn)
	}
}
