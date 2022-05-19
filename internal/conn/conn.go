package conn

import (
	"encoding/json"

	"github.com/SpatiumPortae/portal/protocol/rendezvous"
	"github.com/SpatiumPortae/portal/protocol/transfer"
	"github.com/gorilla/websocket"
)

// Conn is an interface that wraps a network connection.
type Conn interface {
	Write([]byte) error
	Read() ([]byte, error)
}

// ------------------ Conn implementations ------------------

// WS is a wrapper around a websocket connection.
type WS struct {
	Conn *websocket.Conn
}

func (ws *WS) Write(payload []byte) error {
	return ws.Conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (ws *WS) Read() ([]byte, error) {
	_, payload, err := ws.Conn.ReadMessage()
	return payload, err
}

// ------------------ Rendezvous Conn ------------------------

// Rendezvous specifies a connection to the rendezvous server.
type Rendezvous struct {
	Conn Conn
}

// WriteMsg writes a rendezvous message to the underlying connection.
func (r Rendezvous) WriteMsg(msg rendezvous.Msg) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return r.Conn.Write(payload)
}

// ReadMsg reads a rendezvous message from the underlying connection.
func (r Rendezvous) ReadMsg(expected ...rendezvous.MsgType) (rendezvous.Msg, error) {
	b, err := r.Conn.Read()
	if err != nil {
		return rendezvous.Msg{}, err
	}
	var msg rendezvous.Msg
	if err := json.Unmarshal(b, &msg); err != nil {
		return rendezvous.Msg{}, err
	}
	if len(expected) != 0 && expected[0] != msg.Type {
		return rendezvous.Msg{}, rendezvous.Error{Expected: expected, Got: msg.Type}
	}
	return msg, nil
}

// ------------------ Transfer Conn ----------------------------

// Transfer specifies a encrypted connection safe to transfer files over.
type Transfer struct {
	Conn  Conn
	crypt crypt
}

// TransferFromSession returns a secure connection using the provided session key
// and salt.
func TransferFromSession(conn Conn, sessionkey, salt []byte) Transfer {
	return Transfer{
		Conn:  conn,
		crypt: NewCrypt(sessionkey, salt),
	}
}

// TransferFromKey returns a secure connection using the provided cryptographic key.
func TransferFromKey(conn Conn, key []byte) Transfer {
	return Transfer{
		Conn:  conn,
		crypt: crypt{Key: key},
	}
}

// Key returns the cryptographic key associated with this connection.
func (tc Transfer) Key() []byte {
	return tc.crypt.Key
}

// WriteMsg encrypts and writes the specified transfer message to the underlying connection.
func (t Transfer) WriteMsg(msg transfer.Msg) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return t.WriteBytes(b)
}

// ReadMsg reads and encrypts the specified transfer message to the underlying connection.
func (t Transfer) ReadMsg(expected ...transfer.MsgType) (transfer.Msg, error) {
	dec, err := t.ReadBytes()
	if err != nil {
		return transfer.Msg{}, err
	}
	var msg transfer.Msg
	if err = json.Unmarshal(dec, &msg); err != nil {
		return transfer.Msg{}, err
	}

	if len(expected) != 0 && expected[0] != msg.Type {
		return transfer.Msg{}, transfer.Error{Expected: expected, Got: msg.Type}
	}
	return msg, nil
}

// WriteBytes encrypts and writes the specified bytes to the underlying connection.
func (t Transfer) WriteBytes(b []byte) error {
	enc, err := t.crypt.Encrypt(b)
	if err != nil {
		return nil
	}
	return t.Conn.Write(enc)
}

// ReadBytes reads and decrypts bytes from the underlying connection.
func (t Transfer) ReadBytes() ([]byte, error) {
	b, err := t.Conn.Read()
	if err != nil {
		return nil, err
	}
	return t.crypt.Decrypt(b)
}
