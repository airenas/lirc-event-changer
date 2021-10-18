package lirc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	e, err := parseEvent("qwe 0 KEY_UP device")
	assert.Nil(t, err)
	assert.Equal(t, 0, e.repeat)
	assert.Equal(t, "qwe", e.id)
	assert.Equal(t, "KEY_UP", e.name)
	assert.Equal(t, "device", e.device)

	e, _ = parseEvent("qwe 1a KEY_UP device")
	assert.Equal(t, 26, e.repeat)
}

func TestToString(t *testing.T) {
	e := Event{id: "id", name: "NAME", device: "de", repeat: 10}
	assert.Equal(t, "id 0 NAME de", toString(&e))
}

