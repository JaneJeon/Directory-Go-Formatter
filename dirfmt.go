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
	"strings"
	"time"
)

type pathfn func(string) error

const (
	eraseLine = "\033[K" // Return cursor to start of line and clean it
	lineUp = "\033[1A"   // Move cursor one line up
)

var (
	scanned int                 // number of files scanned
	fileCount int               // global counter
	fmtCount int                // counter for actual changes
	dirCount int                // counter for each directory
	args = []string{"-d", "-s"} // flags to be passed into gofmt
	dirs []string               // directories to search
	output = os.Stdout          // output for the gofmt changelog
	timeout time.Duration       // number of seconds to sleep
	width, _, _ = terminal.GetSize(int(os.Stdin.Fd()))
)

func main() {
	app := cli.NewApp()
	app.Name = "Systematic go formatter"
	app.Version = "0.1.0"
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
			Name: "sleep, s",
			Usage: "Sleep for `SECONDS` when each .go file is found. " +
				"Use for debugging or reading gofmt output.",
			Value: 0,
		},
		// TODO: parallelize the workload (in a non-blocking manner)
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

	app.Action = cliMain
	app.Run(os.Args)
}

func cliMain(c *cli.Context) (err error) {
	start := time.Now()

	// if no path is specified, uses working directory
	if c.NArg() == 0 {
		pwd, err := exec.Command("pwd").Output() // yupp
		if err != nil {
			return err
		}

		currDir := strings.TrimSpace(string(pwd))
		fmt.Println("Using current directory:", currDir)
		dirs = []string{currDir}
	}

	// TODO: allow use of file mode to allow running from IDE
	if width <= 0 {
		return errors.New("could not determine terminal width")
	}

	timeout = time.Second * time.Duration(c.Uint("sleep"))

	var spacer string // give newline for every directory after the first one

	if argout := c.String("file"); argout != "" {
		if output, err = getLog(argout); err != nil {
			return
		}
	}

	// modify flags to pass into gofmt based on whether it should modify the source code or not
	if !c.Bool("readonly") {
		args = append(args, "-w")
	}

	if dirs == nil {
		dirs = c.Args()
	}

	// go through each file/dir in the argument to process them
	for _, path := range dirs {
		fmt.Print(spacer)

		if err := handlePath(path, fmtFile, fmtDir); err != nil {
			fmt.Errorf("dirfmt: error processing %s: %s", path, err)
		}

		spacer = "\n"
	}

	// in read-only mode, no source code is modified
	if c.Bool("readonly") {
		fmtCount = 0
	}

	fmt.Printf("\n%d %s scanned, %d .go %s found, %d reformatted in %s.\n",
		scanned, sva("file", scanned != 1),
		fileCount, sva("file", fileCount != 1),
		fmtCount, time.Now().Sub(start))

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

// handles a path, whether it is a file or a directory
func handlePath(path string, filefn pathfn, dirfn pathfn) (err error) {
	f, err := os.Stat(path)
	if err != nil {
		return
	}

	// determine the nature of the path
	if mode := f.Mode(); mode.IsDir() {
		err = dirfn(path)
	} else {
		err = filefn(path)
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
func fmtFile(file string) error {
	scanned++

	// detecting go source code is done by naive extension comparison
	if file[len(file)-3:] != ".go" {
		return nil // just ignore non-go source files
	}

	time.Sleep(timeout) // give time to read output
	//inPlace(".go file found: " + file)
	fmt.Println(".go file found:", file)

	// increment global counters
	fileCount++
	dirCount++

	// run gofmt with specified flags
	out, err := exec.Command("gofmt", append(args, file)...).Output()
	if err != nil {
		return err
	}

	strout := string(out)

	// when source code is not modified, the output is empty
	if strout != "" {
		fmtCount++
	}

	output.WriteString(strout) // append any change log to the end of the output

	return nil
}

// run gofmt on all .go files in a directory and its subdirectories
func fmtDir(dir string) error {
	fmt.Printf("Scanning %s for .go files...\n", dir)

	dirCount = 0 // reset directory counter for every directory argument

	// walk through every file in the directory recursively and call gofmt on them
	err := filepath.Walk(dir, func(file string, f os.FileInfo, err error) error {
		return fmtFile(file)
	})

	// directory counter is only used for checking whether a directory is empty
	if dirCount == 0 {
		fmt.Println("No .go files were found in", dir)
	}

	return err
}

// TODO: consider interaction with gofmt output
// TODO: fix (probably has to do with last length)
// prints out strings "in place": i.e., without cluttering up the terminal
// references:
// https://stackoverflow.com/a/45422726
// https://stackoverflow.com/a/47170056
func inPlace(str string) {
	back := "\r" + eraseLine
	lines := (len(str) - 1) / width

	for i := 0; i < lines; i++ {
		back += lineUp + eraseLine
	}

	fmt.Print(back, str)
}
