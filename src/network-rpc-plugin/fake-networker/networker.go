package networker

import (
	"encoding/json"
	"io"
	"net"
	"network-rpc-plugin/message"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

type PluginInstruction struct {
	FD      uintptr
	Message message.Message
}

func Listen(socketPath string, receiveCh chan<- PluginInstruction, output []byte) {
	listener := listenOnSocket(socketPath)
	conn, err := listener.Accept()
	Expect(err).NotTo(HaveOccurred())

	receiveCh <- PluginInstruction{
		FD:      getFD(conn),
		Message: decodeMsg(conn),
	}

	var n int
	n, err = conn.Write(output)
	Expect(n).To(Equal(len(output)))
	Expect(err).NotTo(HaveOccurred())
}

func getFD(conn net.Conn) uintptr {
	unixconn, ok := conn.(*net.UnixConn)
	Expect(ok).To(BeTrue(), "failed to cast connection to unixconn")

	return recvFD(unixconn)
}

func recvFD(conn *net.UnixConn) uintptr {
	controlMessageBytesSpace := unix.CmsgSpace(4)

	controlMessageBytes := make([]byte, controlMessageBytesSpace)
	_, readSocketControlMessageBytes, _, _, err := conn.ReadMsgUnix(nil, controlMessageBytes)
	Expect(err).NotTo(HaveOccurred())

	if readSocketControlMessageBytes > controlMessageBytesSpace {
		Fail("received too many things!!")
	}

	controlMessageBytes = controlMessageBytes[:readSocketControlMessageBytes]

	socketControlMessages := parseSocketControlMessage(controlMessageBytes)

	Expect(socketControlMessages).To(HaveLen(1))
	fds := parseUnixRights(&socketControlMessages[0])

	Expect(fds).To(HaveLen(1))
	return uintptr(fds[0])
}

func decodeMsg(r io.Reader) message.Message {
	var content message.Message
	decoder := json.NewDecoder(r)
	Expect(decoder.Decode(&content)).To(Succeed())
	return content
}

func parseUnixRights(m *unix.SocketControlMessage) []int {
	messages, err := unix.ParseUnixRights(m)
	Expect(err).NotTo(HaveOccurred())
	return messages
}

func parseSocketControlMessage(b []byte) []unix.SocketControlMessage {
	messages, err := unix.ParseSocketControlMessage(b)
	Expect(err).NotTo(HaveOccurred())
	return messages
}

func listenOnSocket(socketPath string) net.Listener {
	ln, err := net.Listen("unix", socketPath)
	Expect(err).NotTo(HaveOccurred())
	return ln
}
