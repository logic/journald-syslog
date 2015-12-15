journald-syslog is Copyright 2015 Ed Marshall. All rights reserved. Use of
this source code is governed by the terms of the GNU Gemeral Public License,
either version 3, or (at your option) any later version; please see the file
COPYING for more details.

journald-syslog is a simple syslog ingestor. It is designed to be used with
systemd socket activation to listen on the network for syslog packets in
either RFC3264 or RFC5424 format (or something roughly approaching those
formats), and injecting them into journald in a useful way.

This project depends on the systemd activation and journal code found at:

https://github.com/coreos/go-systemd/
