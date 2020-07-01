package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

// LineStreamer is a type that sends lines to Kinesis on a line-by-line basis.
type LineStreamer struct {
	BaseStreamer
	pending []byte
}

// NewLineStreamer creates a new LineStreamer instance.
func NewLineStreamer(kinesisClient *kinesis.Kinesis, streamName string, hostID string) *LineStreamer {
	ls := new(LineStreamer)

	ls.KinesisClient = kinesisClient
	ls.StreamName = streamName
	ls.HostID = hostID

	return ls
}

// HandleData reads lines of data and streams the result to Kinesis, formatted as JSON.
func (ls *LineStreamer) HandleData(reader io.Reader, outputFormat string, outputKey string, additionalEntries map[string]string) error {
	buffer := make([]byte, 65536, 65536)
	var err error

	recordsChan := make(chan kinesis.PutRecordsRequestEntry, 5)
	doneChan := make(chan bool, 1)
	lineMapping := make(map[string]string)

	if outputFormat == "json" {
		for key, value := range additionalEntries {
			lineMapping[key] = value
		}
	}

	go ls.StreamToKinesis(recordsChan, doneChan)

	for {
		var nRead int
		nRead, err = reader.Read(buffer)
		if err != nil {
			break
		}

		// Ignore the unread bytes in buffer
		bufValid := buffer[:nRead]

		for len(bufValid) > 0 {
			// Look for the next linefeed
			lfPos := bytes.IndexByte(bufValid, byte('\n'))

			if lfPos == -1 {
				// No more lines, though make sure we're not splitting a \r\n pair.
				bufValidLen := len(bufValid)
				if bufValidLen > 0 && bufValid[bufValidLen-1] == byte('\r') {
					// We might be. Remove the \r.
					bufValid = bufValid[:bufValidLen-1]
				}

				ls.pending = append(ls.pending, bufValid...)
				break
			}

			var line []byte
			if lfPos > 0 && bufValid[lfPos-1] == byte('\r') {
				// \r\n-style linefeed.
				line = append(ls.pending, bufValid[:lfPos-1]...)
			} else {
				// \n-style linefeed.
				line = append(ls.pending, bufValid[:lfPos]...)
			}
			ls.pending = nil
			bufValid = bufValid[lfPos+1:]

			if len(line) > 0 {
				var data []byte

				if outputFormat == "string" {
					data = line
				} else {
					lineMapping[outputKey] = string(line)
					data, err = json.Marshal(lineMapping)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to JSON serialize an output mapping: %v\n", err)
						fmt.Fprintf(os.Stderr, "Mapping that caused the error: %v\n", lineMapping)
						continue
					}
				}
				requestEntry := kinesis.PutRecordsRequestEntry{
					Data:         data,
					PartitionKey: aws.String(ls.HostID),
				}
				recordsChan <- requestEntry
			}
		}
	}

	close(recordsChan)
	<-doneChan

	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "Unable to read a line from stdin: %v\n", err)
		return err
	}

	return nil
}
