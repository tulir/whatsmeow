package waLog

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

// newLogFile ensures that loggers in these tests get their own unique file.
var nr int

func newFile() string {
	f := fmt.Sprintf("/tmp/logger_test_%d.log", nr)
	nr++
	return f
}

// TestSub tests the propagation of sub module names.
func TestSub(t *testing.T) {
	for _, test := range []struct {
		existing string
		new      string
		want     string
	}{
		{
			// Empty names
			existing: "",
			new:      "",
			want:     "",
		},
		{
			// No new name
			existing: "existing",
			new:      "",
			want:     "existing",
		},
		{
			// No existing name
			existing: "",
			new:      "new",
			want:     "new",
		},
		{
			// Both existing and new
			existing: "existing",
			new:      "new",
			want:     "existing/new",
		},
	} {
		if got := sub(test.existing, test.new); got != test.want {
			t.Errorf("sub(%q, %q) = %q, want %q", test.existing, test.new, got, test.want)
		}
	}
}

// TestShouldOutput tests the comparison of the verbosity level of a logger vs. that of a message.
func TestShouldOutput(t *testing.T) {
	for _, test := range []struct {
		loggerLevel  int
		messageLevel string
		shouldLog    bool
	}{
		// Unknown level, all should log
		{
			loggerLevel:  -1,
			messageLevel: DebugLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  -1,
			messageLevel: InfoLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  -1,
			messageLevel: WarnLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  -1,
			messageLevel: ErrorLevel,
			shouldLog:    true,
		},
		// DEBUG level, all should log
		{
			loggerLevel:  0,
			messageLevel: DebugLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  0,
			messageLevel: InfoLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  0,
			messageLevel: WarnLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  0,
			messageLevel: ErrorLevel,
			shouldLog:    true,
		},
		// INFO level, all bug DEBUG should log
		{
			loggerLevel:  1,
			messageLevel: DebugLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  1,
			messageLevel: InfoLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  1,
			messageLevel: WarnLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  1,
			messageLevel: ErrorLevel,
			shouldLog:    true,
		},
		// WARN level, all bug DEBUG/INFO should log
		{
			loggerLevel:  2,
			messageLevel: DebugLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  2,
			messageLevel: InfoLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  2,
			messageLevel: WarnLevel,
			shouldLog:    true,
		},
		{
			loggerLevel:  2,
			messageLevel: ErrorLevel,
			shouldLog:    true,
		},
		// ERROR level, only ERROR should log
		{
			loggerLevel:  3,
			messageLevel: DebugLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  3,
			messageLevel: InfoLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  3,
			messageLevel: WarnLevel,
			shouldLog:    false,
		},
		{
			loggerLevel:  3,
			messageLevel: ErrorLevel,
			shouldLog:    true,
		},
	} {
		if got := shouldOutput(test.loggerLevel, test.messageLevel); got != test.shouldLog {
			t.Errorf("shouldOutput(%d, %q) = %v, want %v", test.loggerLevel, test.messageLevel, got, test.shouldLog)
		}
	}
}

// TestFileLogger is the most simple test of the file logger.
func TestFileLogger(t *testing.T) {
	f := newFile()
	defer os.Remove(f)
	l, err := File("Main", InfoLevel, f, false, false)
	if err != nil {
		t.Fatalf("File(_) = %v, need nil error", err)
	}
	for i := 0; i < 100; i++ {
		l.Infof("info msg nr %d", i)
	}
	l.Close()
	contents, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("os.ReadFile(_) = %v, need nil error", err)
	}
	// [0..99] is 100 lines, plus one trailing \n is 101
	if l := len(strings.Split(string(contents), "\n")); l != 101 {
		t.Fatalf("TestFileLogger: got %d lines, need %d", l, 101)
	}
}

