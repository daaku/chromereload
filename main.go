// Command chromereload reloads the current tab.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

type reload struct {
	IgnoreCache bool `json:"ignoreCache"`
}

type app struct {
	host        string
	port        int
	ignoreCache bool
}

func (a *app) debuggerURL() string {
	return fmt.Sprintf("http://%s:%d/json", a.host, a.port)
}

func (a *app) wsURL() (string, error) {
	res, err := http.Get(a.debuggerURL())
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer res.Body.Close()

	var tabs []struct {
		Type                 string
		WebSocketDebuggerURL string `json:"WebSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tabs); err != nil {
		return "", errors.WithStack(err)
	}
	for _, t := range tabs {
		if t.Type == "page" {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", errors.New("no open tabs")
}

func (a *app) reload() error {
	wsURL, err := a.wsURL()
	if err != nil {
		return err
	}

	origin := fmt.Sprintf("http://%s/", a.host)
	ws, err := websocket.Dial(wsURL, "", origin)
	if err != nil {
		return errors.WithStack(err)
	}
	defer ws.Close()

	reloadCmd, err := json.Marshal(map[string]interface{}{
		"id":     0,
		"method": "Page.reload",
		"params": reload{IgnoreCache: a.ignoreCache},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := ws.Write(reloadCmd); err != nil {
		return errors.WithStack(err)
	}

	msg := make([]byte, 1024)
	n, err := ws.Read(msg)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Printf("response: %s\n", msg[:n])
	return nil
}

func run() error {
	var a app
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&a.host, "host", "localhost", "remote debugger host")
	fs.IntVar(&a.port, "port", 9222, "remote debugger port")
	fs.BoolVar(&a.ignoreCache, "ignore-cache", false, "ignore cache when reloading")
	fs.Parse(os.Args[1:])
	return a.reload()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}
