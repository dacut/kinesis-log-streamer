# Kinesis Log Streamer
Stream logs from the standard input to Kinesis.

## Usage
<code>kinesis-log-stream [<i>options</i>] _stream-name_</code>

Stream incoming log entries to a Kinesis stream. This is intended to be used
in an application that supports piping log entries to an external program.
For example, in Apache, the following LogFormat and CustomLog directives might
be used to write JSON-formatted logs to Kinesis:

```
	LogFormat "{\"ClientAddress\": \"%a\", \"PeerAddress\": \"%{c}a\", \
\"Protocol\": \"%H\", \"QueryString\": \"%q\", \"RequestHandler\": \"%R\", \
\"RequestLine\": \"%r\", \"RequestMethod\": \"%m\", \
\"RequestTimeMicroseconds\": %D, \"ResponseBodySize\": %B, \
\"Referer\": \"%{Referer}i\", \
\"StartTime\": \"%{%Y-%m-%dT%H:%M:%S}t.%{usec_frac}tZ\", \
\"Status\": %>s, \"User\": \"%u\", \"UserAgent\": \"%{User-agent}i\", \
\"UrlPath\": \"%U\" }" accessjson
	CustomLog \
		"|/usr/bin/kinesis-log-stream --format json ApacheAccessLog" \
		accessjson
```

## Options
* <code>-f _format_</code> | <code>--format=_format_</code>  
  Format of log entries. Valid values are `json` and `line`; defaults to `line`. This determines how the incoming stream is
  split into Kinesis records.

* <code>-h</code> | <code>--help</code>  
  Show this usage information.

* <code>-p _profile_</code> | <code>--profile=_profile_</code>
  If specified, obtain AWS credentials from the specified profile in ~/.aws/credentials.

* <code>-r _region_</code> | <code>--region=_region_</code>
  The AWS region the Kinesis stream is in. If unspecified, the value from the `$AWS_REGION` environment variable is used.

## License

This program and associated documentation and source code are licensed under the [Apache License, Version 2.0](https://www.apache.org/licenses/LICENSE-2.0).