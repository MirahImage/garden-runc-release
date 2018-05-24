package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"network-rpc-plugin/command"
	networker "network-rpc-plugin/fake-networker"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"
)

var _ = Describe("Integration", func() {
	var (
		workDir    string
		cmd        *exec.Cmd
		init       *exec.Cmd
		receiver   chan networker.PluginInstruction
		session    *gexec.Session
		socketPath string
		reply      []byte
	)

	BeforeEach(func() {
		workDir = tempDir("", "")
		socketPath = filepath.Join(workDir, "test.sock")
		cmd = exec.Command(pluginPath, "--socket", socketPath)
		reply = []byte(`{"Here":"Be Dragons"}`)

		init = initCommand()
		Expect(init.Start()).To(Succeed())

		receiver = make(chan networker.PluginInstruction, 1)
		upInputs := &command.UpInputs{
			Pid: init.Process.Pid,
		}

		cmd.Stdin = strings.NewReader(encode(upInputs))
	})

	JustBeforeEach(func() {
		go func() {
			defer GinkgoRecover()
			networker.Listen(socketPath, receiver, reply)
		}()

		session = gexecStart(cmd)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(workDir)).To(Succeed())
		Expect(init.Process.Signal(unix.SIGTERM)).To(Succeed())
	})

	Describe("Up", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "up")
		})

		It("exits successfully", func() {
			Expect(session.Wait()).To(gexec.Exit(0))
		})

		It("sends the net ns fd of the provided pid to the socket", func() {
			var instruction networker.PluginInstruction
			Eventually(receiver).Should(Receive(&instruction))

			name, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", instruction.FD))
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal(parseNetNS(init.Process.Pid)))
		})

		It("sends the command to the provided socket", func() {
			var instruction networker.PluginInstruction
			Eventually(receiver).Should(Receive(&instruction))
			Expect(instruction.Message.Command).To(Equal("up"))
		})

		It("includes stdin contents in the message sent to the socket", func() {
			var instruction networker.PluginInstruction
			Eventually(receiver).Should(Receive(&instruction))
			Expect(instruction.Message.Data).To(Equal(fmt.Sprintf(`{"Pid":%d}`, init.Process.Pid)))
		})

		It("writes JSON to stdout", func() {
			Expect(session.Wait()).To(gexec.Exit())
			stdout := struct{}{}
			Expect(json.Unmarshal(session.Out.Contents(), &stdout)).To(Succeed())
		})

		It("writes the network daemon's response to stdout", func() {
			var instruction networker.PluginInstruction
			Eventually(receiver).Should(Receive(&instruction))
			Expect(session.Wait()).To(gexec.Exit())
			stdout := string(session.Out.Contents())
			Expect(strings.TrimSpace(stdout)).To(Equal(`{"Here":"Be Dragons"}`))
		})

		Context("when the network daemon reports an error", func() {
			BeforeEach(func() {
				reply = []byte(`{"Error":"no dragons received"}`)
			})

			It("writes the response to stderr", func() {
				Expect(session.Wait()).To(gexec.Exit())
				stderr := string(session.Err.Contents())
				Expect(stderr).To(ContainSubstring("no dragons received"))
			})

			It("exits non zero", func() {
				Expect(session.Wait()).NotTo(gexec.Exit(0))
			})
		})
	})

	Describe("Down", func() {
		BeforeEach(func() {
			cmd.Args = append(cmd.Args, "down")
		})

		It("exits successfully", func() {
			Expect(session.Wait()).To(gexec.Exit(0))
		})

		It("sends the command to the socket", func() {
			var instruction networker.PluginInstruction
			Eventually(receiver).Should(Receive(&instruction))
			Expect(instruction.Message.Command).To(Equal("down"))
		})
	})
})

func encode(thing interface{}) string {
	bytes, err := json.Marshal(thing)
	Expect(err).NotTo(HaveOccurred())
	return string(bytes)
}

func tempDir(dir, prefix string) string {
	name, err := ioutil.TempDir(dir, prefix)
	Expect(err).NotTo(HaveOccurred())
	return name
}

func parseNetNS(pid int) string {
	netNS, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/net", pid))
	Expect(err).NotTo(HaveOccurred())

	return strings.TrimSpace(netNS)
}

func initCommand() *exec.Cmd {
	cmd := exec.Command("sleep", "3600")
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: unix.CLONE_NEWUSER | unix.CLONE_NEWNET}
	return cmd
}
