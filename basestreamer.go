package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
)

// BaseStreamer is an abstract type that can send items to Kinesis
type BaseStreamer struct {
	KinesisClient *kinesis.Kinesis
	StreamName    string
	HostID        string
}

// StreamToKinesis streams a Kinesis PutRecordsRequestEntry to Kinesis, buffering items until they are ready to be sent.
func (bs *BaseStreamer) StreamToKinesis(recordsChan chan kinesis.PutRecordsRequestEntry, doneChan chan bool) {
	pending := make([]*kinesis.PutRecordsRequestEntry, 0, 5)

outer:
	for {
		if len(pending) > 0 {
			// We have pending items -- don't block on input so we can flush
			select {
			case msg, ok := <-recordsChan:
				if !ok {
					// Channel was closed -- stdin closed, etc.
					// We know we have to flush because pending had items.
					bs.Flush(pending)
					break outer
				}

				// Just append the item until the pending list is full.
				pending = append(pending, &msg)
				if len(pending) == 5 {
					bs.Flush(pending)
					pending = pending[:0]
				}

			default:
				// Nothing to do, so go ahead and flush to Kinesis
				bs.Flush(pending)
				pending = pending[:0]
			}
		} else {
			// No pending items -- block on input.
			msg, ok := <-recordsChan
			if !ok {
				break outer
			}

			// We should just append -- we know pending is empty, so we should
			// wait for more items.
			pending = append(pending, &msg)
		}
	}

	doneChan <- true
	close(doneChan)
}

// Flush writes buffered Kinesis records out to Kinesis.
func (bs *BaseStreamer) Flush(records []*kinesis.PutRecordsRequestEntry) {
	input := kinesis.PutRecordsInput{
		Records:    records,
		StreamName: aws.String(bs.StreamName),
	}

	output, err := bs.KinesisClient.PutRecords(&input)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call Kinesis.PutRecords: %v\n", err)
	} else if aws.Int64Value(output.FailedRecordCount) > 0 {
		for _, resultEntry := range output.Records {
			errorMessage := aws.StringValue(resultEntry.ErrorMessage)
			shardID := aws.StringValue(resultEntry.ShardId)

			if errorMessage != "" {
				fmt.Fprintf(os.Stderr, "Failed to write a Kinesis record: %s (ShardId=%s)\n", errorMessage, shardID)
			}
		}
	}

	return
}
