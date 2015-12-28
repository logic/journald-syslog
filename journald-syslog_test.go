package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func TestParseSyslog(t *testing.T) {
	clock := clockwork.NewFakeClock()

	var PST *time.Location
	if timeParse, err := time.Parse("-07:00", "-08:00"); err == nil {
		PST = timeParse.Location()
	} else {
		t.Errorf("Could not set up timezone: %s", err.Error())
		return
	}

	var tests = []struct {
		buf      string
		source   string
		expected *SyslogMessage
	}{
		{
			`<13>1 2015-12-15T11:54:41.946675-08:00 host.domain.com user - - [timeQuality tzKnown="1" isSynced="1" syncAccuracy="380797"] message`,
			"127.0.0.1",
			&SyslogMessage{
				Version:        1,
				Facility:       1,
				Severity:       5,
				Timestamp:      time.Date(2015, 12, 15, 11, 54, 41, 946675000, PST),
				Hostname:       "host.domain.com",
				Tag:            "user - -",
				StructuredData: `[timeQuality tzKnown="1" isSynced="1" syncAccuracy="380797"`,
				Message:        "message",
				Source:         "127.0.0.1",
				clock:          clock,
			},
		},
		{
			`<13>Dec 15 11:55:02 host user: message`,
			"127.0.0.1",
			&SyslogMessage{
				Version:        0,
				Facility:       1,
				Severity:       5,
				Timestamp:      time.Date(0000, 12, 15, 11, 55, 02, 0, time.UTC),
				Hostname:       "host",
				Tag:            "user:",
				StructuredData: "",
				Message:        "message",
				Source:         "127.0.0.1",
				clock:          clock,
			},
		},
		{
			`<13>1 - host.domain.com user - - - message`,
			"127.0.0.1",
			&SyslogMessage{
				Version:        1,
				Facility:       1,
				Severity:       5,
				Timestamp:      clock.Now(),
				Hostname:       "",
				Tag:            "",
				StructuredData: "",
				Message:        "- host.domain.com user - - - message",
				Source:         "127.0.0.1",
				clock:          clock,
			},
		},
		{
			`<13>1 2015-12-15T11:56:01.776597-08:00 host.domain.com user - - - message`,
			"127.0.0.1",
			&SyslogMessage{
				Version:        1,
				Facility:       1,
				Severity:       5,
				Timestamp:      time.Date(2015, 12, 15, 11, 56, 01, 776597000, PST),
				Hostname:       "host.domain.com",
				Tag:            "user - -",
				StructuredData: "",
				Message:        "- message",
				Source:         "127.0.0.1",
				clock:          clock,
			},
		},
		{
			`<13>1 2015-12-15T11:56:13.555187-08:00 - user - - [timeQuality tzKnown="1" isSynced="1" syncAccuracy="426797"] message`,
			"127.0.0.1",
			&SyslogMessage{
				Version:        1,
				Facility:       1,
				Severity:       5,
				Timestamp:      time.Date(2015, 12, 15, 11, 56, 13, 555187000, PST),
				Hostname:       "-",
				Tag:            "user - -",
				StructuredData: `[timeQuality tzKnown="1" isSynced="1" syncAccuracy="426797"`,
				Message:        "message",
				Source:         "127.0.0.1",
				clock:          clock,
			},
		},
	}

	for num, test := range tests {
		msg := NewSyslogMessage()
		msg.Timestamp = clock.Now()
		msg.clock = clock
		msg.Parse(test.buf, test.source)
		if !reflect.DeepEqual(msg, test.expected) {
			t.Errorf("Failed test %d:\nOriginal: %s\nExpected: %v\n     Got: %v", num, test.buf, test.expected, msg)
		}
	}
}
