package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

// parserState is used in the stack of parser states as we read through a JSON document.
type parserState int

// JSONStreamer is a type that sends lines to Kinesis as each JSON object is completed.
type JSONStreamer struct {
	BaseStreamer
}

// NewJSONStreamer creates a new JSONStreamer instance.
func NewJSONStreamer(kinesisClient *kinesis.Kinesis, streamName string, hostID string) *JSONStreamer {
	js := new(JSONStreamer)

	js.KinesisClient = kinesisClient
	js.StreamName = streamName
	js.HostID = hostID

	return js
}

// HandleData reads lines of data and streams the result to Kinesis, re-formatted as JSON.
func (js *JSONStreamer) HandleData(reader io.Reader) error {
	var err error
	recordsChan := make(chan kinesis.PutRecordsRequestEntry, 5)
	doneChan := make(chan bool, 1)

	go js.StreamToKinesis(recordsChan, doneChan)
	decoder := json.NewDecoder(reader)
	for {
		var value interface{}
		err = decoder.Decode(&value)
		if err != nil {
			break
		}

		if value != nil {
			jsonData, err := json.Marshal(value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to re-JSON encode a record: %v\n", value)
				continue
			}

			requestEntry := kinesis.PutRecordsRequestEntry{
				Data:         jsonData,
				PartitionKey: aws.String(js.HostID),
			}
			recordsChan <- requestEntry
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
