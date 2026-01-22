/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trace

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/cmd/crank/beta/trace/internal/printer"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource"
)

// Bubble Tea messages for watch mode.
type treeUpdateMsg struct {
	rendered string
}

type treeErrMsg struct {
	err error
}

type treeQuitMsg struct{}

// Bubble Tea model for watch mode.
type treeModel struct {
	rendered string
	err      error
}

func (m treeModel) Init() tea.Cmd {
	return nil
}

func (m treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case treeUpdateMsg:
		m.rendered = msg.rendered
		return m, nil

	case treeErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case treeQuitMsg:
		return m, tea.Quit

	case tea.KeyMsg:
		// Allow user to quit with q, esc, or ctrl+c
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m treeModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	return m.rendered
}

// renderTreeToString runs the printer into a string buffer.
func renderTreeToString(p printer.Printer, tree *resource.Resource) (string, error) {
	var buf bytes.Buffer
	if err := p.Print(&buf, tree); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// watchResourceTree watches the resource tree until it's deleted.
func (c *Cmd) watchResourceTree(ctx context.Context, k *kong.Context, logger logging.Logger, kClient client.Client, root *resource.Resource, mapping *meta.RESTMapping, p printer.Printer) error {
	// Create a watch for the specific resource
	opts := &client.ListOptions{
		Namespace:     root.Unstructured.GetNamespace(),
		FieldSelector: fields.OneTermEqualSelector("metadata.name", root.Unstructured.GetName()),
	}

	// Use a typed object for watching
	obj := root.Unstructured.DeepCopy()

	// Start the Kubernetes watch
	watchClient, ok := kClient.(client.WithWatch)
	if !ok {
		return errors.New("client does not support watch")
	}
	w, err := watchClient.Watch(ctx, obj, opts)
	if err != nil {
		return errors.Wrap(err, "cannot start watch")
	}
	defer w.Stop()

	// Create Bubble Tea program
	prog := tea.NewProgram(
		treeModel{},
		tea.WithOutput(k.Stdout),
	)

	// Start producer loop in background
	go c.watchProducer(ctx, logger, kClient, root, mapping, p, prog, w)

	// Run Bubble Tea (blocks until quit)
	_, err = prog.Run()
	return err
}

// watchProducer runs the watch loop and sends updates to Bubble Tea.
func (c *Cmd) watchProducer(ctx context.Context, logger logging.Logger, kClient client.Client, root *resource.Resource, mapping *meta.RESTMapping, p printer.Printer, prog *tea.Program, w watch.Interface) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Cache last rendered string to avoid sending duplicates
	var last string

	// Helper to fetch and render the resource tree, then send to Bubble Tea
	renderAndSend := func() error {
		current := resource.GetResource(ctx, kClient, &v1.ObjectReference{
			Kind:       root.Unstructured.GetKind(),
			APIVersion: root.Unstructured.GetAPIVersion(),
			Name:       root.Unstructured.GetName(),
			Namespace:  root.Unstructured.GetNamespace(),
		})

		if current.Error != nil {
			if apierrors.IsNotFound(current.Error) {
				logger.Debug("Resource deleted, stopping watch")
				prog.Send(treeQuitMsg{})
				return nil
			}
			return current.Error
		}

		tree, err := c.getResourceTree(ctx, current, mapping, kClient, logger)
		if err != nil {
			return err
		}

		logger.Debug("Got resource tree", "root", tree)

		rendered, err := renderTreeToString(p, tree)
		if err != nil {
			return err
		}

		// Only send if changed
		if rendered != last {
			last = rendered
			prog.Send(treeUpdateMsg{rendered: rendered})
		}

		return nil
	}

	// Initial render
	if err := renderAndSend(); err != nil {
		c.handleProducerError(prog, err)
		return
	}

	// Watch loop
	for {
		select {
		case evt, ok := <-w.ResultChan():
			if !ok {
				prog.Send(treeQuitMsg{})
				return
			}
			if evt.Type == watch.Deleted {
				prog.Send(treeQuitMsg{})
				return
			}
			if err := renderAndSend(); err != nil {
				c.handleProducerError(prog, err)
				return
			}

		case <-ticker.C:
			// Periodically refresh to catch child resource changes
			if err := renderAndSend(); err != nil {
				c.handleProducerError(prog, err)
				return
			}

		case <-ctx.Done():
			prog.Send(treeQuitMsg{})
			return
		}
	}
}

// handleProducerError handles errors from the watch producer.
func (c *Cmd) handleProducerError(prog *tea.Program, err error) {
	if apierrors.IsNotFound(err) {
		prog.Send(treeQuitMsg{})
		return
	}
	prog.Send(treeErrMsg{err: errors.Wrap(err, errGetResource)})
}
