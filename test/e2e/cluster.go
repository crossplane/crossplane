// Package main implements e2e testing code
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bitfield/script"
	"k8s.io/apimachinery/pkg/util/wait"
)

type cluster struct {
	cli string
}

func (c *cluster) ApplyYaml(content string) error {
	return exec(fmt.Sprintf("%s apply -f -", c.cli), content)
}

func (c *cluster) ApplyYamlToNamespace(namespace, content string) error {
	return exec(fmt.Sprintf("%s apply -f - -n %s", c.cli, namespace), content)
}

func (c *cluster) GetAndFilterResourceByJq(resType string, name string, namespace string, jqExpr string) (string, error) {
	cmd := fmt.Sprintf("%s get %s %s -n %s -o json", c.cli, resType, name, namespace)
	out := &strings.Builder{}
	p := script.NewPipe().WithStdout(out).Exec(cmd).JQ(jqExpr)
	_, err := p.Stdout()
	if err != nil {
		return "", err
	}
	if err := p.Error(); err != nil {
		return "", err
	}
	p.Wait()
	return out.String(), nil
}

func (c *cluster) WaitForResourceValueMatch(resType string, name string, namespace string, jqExpr string, value string) error {
	return wait.PollImmediate(5*time.Second, 60*time.Second, func() (done bool, err error) {
		rawJSON, err := c.GetAndFilterResourceByJq(resType, name, namespace, jqExpr)
		if err != nil {
			return false, nil //nolint:nilerr // should be like that
		}

		var val string
		if err = json.Unmarshal([]byte(rawJSON), &val); err != nil {
			return false, nil //nolint:nilerr // should be like that
		}

		return val == value, nil
	})
}

func (c *cluster) WaitForResourceConditionMatch(resType string, name string, namespace string, conditions map[string]string) error {
	for k, v := range conditions {
		if err := c.WaitForResourceValueMatch(resType, name, namespace, fmt.Sprintf(".status.conditions[] | select(.type==\"%s\").status", k), v); err != nil {
			return err
		}
	}
	return nil
}

func exec(cmd string, stdin string) error {
	out := &strings.Builder{}
	p := script.Echo(stdin).WithStdout(out).Exec(cmd)
	_, err := p.Stdout()
	if err != nil {
		return err
	}
	p.Wait()
	if p.ExitStatus() != 0 {
		return fmt.Errorf("error code returned %d stdout %s", p.ExitStatus(), out.String())
	}
	return nil
}

func (c *cluster) createNamespace(namespace string) error {
	out := &strings.Builder{}
	p := script.NewPipe().WithStdout(out).Exec(fmt.Sprintf("%s create ns %s --dry-run=client -o yaml", c.cli, namespace)).Exec(c.cli + " apply -f -")
	_, err := p.Stdout()
	if err != nil {
		return err
	}
	if err := p.Error(); err != nil {
		return err
	}
	p.Wait()
	if p.ExitStatus() != 0 {
		return fmt.Errorf("error code returned %d \n%s", p.ExitStatus(), out.String())
	}
	return nil
}
