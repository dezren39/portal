package receiver

import (
	"fmt"
	"net"

	"github.com/gorilla/websocket"
	"github.com/schollz/pake/v3"
	"www.github.com/ZinoKader/portal/constants"
	"www.github.com/ZinoKader/portal/models"
	"www.github.com/ZinoKader/portal/models/protocol"
	"www.github.com/ZinoKader/portal/pkg/crypt"
	"www.github.com/ZinoKader/portal/tools"
)

func (r *Receiver) ConnectToRendezvous(password models.Password) error {

	// Establish websocket connection to rendezvous.
	wsConn, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://%s:%s/establish-receiver",
		constants.DEFAULT_RENDEZVOUZ_ADDRESS, constants.DEFAULT_RENDEZVOUZ_PORT), nil)
	if err != nil {
		return err
	}
	r.establishSecureConnection(wsConn, password)

	return nil
}

func (r *Receiver) doTransferHandshake(wsConn *websocket.Conn) (net.IP, int, error) {

	tcpAddr, _ := wsConn.LocalAddr().(*net.TCPAddr)
	msg := protocol.TransferMessage{
		Type: protocol.ReceiverHandshake,
		Payload: protocol.ReceiverHandshakePayload{
			IP: tcpAddr.IP,
		},
	}

	err := tools.WriteEncryptedMessage(wsConn, msg, r.crypt)
	if err != nil {
		return nil, 0, err
	}

	msg, err = tools.ReadEncryptedMessage(wsConn, r.crypt)
	if err != nil {
		return nil, 0, err
	}

	if msg.Type != protocol.SenderHandshake {
		return nil, 0, protocol.NewWrongMessageTypeError([]protocol.TransferMessageType{protocol.SenderHandshake}, msg.Type)
	}

	handshakePayload := protocol.SenderHandshakePayload{}
	err = tools.DecodePayload(msg.Payload, &handshakePayload)
	if err != nil {
		return nil, 0, err
	}

	return handshakePayload.IP, handshakePayload.Port, nil
}

func (r *Receiver) establishSecureConnection(wsConn *websocket.Conn, password models.Password) error {
	// Init curve in background.
	var p *pake.Pake
	pakeErr := make(chan error)
	go func() {
		var err error
		p, err = pake.InitCurve([]byte(password), 1, "p256")
		pakeErr <- err
	}()

	wsConn.WriteJSON(protocol.RendezvousMessage{
		Type: protocol.ReceiverToRendezvousEstablish,
		Payload: protocol.PasswordPayload{
			Password: tools.HashPassword(password),
		},
	})

	msg, err := tools.ReadRendevouzMessage(wsConn, protocol.ReceiverToRendezvousPAKE)
	if err != nil {
		return err
	}

	pakePayload := protocol.PakePayload{}
	err = tools.DecodePayload(msg.Payload, &pakePayload)
	if err != nil {
		return err
	}

	// check if we had an issue with the PAKE2 initialization error.
	if err = <-pakeErr; err != nil {
		return err
	}

	err = p.Update(pakePayload.Bytes)
	if err != nil {
		return err
	}

	wsConn.WriteJSON(protocol.RendezvousMessage{
		Type: protocol.ReceiverToRendezvousPAKE,
		Payload: protocol.PakePayload{
			Bytes: p.Bytes(),
		},
	})

	msg, err = tools.ReadRendevouzMessage(wsConn, protocol.RendezvousToReceiverSalt)
	if err != nil {
		return err
	}

	saltPayload := protocol.SaltPayload{}
	err = tools.DecodePayload(msg.Payload, &saltPayload)
	if err != nil {
		return err
	}

	r.crypt, err = crypt.New([]byte(password), saltPayload.Salt)
	return nil
}
