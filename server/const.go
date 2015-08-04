// Copyright (c) 2015, Air Computing Inc. <oss@aerofs.com>
// All rights reserved.

package server

const respEvent = "000 "

var (
	respOk             = []byte("200\n")
	respBadRequest     = []byte("400\n")
	respUnauthorized   = []byte("401\n")
	respNotFound       = []byte("404\n")
	respNotAllowed     = []byte("405\n")
	respConflict       = []byte("409\n")
	respNotImplemented = []byte("501\n")
)
