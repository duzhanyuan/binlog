// modify based on github.com/go-sql-driver/mysql
package dump

import (
	"bytes"
	"log"
	"testing"
)

func TestErrorsSetLogger(t *testing.T) {
	previous := errLog
	defer func() {
		errLog = previous
	}()

	// set up logger
	const expected = "prefix: test\n"
	buffer := bytes.NewBuffer(make([]byte, 0, 64))
	logger := log.New(buffer, "prefix: ", 0)

	// print
	SetLogger(logger)
	errLog.Print("test")

	// check result
	if actual := buffer.String(); actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}
