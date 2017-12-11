// TODO: Errorf prefix
// TODO: write tests
package main

import (
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type pathfn func(string, *os.File) error

var scanned int                 // number of files scanned
var filecount int               // global counter
var fmtcount int                // counter for actual changes
var dircount int                // counter for each directory
var timeout uint 				// number of seconds to sleep
var args = []string{"-d", "-s"} // flags to be passed into gofmt
var width, _, _ = terminal.GetSize(int(os.Stdin.Fd()))

func main() {
	app := cli.NewApp()
	app.Name = "Systematic go formatter"
	app.Version = "0.0.1"
	app.Compiled = time.Now()
	app.Authors = []cli.Author{
		{
			Name:  "Jane Jeon",
			Email: "jane@dartanon.org",
		},
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Usage: "Direct output to `FILE`",
		},
		cli.BoolFlag{
			Name:  "readonly, r",
			Usage: "Prevent modification of any .go files found",
		},
		cli.UintFlag{
			Name:  "sleep, s",
			Usage: "Sleep for `SECONDS` when each .go file is found. " +
				"Use for debugging or reading gofmt output.",
			Value: 0,
		},
		// TODO: parallelize the workload
		cli.UintFlag{
			Name:  "threads, t",
			Usage: "Number of `THREADS` to split the work",
			Value: 1,
		},
		// TODO: background process
		cli.BoolFlag{
			Name:  "daemon, d",
			Usage: "Run this as a background daemon process",
		},
	}

	app.Action = climain
	app.Run(os.Args)
}

func climain(c *cli.Context) (err error) {
	start := time.Now()

	if c.NArg() == 0 {
		return errors.New("you must supply at least one file or directory to check")
	}

	if width <= 0 {
		return errors.New("could not determine terminal width")
	}

	timeout = c.Uint("sleep")

	var spacer string   // give newline for every directory after the first one
	var output *os.File // output for the gofmt changelog

	if argout := c.String("file"); argout == "" {
		output = os.Stdout
	} else if output, err = getLog(argout); err != nil {
		return
	}

	// modify flags to pass into gofmt based on whether it should modify the source code or not
	if !c.Bool("readonly") {
		args = append(args, "-w")
	}

	// go through each file/dir in the argument to process them
	for _, path := range c.Args() {
		fmt.Print(spacer)

		if err := handlePath(path, output, fmtFile, fmtDir); err != nil {
			fmt.Errorf("dirfmt: error processing %s: %s", path, err)
		}

		spacer = "\n"
	}

	// in read-only mode, no source code is modified
	if c.Bool("readonly") {
		fmtcount = 0
	}

	fmt.Printf("\n%d %s scanned, %d .go %s found, %d reformatted in %s.\n",
		scanned, sva("file", scanned != 1),
		filecount, sva("file", filecount != 1),
		fmtcount, time.Now().Sub(start))

	return nil
}

// opens a log file and returns a pointer to it
func getLog(path string) (output *os.File, err error) {
	output, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)

	// logfile successfully opened
	if err == nil {
		defer output.Close() // automatically close once done
	}

	return
}

// handles a path, whether it is file, directory, or invalid
func handlePath(path string, output *os.File, filefn pathfn, dirfn pathfn) (err error) {
	f, err := os.Stat(path)
	if err != nil {
		return
	}

	// determine the nature of the path
	switch mode := f.Mode(); {
	case mode.IsRegular():
		err = filefn(path, output)
	case mode.IsDir():
		err = dirfn(path, output)
	}

	return // silently ignore invalid files, since we're not going to touch it
}

// subject-verb agreement
func sva(word string, plural bool) string {
	if plural {
		word += "s"
	}
	return word
}

// run gofmt on a single .go file
func fmtFile(file string, output *os.File) error {
	scanned++

	// detecting go source code is done by naive extension comparison
	if file[len(file)-3:] != ".go" {
		return nil // just ignore non-go source files
	}

	time.Sleep(time.Second * time.Duration(timeout))
	inplace(".go file found: " + file)

	// increment global counters
	filecount++
	dircount++

	// run gofmt with specified flags
	out, err := exec.Command("gofmt", append(args, file)...).Output()
	if err != nil {
		return err
	}

	strout := string(out)

	// when source code is not modified, the output is empty
	if strout != "" {
		fmtcount++
	}

	output.WriteString(strout) // append any change log to the end of the output

	return nil
}

// run gofmt on all .go files in a directory and its subdirectories
func fmtDir(dir string, output *os.File) (err error) {
	fmt.Printf("Scanning %s for .go files...\n", dir)

	dircount = 0 // reset directory counter for every directory argument

	// walk through every file in the directory recursively and call gofmt on them
	err = filepath.Walk(dir, func(file string, f os.FileInfo, err error) error {
		return fmtFile(file, output)
	})

	// directory counter is only used for checking whether a directory is empty
	if dircount == 0 {
		fmt.Println("No .go files were found in", dir)
	}

	return
}

// TODO: consider interaction with gofmt output
// prints out strings "in place": i.e., without cluttering up the terminal
// references:
// https://stackoverflow.com/a/45422726
// https://stackoverflow.com/a/47170056
func inplace(str string) {
	back := "\r\033[K"
	lines := (len(str) - 1) / width

	for i := 0; i < lines; i++ {
		back += "\033[1A\033[K"
	}

	fmt.Print(back, str)
}
