package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/linanh/go-message/textproto"
	"github.com/linanh/go-milter"
)

func printAction(prefix string, act *milter.Action) {
	switch act.Code {
	case milter.ActAccept:
		log.Println(prefix, "accept")
	case milter.ActReject:
		log.Println(prefix, "reject")
	case milter.ActDiscard:
		log.Println(prefix, "discard")
	case milter.ActTempFail:
		log.Println(prefix, "temp. fail")
	case milter.ActReplyCode:
		log.Println(prefix, "reply code:", act.SMTPCode, act.SMTPText)
	case milter.ActContinue:
		log.Println(prefix, "continue")
	}
}

func printModifyAction(act milter.ModifyAction) {
	switch act.Code {
	case milter.ActAddHeader:
		log.Printf("add header: name %s, value %s", act.HeaderName, act.HeaderValue)
	case milter.ActInsertHeader:
		log.Printf("insert header: at %d, name %s, value %s", act.HeaderIndex, act.HeaderName, act.HeaderValue)
	case milter.ActChangeFrom:
		log.Printf("change from: %s %v", act.From, act.FromArgs)
	case milter.ActChangeHeader:
		log.Printf("change header: at %d, name %s, value %s", act.HeaderIndex, act.HeaderName, act.HeaderValue)
	case milter.ActReplBody:
		log.Println("replace body:", string(act.Body))
	case milter.ActAddRcpt:
		log.Println("add rcpt:", act.Rcpt)
	case milter.ActDelRcpt:
		log.Println("del rcpt:", act.Rcpt)
	case milter.ActQuarantine:
		log.Println("quarantine:", act.Reason)
	}
}

func main() {
	transport := flag.String("transport", "unix", "Transport to use for milter connection, One of 'tcp', 'unix', 'tcp4' or 'tcp6'")
	address := flag.String("address", "", "Transport address, path for 'unix', address:port for 'tcp'")
	hostname := flag.String("hostname", "localhost", "Value to send in CONNECT message")
	family := flag.String("family", string(milter.FamilyInet), "Protocol family to send in CONNECT message")
	port := flag.Uint("port", 2525, "Port to send in CONNECT message")
	connAddr := flag.String("conn-addr", "127.0.0.1", "Connection address to send in CONNECT message")
	helo := flag.String("helo", "localhost", "Value to send in HELO message")
	mailFrom := flag.String("from", "foxcpp@example.org", "Value to send in MAIL message")
	rcptTo := flag.String("rcpt", "foxcpp@example.com", "Comma-separated list of values for RCPT messages")
	actionMask := flag.Uint("actions",
		uint(milter.OptChangeBody|milter.OptChangeFrom|milter.OptChangeHeader|
			milter.OptAddHeader|milter.OptAddRcpt|milter.OptChangeFrom),
		"Bitmask value of actions we allow")
	disabledMsgs := flag.Uint("disabled-msgs", 0, "Bitmask of disabled protocol messages")
	flag.Parse()

	c := milter.NewClientWithOptions(*transport, *address, milter.ClientOptions{
		ActionMask:   milter.OptAction(*actionMask),
		ProtocolMask: milter.OptProtocol(*disabledMsgs),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
	defer c.Close()

	s, err := c.Session()
	if err != nil {
		log.Println(err)
		return
	}
	defer s.Close()

	act, err := s.Conn(*hostname, milter.ProtoFamily((*family)[0]), uint16(*port), *connAddr)
	if err != nil {
		log.Println(err)
		return
	}
	printAction("CONNECT:", act)
	if act.Code != milter.ActContinue {
		return
	}

	act, err = s.Helo(*helo)
	if err != nil {
		log.Println(err)
		return
	}
	printAction("HELO:", act)
	if act.Code != milter.ActContinue {
		return
	}

	act, err = s.Mail(*mailFrom, nil)
	if err != nil {
		log.Println(err)
		return
	}
	printAction("MAIL:", act)
	if act.Code != milter.ActContinue {
		return
	}

	for _, rcpt := range strings.Split(*rcptTo, ",") {
		act, err = s.Rcpt(rcpt, nil)
		if err != nil {
			log.Println(err)
			return
		}
		printAction("RCPT:", act)
		if act.Code != milter.ActContinue {
			return
		}
	}

	bufR := bufio.NewReader(os.Stdin)
	hdr, err := textproto.ReadHeader(bufR)
	if err != nil {
		log.Println("header parse:", err)
		return
	}

	act, err = s.Header(hdr)
	if err != nil {
		log.Println(err)
		return
	}
	printAction("HEADER:", act)
	if act.Code != milter.ActContinue {
		return
	}

	modifyActs, act, err := s.BodyReadFrom(bufR)
	if err != nil {
		log.Println(err)
		return
	}
	for _, act := range modifyActs {
		printModifyAction(act)
	}
	printAction("EOB:", act)
}
