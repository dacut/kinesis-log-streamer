Name:           kinesis-log-streamer
Version:        %{version}
Release:        0%{?dist}
Summary:        Stream logs from the standard input to Kinesis.
License:        ASL 2.0
URL:            https://github.com/dacut/kinesis-log-streamer
Source0:        https://dist.ionosphere.io/kinesis-log-streamer-src-%{version}.tar.gz

BuildRequires:  golang >= 1.13

%description
Stream incoming log entries to a Kinesis stream. This is intended to be used
in an application that supports piping log entries to an external program.

%prep
%setup -q -n kinesis-log-streamer-%{version}

%build
mkdir -p ./_build/src/github.com/dacut
ln -s $(pwd) ./_build/src/github.com/dacut/kinesis-log-streamer

export GOPATH=$(pwd)/_build:%{gopath}
go build -ldflags=-linkmode=external -o kinesis-log-streamer .

%install
install -d %{buildroot}%{_bindir}
install -p -m 0755 ./kinesis-log-streamer %{buildroot}%{_bindir}/kinesis-log-streamer

%files
%defattr(-,root,root,-)
%doc LICENSE README.md
%{_bindir}/kinesis-log-streamer

%changelog
* Wed Jul 01 2020 David Cuthbert <dacut@kanga.org> - 0.1.0-0
- Test packaging
