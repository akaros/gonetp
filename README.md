About Gonetp
============
Gonetp is a multi-core netperf, written in Go.
It was originally written to evaluate the performance of the
[Akaros](https://github.com/brho/akaros) networking stack.  It can also be ran
on Linux.

Installation
============
These instructions assume you have a proper `GOPATH` setup, and that
`$GOPATH/bin` is in your path. Instructions for setting up your `GOPATH` can be
found here: <https://github.com/golang/go/wiki/GOPATH>

```
go get github.com/klueska/gonetp/gonetp-server
go get github.com/klueska/gonetp/gonetp-client
```

Usage
=====
Start the server on one machine:

`gonetp-server`

And the client on another

`gonetp-client -H=<server_ip>`

And read the results (this could use some cleaning up...):

```
rxmsg	txmsg	Elapsed		Utilization
size	size	Time	Tput	local/remote
bytes	bytes	secs	Mb/s	%cpu	   #cores
	 16384	10.45 	19534.54	15.57/15.57 1.87/1.87	
 16384		10.45 	19534.54	15.57/15.57 1.87/1.87
```
