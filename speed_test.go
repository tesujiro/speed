package main

import (
	"bytes"
	"testing"
)

func compareInputOutputFile(t *testing.T, s string) {
	stdin := bytes.NewBufferString(s)
	stdout := new(bytes.Buffer)

	opt := &option{
		speed:    0,
		unit:     "K",
		silent:   true,
		graph:    false,
		filename: "",
	}
	limitedPipe(stdin, stdout, len(s), opt)
	expected := []byte(s)
	//fmt.Printf("stdin : %s\n", expected)
	//fmt.Printf("stdout: %s\n", stdout)
	if bytes.Compare(expected, stdout.Bytes()) != 0 {
		t.Fatal("stdout not matched to stdin")
	}
}

func TestCompareInputOutputShortFile(t *testing.T) {
	content := "this is a test file.\n"

	compareInputOutputFile(t, content)
}

func TestCompareInputOutputLongFile(t *testing.T) {
	content := "this is a test file.\n"
	for i := 0; i < 10; i++ {
		content += content
	}

	compareInputOutputFile(t, content)
}
