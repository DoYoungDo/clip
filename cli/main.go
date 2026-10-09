package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	commandergo "github.com/DoYoungDo/commander-go"
	"golang.design/x/clipboard"
)

var (
	app_version = "0.0.1"
)

func readClipboardText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	text := string(data)
	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")
	return text, nil
}

func main() {
	app := commandergo.New("cli").
		Version(app_version)
	app.Command("list", "").
		ActionE(func(ctx *commandergo.Context) error {
			return nil
		})
	app.Command("use <id>", "").
		ActionE(func(ctx *commandergo.Context) error {
			return nil
		})
	app.Command("group", "").
		ActionE(func(ctx *commandergo.Context) error {
			return nil
		})
	app.ActionE(func(ctx *commandergo.Context) error {
		if err := clipboard.Init(); err != nil {
			return err
		}

		text, err := readClipboardText(os.Stdin)
		if err != nil {
			return err
		}
		clipboard.Write(clipboard.FmtText, []byte(text))
		return nil
	})
	if err := app.Parse(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}
}
