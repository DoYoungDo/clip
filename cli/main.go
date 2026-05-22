package main

import (
	"fmt"
	"io"
	"os"

	commandergo "github.com/DoYoungDo/commander-go"
	"golang.design/x/clipboard"
)

var (
	app_version = "0.0.1"
)

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

		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text := string(data)
		text = text[:len(text)-1]
		// fmt.Print(text)
		clipboard.Write(clipboard.FmtText, []byte(text))
		return nil
	})
	if err := app.Parse(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return
	}
}
