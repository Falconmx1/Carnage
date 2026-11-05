package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageType identifica el tipo de mensaje del protocolo Carnage.
type MessageType uint8

const (
	// MsgHandshake es el saludo inicial entre Admin y Agent.
	MsgHandshake MessageType = 0x01
	// MsgHandshakeOK confirma el handshake exitoso.
	MsgHandshakeOK MessageType = 0x02
	// MsgCommand envía un comando del Admin al Agent.
	MsgCommand MessageType = 0x03
	// MsgCommandResult devuelve el resultado de un comando.
	MsgCommandResult MessageType = 0x04
	// MsgData transporta datos crudos (para túneles/proxies).
	MsgData MessageType = 0x05
	// MsgClose indica el cierre ordenado de la sesión.
	MsgClose MessageType = 0x06
	// MsgPing mantiene la sesión viva.
	MsgPing MessageType = 0x07
	// MsgPong responde al ping.
	MsgPong MessageType = 0x08
)

// String devuelve el nombre legible de un MessageType.
func (t MessageType) String() string {
	switch t {
	case MsgHandshake:
		return "HANDSHAKE"
	case MsgHandshakeOK:
		return "HANDSHAKE_OK"
	case MsgCommand:
		return "COMMAND"
	case MsgCommandResult:
		return "COMMAND_RESULT"
	case MsgData:
		return "DATA"
	case MsgClose:
		return "CLOSE"
	case MsgPing:
		return "PING"
	case MsgPong:
		return "PONG"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", uint8(t))
	}
}

// Envelope es la unidad de mensaje del protocolo.
type Envelope struct {
	Type    MessageType
	Payload []byte
}

// HeaderSize es el tamaño del encabezado: 1 byte tipo + 4 bytes longitud.
const HeaderSize = 5

// MaxPayloadSize es el tamaño máximo permitido de payload (16 MiB).
const MaxPayloadSize = 16 * 1024 * 1024

// Encode serializa el Envelope a bytes listos para escribir en el socket.
func (e *Envelope) Encode() ([]byte, error) {
	if len(e.Payload) > MaxPayloadSize {
		return nil, fmt.Errorf("payload demasiado grande: %d > %d", len(e.Payload), MaxPayloadSize)
	}
	buf := make([]byte, HeaderSize+len(e.Payload))
	buf[0] = byte(e.Type)
	binary.BigEndian.PutUint32(buf[1:HeaderSize], uint32(len(e.Payload)))
	copy(buf[HeaderSize:], e.Payload)
	return buf, nil
}

// WriteTo escribe el Envelope a un io.Writer.
func (e *Envelope) WriteTo(w io.Writer) (int64, error) {
	data, err := e.Encode()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// ReadFrom lee un Envelope desde un io.Reader.
func ReadFrom(r io.Reader) (*Envelope, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("error leyendo header: %w", err)
	}
	msgType := MessageType(header[0])
	length := binary.BigEndian.Uint32(header[1:HeaderSize])
	if length > MaxPayloadSize {
		return nil, fmt.Errorf("payload declarado demasiado grande: %d > %d", length, MaxPayloadSize)
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("error leyendo payload: %w", err)
		}
	}
	return &Envelope{Type: msgType, Payload: payload}, nil
}

// NewHandshake construye un mensaje de handshake con un identificador.
func NewHandshake(id string) *Envelope {
	return &Envelope{Type: MsgHandshake, Payload: []byte(id)}
}

// NewHandshakeOK construye la respuesta de handshake exitoso.
func NewHandshakeOK() *Envelope {
	return &Envelope{Type: MsgHandshakeOK}
}

// NewCommand construye un mensaje de comando.
func NewCommand(cmd string) *Envelope {
	return &Envelope{Type: MsgCommand, Payload: []byte(cmd)}
}

// NewData construye un mensaje de datos crudos.
func NewData(data []byte) *Envelope {
	return &Envelope{Type: MsgData, Payload: data}
}

// NewClose construye un mensaje de cierre.
func NewClose() *Envelope {
	return &Envelope{Type: MsgClose}
}
