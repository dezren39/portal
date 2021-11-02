package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"

	"www.github.com/ZinoKader/portal/models"
	"www.github.com/ZinoKader/portal/pkg/sender"
	"www.github.com/ZinoKader/portal/tools"
	"www.github.com/ZinoKader/portal/ui"
)

const SEND_COMMAND = "send"
const RECEIVE_COMMAND = "receive"

func main() {
	if len(os.Args) <= 1 {
		fmt.Printf("Usage: 'portal %s' to send files and 'portal %s [password]' to receive files\n", SEND_COMMAND, RECEIVE_COMMAND)
		return
	}

	sendCmd := flag.NewFlagSet(SEND_COMMAND, flag.ExitOnError)
	receiveCmd := flag.NewFlagSet(RECEIVE_COMMAND, flag.ExitOnError)

	switch os.Args[1] {

	case SEND_COMMAND:
		if len(os.Args) <= 2 {
			fmt.Println("Provide either one or more files/folder delimited by spaces, or a text string enclosed by quotes.")
			return
		}
		sendCmd.Parse(os.Args[2:])
		send(sendCmd.Args())

	case RECEIVE_COMMAND:
		receiveCmd.Parse(os.Args[2:])
		receive()

	default:
		fmt.Printf("Unrecognized command. Recognized commands: '%s' and '%s'.\n", SEND_COMMAND, RECEIVE_COMMAND)

	}
}

func send(fileNames []string) {
	// initialize sender
	// TODO: Add real logger, current logger doesn't log to avoid messing up the interactive UI
	sender := sender.NewSender(log.New(ioutil.Discard, "", 0))
	// initialize and start sender-UI
	senderUI := ui.NewSenderUI()
	go func() {
		if err := senderUI.Start(); err != nil {
			fmt.Println("Error initializing  UI", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	fileContentsBufferCh := make(chan *bytes.Buffer, 1)
	totalFileSizesCh := make(chan int64)
	senderReadyCh := make(chan bool, 1)
	// read, archive and compress files in parallel
	go func() {
		files, err := tools.ReadFiles(fileNames)
		if err != nil {
			fmt.Printf("Error reading file(s): %s\n", err.Error())
			return
		}
		fileSizesBytes, err := tools.FilesTotalSize(files)
		if err != nil {
			fmt.Printf("Error reading file size(s): %s\n", err.Error())
			return
		}
		totalFileSizesCh <- fileSizesBytes
		compressedBytes, err := tools.CompressFiles(files)
		for _, file := range files {
			file.Close()
		}
		if err != nil {
			fmt.Printf("Error compressing file(s): %s\n", err.Error())
			return // TODO: replace with graceful shutdown, this does nothing!
		}
		fileContentsBufferCh <- compressedBytes
		senderReadyCh <- true
		senderUI.Send(ui.ReadyMsg{})
	}()

	senderUI.Send(ui.FileInfoMsg{FileNames: fileNames, Bytes: <-totalFileSizesCh})

	// initiate communications with rendezvous-server
	passCh := make(chan models.Password)
	startServerCh := make(chan int)
	receiverIPCh := make(chan net.IP)
	go func() {
		err := sender.ConnectToRendezvous(passCh, startServerCh, senderReadyCh)
		if err != nil {
			fmt.Printf("Failed connecting to rendezvous server: %s\n", err.Error())
			return // TODO: replace with graceful shutdown, this does nothing!
		}
		senderPortCh <- senderPort
		receiverIPCh <- receiverIP
	}()

	senderUI.Send(ui.PasswordMsg{Password: string(<-passCh)})

	// send payload to receiver
	uiCh := make(chan sender.UIUpdate)
	fileContentsBuffer := <-fileContentsBufferCh
	senderServerPort := <-senderPortCh
	receiverIP := <-receiverIPCh

	/*s :=
	sender.WithUI(
		sender.WithServer(
			sender.NewSender(fileContentsBuffer, int64(fileContentsBuffer.Len()), receiverIP, throwawayLogger),
			senderServerPort),
		uiCh)
	*/

	go func() {
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
	}()

	s.Start()
}

func receive() {
}
