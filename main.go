package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"

	"github.com/felixge/fgprof"
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

	if err := os.MkdirAll("metrics/pprof", 0755); err != nil {
		panic(err)
	}

	stopCPUProfile, err := cpuProfiler()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := stopCPUProfile(); err != nil {
			panic(err)
		}
	}()

	stopFGProfile, err := fgProfiler()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := stopFGProfile(); err != nil {
			panic(err)
		}
	}()

	stopMemProfile, err := memProfiler()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := stopMemProfile(); err != nil {
			panic(err)
		}
	}()

	stopTrace, err := tracer()
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := stopTrace(); err != nil {
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

func cpuProfiler() (func() error, error) {
	f, err := os.Create("metrics/pprof/cpu.prof")
	if err != nil {
		return nil, err
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		return nil, fmt.Errorf("failed to start CPU profiling: %w", err)
	}

	return func() error {
		pprof.StopCPUProfile()

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close CPU profile file: %w", err)
		}

		return nil
	}, nil
}

func fgProfiler() (func() error, error) {
	f, err := os.Create("metrics/pprof/fg.prof")
	if err != nil {
		return nil, err
	}

	stop := fgprof.Start(f, fgprof.FormatPprof)

	return func() error {
		if err := stop(); err != nil {
			return fmt.Errorf("failed to stop fg profiling: %w", err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close CPU profile file: %w", err)
		}

		return nil
	}, nil
}

func memProfiler() (func() error, error) {
	f, err := os.Create("metrics/pprof/mem.prof")
	if err != nil {
		return nil, err
	}

	return func() error {
		if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
			return fmt.Errorf("failed to write memory profile: %w", err)
		}

		runtime.GC()

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close memory profile file: %w", err)
		}

		return nil
	}, nil
}

func tracer() (func() error, error) {
	f, err := os.Create("metrics/trace.out")
	if err != nil {
		return nil, err
	}

	if err := trace.Start(f); err != nil {
		return nil, fmt.Errorf("failed to start trace: %w", err)
	}

	return func() error {
		trace.Stop()

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close trace file: %w", err)
		}

		return nil
	}, nil
}
