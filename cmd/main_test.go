package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	e, err := parseEvent("qwe 0 KEY_UP device")
	assert.Nil(t, err)
	assert.Equal(t, 0, e.repeat)
	assert.Equal(t, "qwe", e.id)
	assert.Equal(t, "KEY_UP", e.name)
	assert.Equal(t, "device", e.de)

	e, _ = parseEvent("qwe 1a KEY_UP device")
	assert.Equal(t, 26, e.repeat)
}

func TestToString(t *testing.T) {
	e := event{id: "id", name: "NAME", de: "de", repeat: 10}
	tm := time.Now()
	d := wdata{event: &e, at: tm, last: tm}

	assert.Equal(t, "id 0 NAME de", toString(&d))
	d.repeat = 17
	assert.Equal(t, "id 0 NAME de", toString(&d))
	d.last = tm.Add(time.Millisecond*300)
	assert.Equal(t, "id 0 NAME de", toString(&d))
	d.last = tm.Add(time.Millisecond*400)
	assert.Equal(t, "id 0 NAME_HOLD de", toString(&d))
}

func TestNeedSend(t *testing.T) {
	tm := time.Now()
	e := &event{id: "id", name: "NAME", de: "de", repeat: 10}
	wd := &wdata{event: e, at: tm.Add(-100*time.Millisecond), last: tm.Add(-100*time.Millisecond), repeat: 0}
	assert.False(t, needSend(wd, tm))
	wd.at = tm.Add(-510*time.Millisecond)
	assert.True(t, needSend(wd, tm))
	wd.at = tm.Add(-490*time.Millisecond)
	assert.False(t, needSend(wd, tm))
}
