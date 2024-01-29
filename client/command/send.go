package main

import (
	"fmt"
	"net/http"
)

const httpUrl = "http://localhost:22331/api"

func SendCommand(command string, own int64, name, passwd, email, peers, content string, verify, peer, group int64, op int) (error, http.Header) {

	req, err := http.NewRequest(http.MethodGet, httpUrl, nil)
	if err != nil {
		return err, nil
	}
	if len(command) > 0 {
		req.Header.Set("command", command)
	}
	if own > 0 {
		req.Header.Set("own", fmt.Sprintf("%d", own))
	}
	if len(name) > 0 {
		req.Header.Set("name", name)
	}
	if len(passwd) > 0 {
		req.Header.Set("passwd", passwd)
	}
	if len(email) > 0 {
		req.Header.Set("email", email)
	}
	if len(peers) > 0 {
		req.Header.Set("peers", peers)
	}
	if len(content) > 0 {
		req.Header.Set("content", content)
	}
	if peer > 0 {
		req.Header.Set("peer", fmt.Sprintf("%d", peer))
	}
	if group > 0 {
		req.Header.Set("group", fmt.Sprintf("%d", group))
	}
	if verify > 0 {
		req.Header.Set("verifyCode", fmt.Sprintf("%d", verify))
	}
	if op >= 0 {
		req.Header.Set("op", fmt.Sprintf("%d", op))
	}

	client := &http.Client{}
	rsp, err := client.Do(req)
	if err != nil {
		return err, nil
	}
	defer rsp.Body.Close()

	return nil, rsp.Header
}
