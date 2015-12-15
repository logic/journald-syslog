// Copyright 2015 Ed Marshall. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the COPYING file.

package main

import (
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/go-systemd/journal"
)

// RFC5424: MUST receive 408-octet messages, SHOULD accept 2048-octet messages
const PACKETSIZE = 2048

// SyslogMessage represents a completely-parsed syslog packet.
type SyslogMessage struct {
	Version        int
	Facility       int
	Severity       int
	Timestamp      string
	Hostname       string
	Tag            string
	StructuredData string
	Message        string
	Source         string
}

// ParseSyslog takes a syslog packet and source address as strings, and
// parses them into a SyslogMessage.
func ParseSyslog(buf string, source string) *SyslogMessage {
	// We're technically a relay, so per RFC3164, we're expected to fill
	// in a few defaults before passing the message along.
	msg := SyslogMessage{
		Version:   0,
		Facility:  0,
		Severity:  5,
		Timestamp: time.Now().UTC().String(),
		Hostname:  source,
		Source:    source,
	}

	rest := buf[:]

	// PRI
	if rest[0] == '<' {
		if priEnd := strings.IndexRune(rest, '>'); priEnd > 1 && priEnd < 5 {
			if pri, err := strconv.Atoi(rest[1:priEnd]); err == nil {
				msg.Facility = pri >> 3
				msg.Severity = pri & 7
				rest = rest[priEnd+1:]

				// VERSION
				if rest[0] == '1' {
					msg.Version = 1
					rest = rest[2:]

					// TIMESTAMP
					if tsEnd := strings.IndexRune(rest, ' '); tsEnd >= 0 {
						// Try a couple of RFC3339-compatible parsings.
						ts, err := time.Parse(time.RFC3339Nano, rest[:tsEnd])
						if err != nil {
							ts, err = time.Parse(time.RFC3339, rest[:tsEnd])
						}
						if err == nil {
							msg.Timestamp = ts.String()
							rest = rest[tsEnd+1:]

							// HOSTNAME, APP-NAME/PROCID/MSGID (TAG)
							if parts := strings.SplitN(rest, " ", 5); len(parts) == 5 {
								msg.Hostname = parts[0]
								msg.Tag = strings.Join(parts[1:4], " ")
								rest = parts[4]
							}

							// TODO: This is lame. Do proper structured data parsing
							// Make SyslogMessage.StructuredData a map[string]map[string]string,
							// populated as {SD-ID:{PARAM-NAME:PARAM-VALUE,...},...}.
							if rest[0] == '[' {
								if sdEnd := strings.IndexRune(rest, ']'); sdEnd > 1 {
									msg.StructuredData = rest[:sdEnd]
									rest = rest[sdEnd+2:]
								}
							}
						}
					}
				} else {
					// TIMESTAMP
					if ts, err := time.Parse(time.Stamp, rest[:15]); err == nil {
						msg.Timestamp = ts.String()
						rest = rest[16:]

						// HOSTNAME, TAG
						if parts := strings.SplitN(rest, " ", 3); len(parts) == 3 {
							msg.Hostname = parts[0]
							msg.Tag = parts[1]
							rest = parts[2]
						}
					}
				}
			}
		}
	}
	msg.Message = rest
	return &msg
}

// IngestMessage takes a syslog packet and source address as strings, and
// logs a parsed version of them to journald.
func IngestMessage(buf string, source string) {
	msg := ParseSyslog(buf, source)

	vars := map[string]string{
		"SYSLOG_VERSION":  strconv.Itoa(msg.Version),
		"SYSLOG_FACILITY": strconv.Itoa(msg.Facility),
		"SYSLOG_SEVERITY": strconv.Itoa(msg.Severity),

		// Without the hostname, the tag isn't a complete identifier.
		"SYSLOG_IDENTIFIER": strings.Join([]string{
			msg.Hostname, msg.Tag}, " "),
	}

	if len(msg.Timestamp) > 0 {
		vars["SYSLOG_TIMESTAMP"] = msg.Timestamp
	}

	if len(msg.Hostname) > 0 {
		vars["SYSLOG_HOSTNAME"] = msg.Hostname
	}

	if len(msg.Source) > 0 {
		vars["SYSLOG_SOURCE"] = msg.Source
	}

	// TODO: When structured data is actually stored in a structured form,
	// populate entries as SYSLOG_SD_<SD_ID>=<SD-PARAM ...>.
	if len(msg.StructuredData) > 0 {
		vars["SYSLOG_STRUCTURED_DATA"] = msg.StructuredData
	}

	err := journal.Send(msg.Message, journal.Priority(msg.Severity), vars)
	if err != nil {
		log.Println(err)
	}
}

// HandleListener takes a TCPListener socket (passed in from systemd) and
// repeatedly accepts new connections from it, handing the packets off for
// processing to IngestMessage.
func HandleListener(fd *net.TCPListener) {
	for {
		conn, err := fd.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()
			buf := make([]byte, PACKETSIZE)
			if count, err := conn.Read(buf); err != nil {
				log.Println(err)
			} else {
				addr := conn.RemoteAddr()
				IngestMessage(string(buf[:count]), addr.String())
			}
		}(conn)
	}
}

// HandlePacket takes a UDPConn socket (passed in from systemd) and repeatedly
// reads new packets from it, handing them off for processing to IngestMessage.
func HandlePacket(fd *net.UDPConn) {
	for {
		buf := make([]byte, PACKETSIZE)
		if count, addr, err := fd.ReadFromUDP(buf); err != nil {
			log.Println(err)
		} else {
			go IngestMessage(string(buf[:count]), addr.String())
		}
	}
}

func main() {
	packetConns, _ := activation.PacketConns(false)
	listeners, _ := activation.Listeners(false)
	if len(packetConns) == 0 && len(listeners) == 0 {
		log.Fatal("no UDP or TCP sockets supplied by systemd")
	}

	var wg sync.WaitGroup
	for _, fd := range packetConns {
		if conn, ok := fd.(*net.UDPConn); ok {
			wg.Add(1)
			go func(conn *net.UDPConn) {
				defer wg.Done()
				HandlePacket(conn)
			}(conn)
		}
	}
	for _, fd := range listeners {
		if conn, ok := fd.(*net.TCPListener); ok {
			wg.Add(1)
			go func(conn *net.TCPListener) {
				defer wg.Done()
				HandleListener(conn)
			}(conn)
		}
	}
	wg.Wait()
}
