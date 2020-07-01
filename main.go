package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
For example, in Apache, the following directives can be used to write
JSON-formatted logs to Kinesis:

    LogFormat "{\
        \"ClientAddress\": \"%%a\", \"PeerAddress\": \"%%{c}a\", \
        \"Protocol\": \"%%H\", \"QueryString\": \"%%q\", \
        \"RequestHandler\": \"%%R\", \"RequestLine\": \"%%r\", \
        \"RequestMethod\": \"%%m\", \"RequestTimeMicroseconds\": %%D, \
        \"ResponseBodySize\": %%B, \"Referer\": \"%%{Referer}i\", \
        \"StartTime\": \"%%{%%Y-%%m-%%dT%%H:%%M:%%S}t.%%{usec_frac}tZ\", \
        \"Status\": %%>s, \"User\": \"%%u\", \
        \"UserAgent\": \"%%{User-agent}i\", \"UrlPath\": \"%%U\" \
    }" accessjson
    CustomLog \
        "|/usr/bin/kinesis-log-streamer --format json \
            --add-entry LogFile=AccessLog Apache" accessjson
    ErrorLog \
        "|/usr/bin/kinesis-log-streamer --format line \
            --output-format json --add-entry LogFile=ErrorLog Apache"

`)

	getopt.PrintUsage(w)
}

// KinesisStreamer is an interface that all formatters must provide for streaming data to Kinesis
type KinesisStreamer interface {
	HandleData(reader io.Reader, outputFormat string, outputKey string, additionalEntries map[string]string) error
}

func main() {
	getopt.BoolLong("help", 'h', "Show this usage information.")
	getopt.StringLong("format", 'f', "line", "Format of incoming log entries. Valid values are \"json\" and \"line\". This determines how the incoming stream is split into Kinesis records.", "<input-format>")
	getopt.StringLong("region", 'r', "", "The AWS region to use. If unspecified, the value from the $AWS_REGION environment variable is used.", "<region>")
	getopt.StringLong("profile", 'p', "", "If specified, obtain AWS credentials from the specified profile in ~/.aws/credentials.", "<profile>")
	getopt.StringLong("output-format", 'F', "auto", "Format of the output. Valid values are \"auto\", \"json\" and \"string\", with \"auto\" mapping to the appropriate output format based on the input format (line->string, json->json).", "<output-format>")
	getopt.StringLong("output-key", 'k', "LogEntry", "When mapping incoming line entries to JSON output, this determines what JSON key to output the line under.", "<key>")
	addedEntriesOpt := getopt.ListLong("add-entry", 'I', "Add the specified entry to each JSON output; this may be specified multiple times.", "<key>=<value>")
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

	outputFormat := getopt.GetValue('F')
	if outputFormat != "auto" && outputFormat != "json" && outputFormat != "string" {
		fmt.Fprintf(os.Stderr, "Unrecognized output format: %s\n", outputFormat)
		WriteUsageToStderr()
		os.Exit(2)
	}

	if outputFormat == "auto" {
		if format == "json" {
			outputFormat = "json"
		} else {
			outputFormat = "string"
		}
	}

	outputKey := getopt.GetValue('k')
	additionalEntries := make(map[string]string)
	if addedEntriesOpt != nil {
		for _, addedEntryString := range *addedEntriesOpt {
			parts := strings.SplitN(addedEntryString, "=", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Additional entry is missing '=': %s\n", addedEntryString)
				WriteUsageToStderr()
				os.Exit(2)
			}

			key := parts[0]
			value := parts[1]

			if _, found := additionalEntries[key]; found {
				fmt.Fprintf(os.Stderr, "Duplicate key for --add-entry: %s\n", key)
				os.Exit(2)
			}

			additionalEntries[key] = value
		}
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

	err = handler.HandleData(os.Stdin, outputFormat, outputKey, additionalEntries)
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "Failed to read from stdin: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
