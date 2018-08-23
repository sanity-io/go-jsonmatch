package jsonmatch_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sanity-io/jsonmatch"
)

var failed []string

func tokenize(path string) string {
	s := jsonmatch.NewScanner(strings.NewReader(path))
	output := bytes.Buffer{}
	for token, text, pos := s.Scan(); token != jsonmatch.EOF; token, text, pos = s.Scan() {
		output.WriteString(fmt.Sprintf("%d: %q, %v\n", pos, text, token))
	}
	return output.String()
}

func TestReferencePaths(t *testing.T) {
	file, err := os.Open("./test_data/reference.txt")
	require.NoError(t, err)
	scanner := bufio.NewScanner(file)
	i := 1
	for scanner.Scan() {
		current := bytes.Buffer{}
		line := scanner.Text()
		current.WriteString(fmt.Sprintf("line #%d %q\n\r------------\n\r", i, line))
		current.WriteString(tokenize(line))
		filename := fmt.Sprintf("./test_data/%03d_tokens.txt", i)
		reference, error := ioutil.ReadFile(filename)
		if os.IsNotExist(error) {
			err := ioutil.WriteFile(filename, current.Bytes(), 0644)
			require.NoError(t, err)
		} else {
			currentBytes := current.Bytes()
			if !bytes.EqualFold(currentBytes, reference) {
				fmt.Printf("Mismatch tokenizing line #%d: %q\n\r", i, line)
				fmt.Printf("current: %s\n\r", currentBytes)
				fmt.Print("---\n\r")
				fmt.Printf("reference: %s\n\r\n\r", string(reference))
				failed = append(failed, fmt.Sprintf("line #%d tokenization %q", i, line))
			}
		}

		expr, err := jsonmatch.Parse(line)
		require.NoError(t, err, "Error while parsing line #%d %q", i, line)
		parseTree, err := json.MarshalIndent(expr, "", "  ")
		require.NoError(t, err, "Error while marhsalling line #%d %q", i, line)

		filename = fmt.Sprintf("./test_data/%03d_parse.txt", i)
		reference, error = ioutil.ReadFile(filename)
		if os.IsNotExist(error) {
			err := ioutil.WriteFile(filename, parseTree, 0644)
			require.NoError(t, err)
		} else {
			if !bytes.EqualFold(parseTree, reference) {
				fmt.Printf("Mismatch parsing line #%d: %q\n\r", i, line)
				fmt.Printf("current: %s\n\r", parseTree)
				fmt.Print("---\n\r")
				fmt.Printf("reference: %s\n\r\n\r", string(reference))
				failed = append(failed, fmt.Sprintf("line #%d parsed %q", i, line))
			}
		}
		i++
	}
	if len(failed) > 0 {
		for _, l := range failed {
			fmt.Printf("    FAIL: %s\n\r", l)
		}
		assert.Fail(t, "Tests failed verifying JSONpath scanner and parser")
	}
}
