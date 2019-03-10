package commands

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"regexp"
	"strings"

	"github.com/mjarkk/go-ps"
	"github.com/sirupsen/logrus"
)

// Listener is the the type that handles is the callback for server responses
type Listener int

// InputQuestion is what is send by the lazygit client
type InputQuestion struct {
	PublicKey string
	Question  string
	Listener  string
}

// listenerMetaType is a listener there private key and ask function
type listenerMetaType struct {
	AskFunction func(string) string
	AskedFor    struct {
		Password bool
		Username bool
	}
}

type prompt struct {
	Pattern  string
	AskedFor *bool
}

var listenerMeta = map[string]listenerMetaType{} // a list of listeners
var totalListener uint32                         // this gets used to set the key of listenerMeta

// Input interacts with the lazygit client spawned by git
func (l *Listener) Input(in InputQuestion, out *EncryptedMessage) error {
	suspiciousErr := errors.New("closing message due to suspicious behavior")

	listener, ok := listenerMeta[in.Listener]
	if !ok {
		return suspiciousErr
	}

	if !HasLGAsSubProcess() {
		return suspiciousErr
	}

	updateListenerMeta := func() {
		listenerMeta[in.Listener] = listener
	}

	updateListenerMeta()

	question := in.Question

	prompts := map[string]prompt{
		"password": {Pattern: `Password\s*for\s*'.+':`, AskedFor: &listener.AskedFor.Password},
		"username": {Pattern: `Username\s*for\s*'.+':`, AskedFor: &listener.AskedFor.Username},
	}

	var toSend string

	for askFor, prompt := range prompts {
		match, _ := regexp.MatchString(prompt.Pattern, question)
		if match && !*prompt.AskedFor {
			*prompt.AskedFor = true
			updateListenerMeta()
			toSend = strings.Replace(listener.AskFunction(askFor), "\n", "", -1)
			break
		}
	}

	encryptedData, err := encryptMessage(in.PublicKey, toSend)
	if err != nil {
		return suspiciousErr
	}

	*out = encryptedData

	return nil
}

// RunWithCredentialListener runs git commands that need credentials
// ask() gets executed when git needs credentials
// The ask argument will be "username" or "password"
func (c *OSCommand) RunWithCredentialListener(command string, ask func(string) string) error {
	totalListener++
	currentListener := fmt.Sprintf("%v", totalListener)

	listener := listenerMetaType{AskFunction: ask}
	listenerMeta[currentListener] = listener

	defer delete(listenerMeta, currentListener)

	end := make(chan error)
	hostPort := GetFreePort()
	serverStartedChan := make(chan struct{})

	go c.runGit(
		serverStartedChan,
		end,
		command,
		hostPort,
		currentListener,
	)

	go runServer(
		serverStartedChan,
		end,
		hostPort,
		currentListener,
	)

	err := <-end

	return err
}

func (c *OSCommand) runGit(serverStartedChan chan struct{}, end chan error, command, hostPort, currentListener string) {
	<-serverStartedChan

	ex, err := os.Executable()
	if err != nil {
		ex = os.Args[0]
	}

	cmd := c.ExecutableFromString(command)
	cmd.Env = append(
		os.Environ(),
		"LAZYGIT_CLIENT_COMMAND=GET_CREDENTIAL",
		"LAZYGIT_HOST_PORT="+hostPort,
		"LAZYGIT_LISTENER="+currentListener,
		"GIT_ASKPASS="+ex, // tell git where lazygit is located so it can ask lazygit for credentials
		"LANG=en_US.UTF-8",
		"LC_ALL=en_US.UTF-8",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		outString := string(out)
		if len(outString) == 0 {
			end <- err
			return
		}
		end <- errors.New(outString)
		return
	}
	end <- nil
}

// runServer starts the server that waits for events from the lazygit client
func runServer(serverStartedChan chan struct{}, end chan error, hostPort, currentListener string) {
	serverRunning := false

	addy, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+hostPort)
	if err != nil {
		end <- err
		return
	}

	inbound, err := net.ListenTCP("tcp", addy)
	if err != nil {
		end <- err
		return
	}

	go func() {
		<-end
		if serverRunning {
			inbound.Close()
		}
	}()

	listener := new(Listener)

	// every listener needs a different name it this is not dune rpc.RegisterName will error
	err = rpc.RegisterName("Listener"+currentListener, listener)
	if err != nil {
		end <- err
		return
	}

	serverStartedChan <- struct{}{}

	serverRunning = true
	rpc.Accept(inbound)
	serverRunning = false
}

// GetFreePort returns a free port that can be used by lazygit
func GetFreePort() string {
	checkFrom := 5000
	for {
		checkFrom++
		check := fmt.Sprintf("%v", checkFrom)
		if IsFreePort(check) {
			return check
		}
	}
}

// IsFreePort return true if the port if not in use
func IsFreePort(port string) bool {
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err == nil {
		go conn.Close()
		return false
	}
	return true
}

// SetupClient sets up the client
// This will be called if lazygit is called through git
func SetupClient(log *logrus.Entry) {
	port := os.Getenv("LAZYGIT_HOST_PORT")
	listener := os.Getenv("LAZYGIT_LISTENER")

	privateKey, publicKey, err := generateKeyPair()
	if err != nil {
		log.Errorln(err)
		return
	}

	client, err := rpc.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Errorln(err)
		return
	}

	var data *EncryptedMessage
	err = client.Call("Listener"+listener+".Input", InputQuestion{
		Question:  os.Args[len(os.Args)-1],
		Listener:  listener,
		PublicKey: publicKey,
	}, &data)
	client.Close()
	if err != nil {
		log.Errorln(err)
		return
	}

	msg, err := decryptMessage(privateKey, *data)
	if err != nil {
		log.Errorln(err)
		return
	}

	fmt.Println(msg)
}

// HasLGAsSubProcess returns true if lazygit is a child of this process
func HasLGAsSubProcess() bool {
	if !ps.Supported() {
		return true
	}

	lgHostPid := os.Getpid()
	list, err := ps.Processes()
	if err != nil {
		return false
	}
procListLoop:
	for _, proc := range list {
		procName := proc.Executable()
		if procName != "lazygit" && procName != "lazygit.exe" {
			continue
		}
		parrent := proc.PPid()
		for {
			if parrent < 30 {
				continue procListLoop
			}
			proc, err := ps.FindProcess(parrent)
			if err != nil {
				continue procListLoop
			}
			if proc.Pid() == lgHostPid {
				return true
			}
			ex := proc.Executable()
			if !strings.Contains(ex, "git") && !strings.Contains(ex, "GIT") {
				continue procListLoop
			}
			parrent = proc.PPid()
		}
	}
	return false
}