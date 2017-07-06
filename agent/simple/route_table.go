package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
)

const (
	RT_TABLES_FILE = "/etc/iproute2/rt_tables"
)

// ensureRouteTableExist verifies that romana route table with appropriate index
// exist in RT_TABLES_FILE file.
func ensureRouteTableExist(routeTableId int) error {
	var err error

	file, err := os.OpenFile(RT_TABLES_FILE, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	targetEntry := fmt.Sprintf("%d romana\n", routeTableId)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == targetEntry {
			return nil
		}
	}

	_, err = file.WriteString(targetEntry)
	if err != nil {
		return err
	}

	return nil
}

// ensureRomanaRouteRule verifies that rule for romana routing table installed.
func ensureRomanaRouteRule(romanaRouteTableId int) error {
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	for _, rule := range rules {
		if rule.Table == romanaRouteTableId {
			return nil
		}
	}

	// TODO, not tested
	// it's possible will need to add 2 rules
	// with different src and dst
	inRule := netlink.Rule{
		Table: romanaRouteTableId,
	}

	err = netlink.RuleAdd(&inRule)
	if err != nil {
		return err
	}

	return nil
}

// flushRomanaTable attempts to delete all routes from table called romana.
func flushRomanaTable() error {
	command := exec.Command("ip", "ro", "flush", "table", "romana")

	out, err := CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("failed to flush romana route table out=%s, err=%s", string(out), err)
	}

	return nil

}

// exists for testing purpuses.
var CombinedOutput = func(cmd *exec.Cmd) ([]byte, error) { return cmd.CombinedOutput() }