// TestAtomicWritesAndLevel ensures that all writes to the log are atomic and that the output
// level is respected.
func TestAtomicWrites(t *testing.T) {
	f := newFile()
	defer os.Remove(f)
	l, err := File("Main", InfoLevel, f, false, false)
	if err != nil {
		t.Fatalf("File(_) = %v, need nil error", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Infof("info")
			l.Debugf("debug")
		}()
	}
	wg.Wait()
	if err := l.Close(); err != nil {
		t.Fatalf("Close() = %v, need nil error", err)
	}

	contents, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("os.ReadFile(_) = %v, need nil error", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, "debug") {
			t.Errorf("`line %q: unexpected DEBUG entry", line)
		}
		if !strings.HasSuffix(line, "[Main "+InfoLevel+"] info") {
			t.Errorf(`line %q: suffix "[Main " + InfoLevel + "] info" expected`, line)
		}
	}
}

// TestSubMods tests that modules appear correctly and that atomic writes are respected across subbed
// loggers.
func TestSubMods(t *testing.T) {
	f := newFile()
	defer os.Remove(f)

	l, err := File("", InfoLevel, f, false, false)
	if err != nil {
		t.Fatalf("File(_) = %v, need nil error", err)
	}
	defer l.Close()

	a := l.Sub("sub_a")
	defer a.Close()

	b := a.Sub("sub_b")
	defer a.Close()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			l.Infof("info from no_module")
		}()
		go func() {
			defer wg.Done()
			a.Infof("info from sub_a")
		}()
		go func() {
			defer wg.Done()
			b.Infof("info from sub_b")
		}()
	}
	wg.Wait()

	// The generated output lines may now contain as the module and level one of the following keys.
	allowed := map[string]bool{
		"[" + InfoLevel + "]":             true,
		"[sub_a " + InfoLevel + "]":       true,
		"[sub_a/sub_b " + InfoLevel + "]": true,
	}
	var want []string
	for k := range allowed {
		want = append(want, k)
	}
	contents, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("os.ReadFile(_) = %v, need nil error", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if line == "" {
			continue
		}
		matched := false
		for k := range allowed {
			if strings.Contains(line, k) {
				matched = true
			}
		}
		if !matched {
			t.Errorf("line %q: fails to match either of %v", line, want)
		}
	}
}

// TestClose verifies that a closed logger is not usable.
func TestClose(t *testing.T) {
	f := newFile()
	defer os.Remove(f)

	one, err := File("one", DebugLevel, f, false, false)
	if err != nil {
		t.Fatalf("File(...) = %v, need nil error", err)
	}
	one.Close()
	if err := one.Infof("info"); err == nil {
		t.Errorf("Infof() after closing returns nil error")
	}
}

// TestCloseSubs checks that when (sub) loggers are closed, dependent loggers still work.
func TestCloseSubs(t *testing.T) {
	// Close sub logger, main must continue to work
	f1 := newFile()
	defer os.Remove(f1)
	one, err := File("one", DebugLevel, f1, false, false)
	if err != nil {
		t.Fatalf("File(...) = %v, need nil error", err)
	}
	oneSub := one.Sub("sub")
	one.Infof("one first entry")
	oneSub.Warnf("closing oneSub")
	oneSub.Close()
	if err := one.Infof("one second entry"); err != nil {
		t.Errorf("one.Infof(...) = %v, need nil error", err)
	}
	one.Close()
	contents, err := os.ReadFile(f1)
	if err != nil {
		t.Fatalf("os.ReadFile(_) = %v, want nil error", err)
	}
	if !strings.Contains(string(contents), "one second entry") {
		t.Errorf("TestCloseSubs: failed to find main log after closing sub")
	}

	// Close main logger, sub must continue to work
	f2 := newFile()
	defer os.Remove(f2)
	one, err = File("one", DebugLevel, f2, false, false)
	if err != nil {
		t.Fatalf("File(...) = %v, need nil error", err)
	}
	oneSub = one.Sub("sub")
	one.Infof("one first entry")
	oneSub.Infof("oneSub first entry")
	one.Warnf("closing one")
	one.Close()
	if err := oneSub.Infof("oneSub second entry"); err != nil {
		t.Errorf("oneSub.Infof(...) = %v, want nil error", err)
	}
	oneSub.Close()
	contents, err = os.ReadFile(f2)
	if err != nil {
		t.Fatalf("os.ReadFile(_) = %v, need nil error", err)
	}
	if !strings.Contains(string(contents), "oneSub second entry") {
		t.Errorf("TestCloseSubs: failed to find sub log after closing sub")
	}
}
