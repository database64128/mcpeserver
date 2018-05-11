package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"github.com/valyala/fasttemplate"
)

var replacer = strings.NewReplacer(
	"§0", "\033[30m", // black
	"§1", "\033[34m", // blue
	"§2", "\033[32m", // green
	"§3", "\033[36m", // aqua
	"§4", "\033[31m", // red
	"§5", "\033[35m", // purple
	"§6", "\033[33m", // gold
	"§7", "\033[37m", // gray
	"§8", "\033[90m", // dark gray
	"§9", "\033[94m", // light blue
	"§a", "\033[92m", // light green
	"§b", "\033[96m", // light aque
	"§c", "\033[91m", // light red
	"§d", "\033[95m", // light purple
	"§e", "\033[93m", // light yellow
	"§f", "\033[97m", // light white
	"§k", "\033[5m", // Obfuscated
	"§l", "\033[1m", // Bold
	"§m", "\033[2m", // Strikethrough
	"§n", "\033[4m", // Underline
	"§o", "\033[3m", // Italic
	"§r", "\033[0m", // Reset
	"[", "\033[1m[",
	"]", "]\033[22m",
	"(", "(\033[4m",
	")", "\033[24m)",
	"<", "\033[1m<",
	">", ">\033[22m",
)

func packOutput(input io.Reader, output func(string)) {
	reader := bufio.NewReader(input)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		output(strings.TrimRight(replacer.Replace(line), "\n"))
	}
}

func runImpl(base string, datapath string) (*os.File, func()) {
	abs, err := filepath.Abs(base)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(filepath.Join(abs, "server"))
	cmd.Env = append(os.Environ(), fmt.Sprintf("LD_LIBRARY_PATH=%s", abs))
	cmd.Dir = datapath
	f, err := pty.Start(cmd)
	if err != nil {
		panic(err)
	}
	return f, func() {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
	}
}

var upgrader = websocket.Upgrader{}

func run(base string, datapath string, ws string, prompt *fasttemplate.Template) {
	f, stop := runImpl(base, datapath)
	defer f.Close()
	defer stop()
	username := "nobody"
	hostname := "mcpeserver"
	{
		u, err := user.Current()
		if err == nil {
			username = u.Username
		}
		hn, err := os.Hostname()
		if err == nil {
			hostname = hn
		}
	}
	rl, _ := readline.NewEx(&readline.Config{
		Prompt: prompt.ExecuteString(map[string]interface{}{
			"username": username,
			"hostname": hostname,
			"esc":      "\033",
		}),
		HistoryFile:     ".readline-history",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "quit",

		HistorySearchFold: true,
		FuncFilterInputRune: func(r rune) (rune, bool) {
			if r == readline.CharCtrlZ {
				return r, false
			}
			return r, true
		},
	})
	defer rl.Close()
	lw := rl.Stdout()
	cache := 0
	go packOutput(f, func(text string) {
		if cache == 0 {
			fmt.Fprintf(lw, "\033[0m%s\033[0m\n", text)
		} else {
			cache--
		}
	})
	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}
		line = strings.TrimSpace(line)
		switch {
		default:
			cache++
			fmt.Fprintf(f, "%s\n", line)
		}
	}
}

func prepare(data string, link string) {
	games := filepath.Join(data, "games")
	props := filepath.Join(data, "server.properties")
	mods := filepath.Join(data, "mods")
	linkProps := filepath.Join(link, "server.properties")
	linkMods := filepath.Join(link, "mods")
	gamesProps := filepath.Join(games, "server.properties")
	gamesMods := filepath.Join(games, "mods")
	os.MkdirAll(link, os.ModePerm)
	os.MkdirAll(linkMods, os.ModePerm)
	if _, err := os.Stat(linkProps); os.IsNotExist(err) {
		f, err := os.OpenFile(linkProps, os.O_RDWR|os.O_CREATE, os.ModePerm)
		fmt.Fprintln(f, "motd=Minecraft Server\nlevel-dir=world\nlevel-name=Default Server World")
		if err != nil {
			panic(err)
		}
		if err = f.Close(); err != nil {
			panic(err)
		}
	}
	os.RemoveAll(games)
	os.Symlink(link, games)
	os.Symlink(gamesProps, props)
	os.Symlink(gamesMods, mods)
}
