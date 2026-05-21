package daemon

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
)

func ValidateBind(bind string, insecure bool, input io.Reader, output io.Writer) error {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	isLoopback := host == "localhost" || (ip != nil && ip.IsLoopback())
	if isLoopback {
		return nil
	}
	if !insecure {
		return fmt.Errorf("non-loopback daemon bind %q requires --insecure", bind)
	}
	if input == nil {
		return fmt.Errorf("non-loopback daemon bind requires interactive confirmation")
	}
	fmt.Fprintf(output, "Binding ACPA daemon to %s exposes unauthenticated local APIs. Type %q to continue: ", bind, bind)
	line, _ := bufio.NewReader(input).ReadString('\n')
	if strings.TrimSpace(line) != bind {
		return fmt.Errorf("non-loopback daemon bind was not confirmed")
	}
	return nil
}
