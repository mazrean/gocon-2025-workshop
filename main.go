package main

import (
	"fmt"
	"io"
	"os"
)

const (
	stdinFileName  = "stdin.txt"
	stdoutFileName = "stdout.txt"
)

func main() {
	fmt.Fprintln(os.Stderr, "CacheProg used.")
	stdin, closeStdin, err := stdinReader()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := closeStdin(); err != nil {
			panic(err)
		}
	}()

	stdout, closeStdout, err := stdoutWriter()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := closeStdout(); err != nil {
			panic(err)
		}
	}()

	if err := CacheProg(stdin, stdout); err != nil {
		panic(err)
	}
}

func stdinReader() (io.Reader, func() error, error) {
	stdinFile, err := os.Create(stdinFileName)
	if err != nil {
		return nil, nil, err
	}

	return io.TeeReader(os.Stdin, stdinFile), stdinFile.Close, nil
}

func stdoutWriter() (io.Writer, func() error, error) {
	stdoutFile, err := os.Create(stdoutFileName)
	if err != nil {
		return nil, nil, err
	}

	return io.MultiWriter(os.Stdout, stdoutFile), stdoutFile.Close, nil
}
