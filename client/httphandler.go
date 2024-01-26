package main

import "net/http"

type ClientHttpHandler struct {
}

func (c *ClientHttpHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {

}
