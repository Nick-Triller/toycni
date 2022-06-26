package main

import (
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
)

// generateMac was copied from https://stackoverflow.com/questions/21018729/generate-mac-address-in-go
func generateMac() (string, error) {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		//fmt.Println("error:", err)
		return "", err
	}
	// Set the local bit
	buf[0] |= 2
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

// randStringBytes was copied from
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// run is a simple wrapper to execute a command synchronously.
func run(args ...string) error {
	l.Println(strings.Join(args, " "))
	command := exec.Command(args[0], args[1:]...)
	// command.Stdin = os.Stdin
	return command.Run()
}
