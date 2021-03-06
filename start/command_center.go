package start

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DarthSim/overmind/utils"
)

type commandCenter struct {
	processes processesMap
	output    *multiOutput
	listener  net.Listener
	stop      bool

	SocketPath string
}

func newCommandCenter(processes processesMap, socket string, output *multiOutput) (*commandCenter, error) {
	s, err := filepath.Abs(socket)

	return &commandCenter{
		processes: processes,
		output:    output,

		SocketPath: s,
	}, err
}

func (c *commandCenter) Start() (err error) {
	if c.listener, err = net.Listen("unix", c.SocketPath); err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			err = fmt.Errorf("it looks like Overmind is already running. If it's not, remove %s and try again", c.SocketPath)
		}
		return
	}

	c.output.WriteBoldLinef(nil, "Listening at %v", c.SocketPath)

	go func(c *commandCenter) {
		for {
			if conn, err := c.listener.Accept(); err == nil {
				go c.handleConnection(conn)
			}

			if c.stop {
				break
			}
		}
	}(c)

	return nil
}

func (c *commandCenter) Stop() {
	c.stop = true
	c.listener.Close()
}

func (c *commandCenter) handleConnection(conn net.Conn) {
	re := regexp.MustCompile(`\S+`)

	utils.ScanLines(conn, func(b []byte) bool {
		args := re.FindAllString(string(b), -1)

		if len(args) == 0 {
			return true
		}

		cmd := args[0]

		if len(args) > 1 {
			args = args[1:]
		} else {
			args = []string{}
		}

		switch cmd {
		case "restart":
			c.processRestart(cmd, args)
		case "stop":
			c.processStop(cmd, args)
		case "kill":
			c.processKill()
		case "get-connection":
			c.processGetConnection(cmd, args, conn)
		}

		return true
	})
}

func (c *commandCenter) processRestart(cmd string, args []string) {
	for name, p := range c.processes {
		if len(args) == 0 {
			p.Restart()
			continue
		}

		for _, pattern := range args {
			if utils.WildcardMatch(pattern, name) {
				p.Restart()
				break
			}
		}
	}
}

func (c *commandCenter) processStop(cmd string, args []string) {
	for name, p := range c.processes {
		if len(args) == 0 {
			p.Stop(true)
			continue
		}

		for _, pattern := range args {
			if utils.WildcardMatch(pattern, name) {
				p.Stop(true)
				break
			}
		}
	}
}

func (c *commandCenter) processKill() {
	for _, p := range c.processes {
		p.Kill(false)
	}
}

func (c *commandCenter) processGetConnection(cmd string, args []string, conn net.Conn) {
	if len(args) > 0 {
		if proc, ok := c.processes[args[0]]; ok {
			fmt.Fprintf(conn, "%s %s\n", proc.tmux.Socket, proc.WindowID())
		} else {
			fmt.Fprintln(conn, "")
		}
	}
}
