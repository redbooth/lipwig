Lipwig
======

<img src="lipwig.png" alt="going postal" width="600px" />

Lipwig is the reference implementation of [SSMP](https://github.com/aerofs/ssmp),
the Stupid-Simple Messaging Protocol, which aims to be a minimalist alternative
to XMPP, STOMP and similar protocols.

License
-------

BSD 3-clause, see accompanying LICENSE file.


Dependencies
------------

Required:
  - [Go](https://golang.org) 1.4+

Optional:
  - [gockerize](https://github.com/aerofs/gockerize)
    to build a minimal docker container


Package layout
--------------

    aerofs.com/lipwig/          standalone server
        ssmp                    common code shared by client and server libraries
        server                  server library
        client                  client library


Protocol support
----------------

The following optional SSMP features are supported:

  - anonymous login
  - client certificate authentication, w/ arbitrary path suffix
  - shared secret authentication
  - open login (i.e. unauthenticated)


Usage
-----

```
Usage of ./lipwig:
  -cacert=""                Path to CA certificate
  -cert=""                  Path to server certificate
  -host=""                  TLS hostname
  -insecure=false           Disable TLS
  -key=""                   Path to server private key
  -listen="0.0.0.0:8787"    Listening address
  -open=false               Enable open login
  -secret=""                Path to shared secret
```


Performance
-----------

Although correctness and simplicity are the primary concerns in a reference
implementation, lipwig is designed to scale. The amount of memory per active
connection is very low (a few kilobytes) and given enough network bandwidth
the server can handle upwards of 1 million messages per second on a laptop.

Don't take our word for it: run [ssmperf](https://github.com/aerofs/ssmperf)
on your own machine and see for yourself.

