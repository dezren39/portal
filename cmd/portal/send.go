package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"www.github.com/ZinoKader/portal/constants"
	"www.github.com/ZinoKader/portal/models"
	"www.github.com/ZinoKader/portal/pkg/sender"
	"www.github.com/ZinoKader/portal/tools"
	"www.github.com/ZinoKader/portal/ui"
	senderui "www.github.com/ZinoKader/portal/ui/sender"
)

// handleSendCommand is the sender application.
func handleSendCommand(fileNames []string) {
	// communicate ui updates on this channel between senderClient and handleSendCommand.
	uiCh := make(chan sender.UIUpdate)
	// initialize a senderClient with a UI.
	senderClient := sender.WithUI(sender.NewSender(log.New(ioutil.Discard, "", 0)), uiCh)
	// initialize and start sender-UI.
	senderUI := senderui.NewSenderUI()
	// clean up temporary files previously created by this command.
	tools.RemoveTemporaryFiles(constants.SEND_TEMP_FILE_NAME_PREFIX)

	go initUI(senderUI)
	uiStartGraceTimeout := time.NewTimer(ui.START_PERIOD)
	<-uiStartGraceTimeout.C
	go listenForUIUpdates(senderUI, uiCh)

	closeFileCh := make(chan *os.File)
	senderReadyCh := make(chan bool, 1)
	// read, archive and compress files in parallel
	go prepareFiles(senderClient, senderUI, fileNames, senderReadyCh, closeFileCh)

	// initiate communications with rendezvous-server.
	startServerCh := make(chan sender.ServerOptions)
	relayCh := make(chan *websocket.Conn)
	passCh := make(chan models.Password)
	go initiateRendezvousCommunication(senderClient, senderUI, passCh, startServerCh, senderReadyCh, relayCh)
	// receive password and send to UI
	senderUI.Send(senderui.PasswordMsg{Password: string(<-passCh)})

	// keeps program alive
	doneCh := make(chan bool)
	// attach server to senderClient.
	senderClient = sender.WithServer(senderClient, <-startServerCh)

	// start sender-server to be able to respond to receiver direct-communication-probes
	go startDirectCommunicationServer(senderClient, senderUI, doneCh)
	// prepare a fallback to relay communications through rendezvous if direct communications unavailble
	prepareRelayCommunicationFallback(senderClient, senderUI, relayCh, doneCh)

	<-doneCh
	(<-closeFileCh).Close()
	ui.GracefulUIQuit(senderUI)
}

func initUI(senderUI *tea.Program) {
	if err := senderUI.Start(); err != nil {
		fmt.Println("Error initializing UI", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func listenForUIUpdates(senderUI *tea.Program, uiCh chan sender.UIUpdate) {
	latestProgress := 0
	for uiUpdate := range uiCh {
		// make sure progress is 100 if connection is to be closed
		if uiUpdate.State == sender.WaitForCloseMessage {
			latestProgress = 100
			senderUI.Send(ui.ProgressMsg{Progress: 1})
			continue
		}
		// limit progress update ui-send events
		newProgress := int(math.Ceil(100 * float64(uiUpdate.Progress)))
		if newProgress > latestProgress {
			latestProgress = newProgress
			senderUI.Send(ui.ProgressMsg{Progress: uiUpdate.Progress})
		}
	}
}

func prepareFiles(senderClient *sender.Sender, senderUI *tea.Program, fileNames []string, readyCh chan bool, closeFileCh chan *os.File) {
	files, err := tools.ReadFiles(fileNames)
	if err != nil {
		senderUI.Send(ui.ErrorMsg{Message: "Error reading files"})
		ui.GracefulUIQuit(senderUI)
	}
	uncompressedFileSize, err := tools.FilesTotalSize(files)
	if err != nil {
		senderUI.Send(ui.ErrorMsg{Message: "Error during file preparation"})
		ui.GracefulUIQuit(senderUI)
	}
	senderUI.Send(ui.FileInfoMsg{FileNames: fileNames, Bytes: uncompressedFileSize})

	tempFile, fileSize, err := tools.ArchiveAndCompressFiles(files)
	for _, file := range files {
		file.Close()
	}
	if err != nil {
		senderUI.Send(ui.ErrorMsg{Message: "Error compressing files"})
		ui.GracefulUIQuit(senderUI)
	}
	sender.WithPayload(senderClient, tempFile, fileSize)
	senderUI.Send(ui.FileInfoMsg{FileNames: fileNames, Bytes: fileSize})
	readyCh <- true
	senderUI.Send(senderui.ReadyMsg{})
	closeFileCh <- tempFile
}

func initiateRendezvousCommunication(senderClient *sender.Sender, senderUI *tea.Program, passCh chan models.Password,
	startServerCh chan sender.ServerOptions, readyCh chan bool, relayCh chan *websocket.Conn) {
	err := senderClient.ConnectToRendezvous(passCh, startServerCh, readyCh, relayCh)
	if err != nil {
		senderUI.Send(ui.ErrorMsg{Message: "Failed to communicate with rendezvous server"})
		ui.GracefulUIQuit(senderUI)
	}
}

func startDirectCommunicationServer(senderClient *sender.Sender, senderUI *tea.Program, doneCh chan bool) {
	if err := senderClient.StartServer(); err != nil {
		senderUI.Send(ui.ErrorMsg{Message: fmt.Sprintf("Something went wrong during file transfer: %e", err)})
		ui.GracefulUIQuit(senderUI)
	}
	doneCh <- true
}

func prepareRelayCommunicationFallback(senderClient *sender.Sender, senderUI *tea.Program, relayCh chan *websocket.Conn, doneCh chan bool) {
	if relayWsConn, closed := <-relayCh; closed {
		// close our direct-communication server and start transferring to the rendezvous-relay
		senderClient.CloseServer()
		go func() {
			if err := senderClient.Transfer(relayWsConn); err != nil {
				senderUI.Send(ui.ErrorMsg{Message: fmt.Sprintf("Something went wrong during file transfer: %e", err)})
				ui.GracefulUIQuit(senderUI)
			}
			doneCh <- true
		}()
	}
}
