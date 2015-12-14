/*
 * Webrtc chat demo.
 * Send chat messages via webrtc, over go.
 * Can interop with the JS client. (Open chat.html in a browser)
 *
 * To use: `go run chat.go`
 */
package main

// #cgo LDFLAGS: -L../lib
import "C"
import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/keroserene/go-webrtc"
	"github.com/keroserene/go-webrtc/data"
	"os"
	"strings"
)

type Mode int

const (
	ModeInit Mode = iota
	ModeConnect
	ModeChat
)

var pc *webrtc.PeerConnection
var dc *data.Channel
var mode Mode
var err error
var username = "go-Alice"

// Signaling channel can be copy paste.
func signalSend(msg string) {
	fmt.Println("\n ---- Please copy the below to peer ---- \n")
	fmt.Println(msg + "\n")
}

type (
	// TODO: Put json stuff into go-webrtc.
	Description struct {
		Type string `json:"type"`
		Sdp  string `json:"sdp"`
	}
	Message struct {
		Desc Description `json:"desc"`
	}
)

func signalReceive(msg string) {
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(msg), &parsed)
	if nil != err {
		fmt.Println(err, ", try again.")
		return;
	}
	if nil != parsed["desc"] {
		data := parsed["desc"].(map[string]interface{})
		receiveDescription(Description{
			data["type"].(string),
			data["sdp"].(string),
		})
	}
	if nil != parsed["candidate"] {
		ice := webrtc.IceCandidate{
			parsed["candidate"].(string),
			parsed["sdpMid"].(string),
			int(parsed["sdpMLineIndex"].(float64)),
		}
		pc.AddIceCandidate(ice)
		fmt.Println("ICE candidate successfully received.")
	}

	// TODO: Only do this once it's a valid SDP.
	if nil == pc {
		start(false)
	}
}

func sendOffer() {
	offer, err := pc.CreateOffer()
	if err != nil {
		fmt.Println(err)
		return
	}
	pc.SetLocalDescription(offer)
	data := Description{
		"offer",
		offer.Description,
		// "",
	}
	bytes, err := json.Marshal(data)
	message := fmt.Sprintf(`{"desc":%s}`, string(bytes))
	signalSend(message)
}

func sendAnswer() {
	answer, err := pc.CreateAnswer()
	if err != nil {
		fmt.Println(err)
		return
	}
	pc.SetLocalDescription(answer)
	data := Description{
		"answer",
		answer.Description,
		// "",
	}
	bytes, err := json.Marshal(data)
	message := fmt.Sprintf(`{"desc":%s}`, string(bytes))
	signalSend(message)
}

func receiveDescription(desc Description) {
	sdp := webrtc.NewSessionDescription(desc.Type, desc.Sdp)
	if nil == sdp {
		fmt.Println("Invalid SDP.")
		return;
	}
	pc.SetRemoteDescription(sdp)
	fmt.Println("SDP " + desc.Type + " successfully received.")
	if "offer" == desc.Type {
		fmt.Println("Replying with answer.")
		go sendAnswer()
	}
}

func prepareDataChannel(channel *data.Channel) {
	channel.OnOpen = func() {
		fmt.Println("Data Channel opened!")
		mode = ModeChat
		fmt.Println("------- chat enabled! -------")
	}
	channel.OnClose = func() {
		fmt.Println("Data Channel closed.")
		fmt.Println("------- chat disabled -------")
	}
	channel.OnMessage = func(msg []byte) {
		fmt.Println(string(msg))
	}
}

func sendChat(msg string) {
	line := username + ": " + msg
	fmt.Println(line)
	dc.Send([]byte(line))
}

func start(instigator bool) {
	mode = ModeConnect
	fmt.Println("Starting up PeerConnection...")
	config := webrtc.NewConfiguration(
		webrtc.OptionIceServer("stun:stun.l.google.com:19302"))
	pc, err = webrtc.NewPeerConnection(config)
	if nil != err {
		fmt.Println("Failed to create PeerConnection.")
		return
	}

	// The below three callbacks are the minimum required.
	pc.OnNegotiationNeeded = func() {
		go sendOffer()
	}
	pc.OnIceCandidate = func(candidate webrtc.IceCandidate) {
		bytes, _ := json.Marshal(candidate)
		signalSend(string(bytes))
	}
	pc.OnDataChannel = func(channel *data.Channel) {
		fmt.Println("Datachannel established...", channel)
		prepareDataChannel(channel)
	}

	if instigator {
		// Attempting to create the first datachannel triggers ICE.
		fmt.Println("Trying to create a datachannel.")
		dc, err = pc.CreateDataChannel("init", data.Init{})
		if nil != err {
			fmt.Println("Unexpected failure creating data.Channel.")
			return
		}
		prepareDataChannel(dc)
	}
}

func main() {
	webrtc.SetVerbosity(1)
	reader := bufio.NewReader(os.Stdin)

	mode = ModeInit
	wait := make(chan int, 1)
	fmt.Println("=== go-webrtc chat demo ===")
	fmt.Println("What is your username?")
	username, _ = reader.ReadString('\n')
	username = strings.TrimSpace(username)
	fmt.Println("Welcome, " + username + "!")
	fmt.Println("To initiate a WebRTC PeerConnection, type \"start\".")
	fmt.Println("(Alternatively, immediately input SDP messages from the peer.)")

	// Input loop.
	for true {
		text, _ := reader.ReadString('\n')
		switch mode {
		case ModeInit:
			if strings.HasPrefix(text, "start") {
				start(true)
			} else {
				signalReceive(text)
			}
		case ModeConnect:
			signalReceive(text)
		case ModeChat:
			sendChat(text)
			break
		}
		text = ""
	}
	<-wait
	fmt.Println("done")
}
