package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	getopt "github.com/pborman/getopt/v2"
)

const bufferSize = 262144

var cachedHostID string

// WriteUsageToStderr print usage information to os.Stderr. This is provided
// for signature compatibility is getopt.SetUsage.
func WriteUsageToStderr() {
	WriteUsage(os.Stderr)
}

// WriteUsage prints usage information to the specified Writer.
func WriteUsage(w io.Writer) {
	fmt.Fprintf(w, `kinesis-log-stream [options] <stream-name>

Stream incoming log entries to a Kinesis stream. This is intended to be used
in an application that supports piping log entries to an external program.
For example, in Apache, the following LogFormat and CustomLog directives might
be used to write JSON-formatted logs to Kinesis:

	LogFormat "{\"ClientAddress\": \"%%a\", \"PeerAddress\": \"%%{c}a\", \
\"Protocol\": \"%%H\", \"QueryString\": \"%%q\", \"RequestHandler\": \"%%R\", \
\"RequestLine\": \"%%r\", \"RequestMethod\": \"%%m\", \
\"RequestTimeMicroseconds\": %%D, \"ResponseBodySize\": %%B, \
\"Referer\": \"%%{Referer}i\", \
\"StartTime\": \"%%{%%Y-%%m-%%dT%%H:%%M:%%S}t.%%{usec_frac}tZ\", \
\"Status\": %%>s, \"User\": \"%%u\", \"UserAgent\": \"%%{User-agent}i\", \
\"UrlPath\": \"%%U\" }" accessjson
	CustomLog \
		"|/usr/bin/kinesis-log-stream --format json ApacheAccessLog" \
		accessjson

`)

	getopt.PrintUsage(w)
}

// KinesisStreamer is an interface that all formatters must provide for streaming data to Kinesis
type KinesisStreamer interface {
	HandleData(reader io.Reader) error
}

func main() {
	getopt.BoolLong("help", 'h', "Show this usage information.")
	getopt.StringLong("format", 'f', "line", "Format of log entries. Valid values are \"json\" and \"line\". This determines how the incoming stream is	split into Kinesis records.")
	getopt.StringLong("region", 'r', "", "The AWS region to use. If unspecified, the value from the $AWS_REGION environment variable is used.")
	getopt.StringLong("profile", 'p', "", "If specified, obtain AWS credentials from the specified profile in ~/.aws/credentials.")
	getopt.SetUsage(WriteUsageToStderr)

	getopt.Parse()

	if getopt.GetCount('h') > 0 {
		WriteUsage(os.Stdout)
		os.Exit(0)
	}

	format := getopt.GetValue('f')
	if format != "json" && format != "line" {
		fmt.Fprintf(os.Stderr, "Unrecognized format: %s\n", format)
		WriteUsageToStderr()
		os.Exit(2)
	}

	args := getopt.Args()

	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Kinesis stream must be specified.\n")
		WriteUsageToStderr()
		os.Exit(2)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Unknown argument: %s\n", args[1])
		WriteUsageToStderr()
		os.Exit(2)
	}

	kinesisStreamName := args[0]

	region := getopt.GetValue('r')
	profile := getopt.GetValue('p')

	awsSessionOptions := session.Options{Profile: profile}
	if region != "" {
		awsSessionOptions.Config.Region = aws.String(region)
	}

	awsSession, err := session.NewSessionWithOptions(awsSessionOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create an AWS session: %v\n", err)
		os.Exit(1)
	}

	kinesisClient := kinesis.New(awsSession)

	var handler KinesisStreamer

	if format == "line" {
		handler = NewLineStreamer(kinesisClient, kinesisStreamName, GetHostID())
	} else {
		handler = NewJSONStreamer(kinesisClient, kinesisStreamName, GetHostID())
	}

	err = handler.HandleData(os.Stdin)
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "Failed to read from stdin: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
