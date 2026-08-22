package net

import (
	"fmt"
	"testing"
)

func TestRCONRoundTrip(t *testing.T) {
	listener, err := ListenRCON("localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	done := make(chan error, 1)
	go func() {
		done <- serveRCONOnce(listener)
	}()

	client(t, listener.Addr().String())
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func serveRCONOnce(listener *RCONListener) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	err = conn.AcceptLogin("RightPassword")
	if err != nil {
		return fmt.Errorf("accept login: %w", err)
	}

	cmd, err := conn.AcceptCmd()
	if err != nil {
		return fmt.Errorf("accept command: %w", err)
	}

	resp := handleCommand(cmd)
	err = conn.RespCmd(resp)
	if err != nil {
		return fmt.Errorf("respond to command: %w", err)
	}
	return nil
}

func handleCommand(cmd string) (resp string) {
	return fmt.Sprintf("your command is %q", cmd)
}

func client(t *testing.T, address string) {
	conn, err := DialRCON(address, "RightPassword")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	err = conn.Cmd("TEST COMMAND")
	if err != nil {
		t.Fatal(err)
	}

	resp, err := conn.Resp()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Server response: %q", resp)
}

func ExampleListenRCON() {
	l, err := ListenRCON("localhost:25575")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Printf("Accept connection error: %v", err)
		}

		go func(conn RCONServerConn) {
			err = conn.AcceptLogin("CORRECT_PASSWORD")
			if err != nil {
				fmt.Printf("Login fail: %v", err)
			}
			defer conn.Close()

			// The client is login, we are accepting its command
			for {
				cmd, err := conn.AcceptCmd()
				if err != nil {
					fmt.Printf("Read command fail: %v", err)
					break
				}

				resp := handleCommand(cmd)

				// Return the result of command.
				// It's allowed to call RespCmd multiple times for one command.
				err = conn.RespCmd(resp)
				if err != nil {
					fmt.Printf("Response command fail: %v", err)
					break
				}
			}
		}(conn)
	}
}

func ExampleDialRCON() {
	conn, err := DialRCON("localhost:25575", "CORRECT_PASSWORD")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	err = conn.Cmd("TEST COMMAND")
	if err != nil {
		panic(err)
	}

	for {
		// Server may send the result in more(or less) than one packet.
		// See: https://wiki.vg/RCON#Fragmentation
		resp, err := conn.Resp()
		if err != nil {
			fmt.Print(err)
		}
		fmt.Printf("Server response: %q", resp)
		break
	}
}
